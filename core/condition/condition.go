package condition

import (
	"github.com/open-integration/open-integration/core/event"
	"github.com/open-integration/open-integration/core/state"
)

const (

	// KindConditionTaskFinishedWithStatus builds condition_task_finished_with_status.go
	KindConditionTaskFinishedWithStatus = "condition.task.finished.with.status"

	// KindConditionTaskFinished builds condition_task_finished.go
	KindConditionTaskFinished = "condition.task.finished"
)

type (
	// Condition exposes interface to run condition on state events
	Condition interface {
		Met(ev event.Event, s state.State) bool
	}
)

// EngineStarted builds engine_started.go
func EngineStarted() Condition {
	return &conditionEngineStarted{}
}

// TaskFinished builds task_finished.go
func TaskFinished(task string) Condition {
	return &conditionTaskFinished{task}
}

// TaskFinishedWithStatus builds task_finished_with_status.go
func TaskFinishedWithStatus(task string, status string) Condition {
	return &conditionTaskFinishedWithStatus{task, status}
}

// Combined builds combined.go
func Combined(condition ...Condition) Condition {
	return &conditionCombined{
		conditions: condition,
	}
}

// TaskEventReported build task_event_reported.go
func TaskEventReported(name string) Condition {
	return &conditionTaskEventReported{
		name: name,
	}
}
