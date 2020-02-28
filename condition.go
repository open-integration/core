package core

import "github.com/open-integration/core/pkg/state"

// ConditionEngineStarted returns true once the engine emits started event
func ConditionEngineStarted(ev state.Event, s state.State) bool {
	return ev.Metadata.Name == state.EventEngineStarted
}

// ConditionTaskFinishedWithStatus returns true once a task is finished with given status
func ConditionTaskFinishedWithStatus(task string, status string) func(ev state.Event, s state.State) bool {
	return func(ev state.Event, s state.State) bool {
		if ev.Metadata.Name != state.EventTaskFinished {
			return false
		}
		for _, t := range s.Tasks() {
			if t.Status == status && t.State == state.TaskStateFinished && t.Task.Metadata.Name == task {
				return true
			}
		}
		return false
	}
}

// ConditionTaskFinished returns true once a task reached finished state
func ConditionTaskFinished(task string) func(ev state.Event, s state.State) bool {
	return func(ev state.Event, s state.State) bool {
		if ev.Metadata.Name != state.EventTaskFinished {
			return false
		}
		for _, t := range s.Tasks() {
			if t.State == state.TaskStateFinished && t.Task.Metadata.Name == task {
				return true
			}
		}
		return false
	}
}
