package quotacollector

import (
	"testing"
	"time"
)

func TestSchedulerAddsResetTasksAndMergesCloseTimes(t *testing.T) {
	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	resetA := now.Add(30 * time.Minute)
	resetB := now.Add(32 * time.Minute)
	scheduler := NewScheduler()

	scheduler.AddResetTasks(now, []Snapshot{
		{Status: StatusSuccess, FiveHourResetAt: &resetA},
		{Status: StatusSuccess, FiveHourResetAt: &resetB},
	})

	if len(scheduler.tasks) != 2 {
		t.Fatalf("tasks len = %d, want 2 merged before/after tasks", len(scheduler.tasks))
	}
	if scheduler.tasks[0].Reason != ReasonResetBefore {
		t.Fatalf("first reason = %s, want reset_before", scheduler.tasks[0].Reason)
	}
	if scheduler.tasks[1].Reason != ReasonResetAfter {
		t.Fatalf("second reason = %s, want reset_after", scheduler.tasks[1].Reason)
	}
}

func TestSchedulerSkipsFailedSnapshots(t *testing.T) {
	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	reset := now.Add(30 * time.Minute)
	scheduler := NewScheduler()

	scheduler.AddResetTasks(now, []Snapshot{{Status: StatusError, FiveHourResetAt: &reset}})

	if len(scheduler.tasks) != 0 {
		t.Fatalf("tasks len = %d, want 0", len(scheduler.tasks))
	}
}

func TestSchedulerHourlyCap(t *testing.T) {
	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	scheduler := NewScheduler()

	for i := 0; i < maxCollectionsPerHour; i++ {
		if !scheduler.CanCollect(now) {
			t.Fatalf("CanCollect before cap = false at %d", i)
		}
		scheduler.RecordCollection(now.Add(time.Duration(i) * time.Minute))
	}
	if scheduler.CanCollect(now.Add(59 * time.Minute)) {
		t.Fatalf("CanCollect after cap = true, want false")
	}
	if !scheduler.CanCollect(now.Add(time.Hour)) {
		t.Fatalf("CanCollect next hour = false, want true")
	}
}
