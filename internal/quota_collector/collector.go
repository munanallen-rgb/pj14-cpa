package quotacollector

import (
	"context"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// Collector coordinates scheduling, fetching, parsing, and persistence.
type Collector struct {
	cfg       Config
	store     *Store
	client    *Client
	scheduler *Scheduler
	clock     func() time.Time
}

// NewCollector creates a collector.
func NewCollector(cfg Config, store *Store, client *Client) *Collector {
	if client == nil {
		client = NewClient()
	}
	return &Collector{
		cfg:       cfg,
		store:     store,
		client:    client,
		scheduler: NewScheduler(),
		clock:     func() time.Time { return time.Now().UTC() },
	}
}

// Run starts the collector loop until the context is cancelled.
func (c *Collector) Run(ctx context.Context) error {
	if c == nil || c.store == nil {
		return fmt.Errorf("quota collector is not initialized")
	}
	if errMigrate := c.store.Migrate(ctx); errMigrate != nil {
		return errMigrate
	}
	if errCollect := c.CollectOnce(ctx, ReasonStartup); errCollect != nil {
		log.WithError(errCollect).Warn("quota collector startup collection failed")
	}

	hourlyTicker := time.NewTicker(hourlyInterval)
	defer hourlyTicker.Stop()

	for {
		delay := c.scheduler.NextDelay(c.clock())
		taskTimer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			stopTimer(taskTimer)
			return nil
		case <-hourlyTicker.C:
			stopTimer(taskTimer)
			if errCollect := c.CollectOnce(ctx, ReasonHourly); errCollect != nil {
				log.WithError(errCollect).Warn("quota collector hourly collection failed")
			}
		case <-taskTimer.C:
			for {
				task, ok := c.scheduler.Due(c.clock())
				if !ok {
					break
				}
				if !c.scheduler.CanCollect(c.clock()) {
					log.WithFields(log.Fields{"reason": task.Reason, "execute_at": task.ExecuteAt}).Warn("quota collector skipped task due to hourly cap")
					continue
				}
				if errCollect := c.CollectOnce(ctx, task.Reason); errCollect != nil {
					log.WithError(errCollect).Warn("quota collector reset collection failed")
				}
			}
		}
	}
}

func stopTimer(timer *time.Timer) {
	if timer == nil {
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

// CollectOnce performs a full collection across all configured CPA instances.
func (c *Collector) CollectOnce(ctx context.Context, reason string) error {
	if c == nil || c.store == nil || c.client == nil {
		return fmt.Errorf("quota collector is not initialized")
	}
	startedAt := c.clock()
	runID, errRun := c.store.CreateRun(ctx, reason, startedAt, len(c.cfg.Instances))
	if errRun != nil {
		return errRun
	}

	run := CollectionRun{
		ID:                 runID,
		StartedAt:          startedAt,
		Reason:             reason,
		Status:             RunStatusError,
		AttemptedInstances: len(c.cfg.Instances),
	}
	var allSnapshots []Snapshot
	var errors []string

	for _, instance := range c.cfg.Instances {
		report, errFetch := c.client.FetchReport(ctx, instance)
		collectedAt := c.clock()
		if errFetch != nil {
			run.FailedInstances++
			category := classifyFetchError(errFetch)
			errors = append(errors, instance.Name+": "+errFetch.Error())
			allSnapshots = append(allSnapshots, Snapshot{
				RunID:            runID,
				CollectedAt:      collectedAt,
				CPASource:        instance.Name,
				Status:           StatusError,
				AccountPlan:      "unknown",
				CollectionReason: reason,
				ErrorCategory:    category,
				ErrorMessage:     errFetch.Error(),
				DataStale:        true,
			})
			continue
		}
		run.SuccessfulInstances++
		snapshots := snapshotsFromReport(instance.Name, reason, collectedAt, report)
		for i := range snapshots {
			snapshots[i].RunID = runID
		}
		allSnapshots = append(allSnapshots, snapshots...)
		c.scheduler.AddResetTasks(collectedAt, snapshots)
	}

	if errInsert := c.store.InsertSnapshots(ctx, allSnapshots); errInsert != nil {
		errors = append(errors, errInsert.Error())
	}

	run.FinishedAt = c.clock()
	run.ErrorMessage = strings.Join(errors, "; ")
	switch {
	case run.SuccessfulInstances == run.AttemptedInstances && len(errors) == 0:
		run.Status = RunStatusSuccess
	case run.SuccessfulInstances > 0:
		run.Status = RunStatusPartial
	default:
		run.Status = RunStatusError
	}
	if errFinish := c.store.FinishRun(ctx, run); errFinish != nil {
		return errFinish
	}
	c.scheduler.RecordCollection(startedAt)
	log.WithFields(log.Fields{
		"run_id":               run.ID,
		"reason":               reason,
		"status":               run.Status,
		"successful_instances": run.SuccessfulInstances,
		"failed_instances":     run.FailedInstances,
		"snapshots":            len(allSnapshots),
	}).Info("quota collector run finished")
	if run.Status == RunStatusError {
		return fmt.Errorf("quota collector run failed: %s", run.ErrorMessage)
	}
	return nil
}
