package quotacollector

import (
	"sort"
	"time"
)

const (
	hourlyInterval        = time.Hour
	resetBeforeOffset     = -time.Minute
	resetAfterOffset      = time.Minute
	taskMergeWindow       = 5 * time.Minute
	maxCollectionsPerHour = 8
)

// Scheduler keeps the collector's in-memory reset collection tasks.
type Scheduler struct {
	tasks     []ResetTask
	hourCount map[time.Time]int
}

// NewScheduler creates an empty scheduler.
func NewScheduler() *Scheduler {
	return &Scheduler{hourCount: make(map[time.Time]int)}
}

func (s *Scheduler) RecordCollection(at time.Time) {
	if s == nil {
		return
	}
	hour := at.UTC().Truncate(time.Hour)
	s.hourCount[hour]++
	for key := range s.hourCount {
		if at.UTC().Sub(key) > 24*time.Hour {
			delete(s.hourCount, key)
		}
	}
}

func (s *Scheduler) CanCollect(at time.Time) bool {
	if s == nil {
		return true
	}
	return s.hourCount[at.UTC().Truncate(time.Hour)] < maxCollectionsPerHour
}

func (s *Scheduler) AddResetTasks(now time.Time, snapshots []Snapshot) {
	if s == nil {
		return
	}
	for _, snapshot := range snapshots {
		if snapshot.Status != StatusSuccess {
			continue
		}
		for _, resetAt := range []*time.Time{snapshot.FiveHourResetAt, snapshot.WeeklyResetAt} {
			if resetAt == nil || !resetAt.After(now) {
				continue
			}
			s.addTask(ResetTask{ExecuteAt: resetAt.Add(resetBeforeOffset), Reason: ReasonResetBefore}, now)
			s.addTask(ResetTask{ExecuteAt: resetAt.Add(resetAfterOffset), Reason: ReasonResetAfter}, now)
		}
	}
	sort.Slice(s.tasks, func(i, j int) bool {
		return s.tasks[i].ExecuteAt.Before(s.tasks[j].ExecuteAt)
	})
}

func (s *Scheduler) Due(now time.Time) (ResetTask, bool) {
	if s == nil || len(s.tasks) == 0 {
		return ResetTask{}, false
	}
	if s.tasks[0].ExecuteAt.After(now) {
		return ResetTask{}, false
	}
	task := s.tasks[0]
	s.tasks = s.tasks[1:]
	return task, true
}

func (s *Scheduler) NextDelay(now time.Time) time.Duration {
	delay := hourlyInterval
	if s != nil && len(s.tasks) > 0 {
		taskDelay := s.tasks[0].ExecuteAt.Sub(now)
		if taskDelay < 0 {
			return 0
		}
		if taskDelay < delay {
			delay = taskDelay
		}
	}
	if delay <= 0 {
		return time.Second
	}
	return delay
}

func (s *Scheduler) addTask(task ResetTask, now time.Time) {
	if task.ExecuteAt.Before(now.Add(-taskMergeWindow)) {
		return
	}
	for i, existing := range s.tasks {
		if absDuration(existing.ExecuteAt.Sub(task.ExecuteAt)) <= taskMergeWindow && existing.Reason == task.Reason {
			if task.ExecuteAt.Before(existing.ExecuteAt) {
				s.tasks[i].ExecuteAt = task.ExecuteAt
			}
			return
		}
	}
	s.tasks = append(s.tasks, task)
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
