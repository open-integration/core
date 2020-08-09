package core

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"time"

	"github.com/open-integration/core/pkg/event"
	"github.com/open-integration/core/pkg/event/reporter"
	"github.com/open-integration/core/pkg/filedescriptor"
	"github.com/open-integration/core/pkg/graph"
	"github.com/open-integration/core/pkg/logger"
	"github.com/open-integration/core/pkg/modem"
	"github.com/open-integration/core/pkg/state"
	"github.com/open-integration/core/pkg/task"
	"github.com/open-integration/core/pkg/utils"
)

type (

	// Engine exposes the interface of the engine
	Engine interface {
		Run() error
		Modem() modem.Modem
	}

	engine struct {
		pipeline           Pipeline
		logger             logger.Logger
		eventChan          chan *event.Event
		stateUpdateRequest chan state.UpdateRequest
		taskLogsDirctory   string
		stateDir           string
		modem              modem.Modem
		statev1            state.State
		wg                 *sync.WaitGroup
		graphBuilder       graph.Builder
	}
)

// Run starts the pipeline execution
func (e *engine) Run() error {
	e.logger.Debug("Starting...", "pipeline", e.pipeline.Metadata.Name)
	err := e.modem.Init()
	if err != nil {
		return err
	}
	defer func() {
		e.logger.Debug("killing all services")
		e.modem.Destroy()
	}()
	e.wg.Add(1)
	e.stateUpdateRequest <- state.UpdateRequest{
		Metadata: state.UpdateRequestMetadata{
			CreatedAt: utils.TimeNow(),
		},
		UpdateStateMetadataRequest: &state.UpdateStateMetadataRequest{
			State: state.EngineStateInProgress,
		},
	}
	go e.waitForFinish()
	e.handleStateEvents()
	e.printGraph()
	return e.printStateStore()
}

// Modem returns the current modem
func (e *engine) Modem() modem.Modem {
	return e.modem
}

// handleStateEvents watch the event channel and act on each evnt
// state.EventEngineFinished - finished watching, execution finished
// state.EventEngineStarted OR state.EventTaskStarted OR state.EventTaskFinished - elect next tasks
// state.EventTaskElected - execute tasks
func (e *engine) handleStateEvents() {
	for {
		ev := <-e.eventChan
		switch ev.Metadata.Name {
		case state.EventEngineFinished:
			return
		case state.EventEngineStarted, state.EventTaskStarted, state.EventTaskFinished:
			go e.electNextTasks(*ev)
		case state.EventTaskElected:
			go e.executeElectedTasks(*ev)
		}
		e.printGraph()
	}
}

// handleTaskEvents watch on dedicated event channel created for each task.
func (e *engine) handleTaskEvents(lgr logger.Logger, ch <-chan event.Event) {
	for {
		ev := <-ch
		lgr.Debug("Got event from task", "name", ev.Metadata.Name)
		go e.electNextTasks(ev)
	}
}

// electNextTasks - running all reactions on the event and sending request to elect matched tasks
func (e *engine) electNextTasks(ev event.Event) {
	log := e.logger.New("event", ev.Metadata.Name)
	log.Debug("Received event, electing next tasks")
	stateCpy, err := e.statev1.Copy()
	if err != nil {
		e.logger.Error("Failed to copy state")
		return
	}
	tasksCandidates := map[string]task.Task{}
	for _, reaction := range e.pipeline.Spec.Reactions {
		if reaction.Condition.Met(ev, stateCpy) {
			for _, t := range reaction.Reaction(ev, stateCpy) {
				tasksCandidates[t.Name()] = t
			}
		}
	}

	tasksToElect := []task.Task{}
	for _, t := range tasksCandidates {
		_, exist := stateCpy.Tasks()[t.Name()]
		if !exist {
			e.logger.Debug("Adding task to elected set", "task", t.Name())
			tasksToElect = append(tasksToElect, t)
		}
	}
	if len(tasksToElect) > 0 {
		e.logger.Debug("Electing tasks", "total", len(tasksToElect))
		ids := []string{}
		for _, t := range tasksToElect {
			ids = append(ids, t.Name())
		}
		e.wg.Add(1)
		e.stateUpdateRequest <- state.UpdateRequest{
			Metadata: state.UpdateRequestMetadata{
				CreatedAt: utils.TimeNow(),
			},
			ElectTasksRequest: &state.ElectTasksRequest{
				Tasks: tasksToElect,
			},
			AddRealtedTaskToEventReuqest: &state.AddRealtedTaskToEventReuqest{
				EventID: ev.Metadata.ID,
				Task:    ids,
			},
		}
	}
}

// executeElectedTasks - execute all elected tasks in parallel
func (e *engine) executeElectedTasks(ev event.Event) {
	log := e.logger.New("event", ev.Metadata.Name)
	stateCpy, err := e.statev1.Copy()
	if err != nil {
		e.logger.Error("Failed to copy state")
		return
	}
	elected := []task.Task{}
	for _, t := range stateCpy.Tasks() {
		if t.State == state.TaskStateElected {
			elected = append(elected, t.Task)
		}
	}
	wg := &sync.WaitGroup{}
	for _, t := range elected {
		wg.Add(1)
		log.Debug("Running task", "task", t.Name())
		go e.runTask(t, ev, log.New("task", t.Name()))
		wg.Done()
	}
	wg.Wait()
}

func (e *engine) runTask(t task.Task, ev event.Event, lgr logger.Logger) {
	fileName := fmt.Sprintf("%s.log", t.Name())
	fileDescriptor := path.Join(e.taskLogsDirctory, fileName)
	e.wg.Add(1)
	times := state.TaskTimes{
		Started: utils.TimeNow(),
	}
	e.stateUpdateRequest <- state.UpdateRequest{
		Metadata: state.UpdateRequestMetadata{
			CreatedAt: utils.TimeNow(),
		},
		UpdateTaskStateRequest: &state.UpdateTaskStateRequest{
			State: state.TaskState{
				State:  state.TaskStateInProgress,
				Task:   t,
				Times:  times,
				Logger: fileDescriptor,
			},
		},
	}

	_, err := utils.CreateLogFile(e.taskLogsDirctory, fileName)
	if err != nil {
		lgr.Error("Failed to create log file for task")
		return
	}

	fd, err := filedescriptor.New(fileDescriptor)
	if err != nil {
		lgr.Error("Failed to create filedescriptor")
		return
	}

	eventChan := make(chan event.Event)
	go e.handleTaskEvents(e.logger.New("module", "task-event-handler"), eventChan)
	payload, err := t.Run(context.Background(), task.RunOptions{
		FD: fd,
		EventReporter: reporter.New(reporter.Options{
			EventChan: eventChan,
			Name:      t.Name(),
		}),
		Modem: e.modem,
	})

	e.wg.Add(1)
	times.Finished = utils.TimeNow()
	e.stateUpdateRequest <- state.UpdateRequest{
		Metadata: state.UpdateRequestMetadata{
			CreatedAt: utils.TimeNow(),
		},
		UpdateTaskStateRequest: &state.UpdateTaskStateRequest{
			State: state.TaskState{
				State:  state.TaskStateFinished,
				Status: e.concludeStatus(err),
				Task:   t,
				Times:  times,
				Output: payload,
				Error:  err,
				Logger: fileDescriptor,
			},
		},
	}
}

// waitForFinish watch all events and send finish command once there are no more tasks in in-progress state
func (e *engine) waitForFinish() {
	time.Sleep(5 * time.Second)
	e.wg.Wait()
	stateCpy, _ := e.statev1.Copy()
	for _, t := range stateCpy.Tasks() {
		if t.State != "finished" {
			go e.waitForFinish()
			return
		}
	}

	e.logger.Debug("All tasks seems to be finished, sending finish command")
	e.wg.Add(1)
	e.stateUpdateRequest <- state.UpdateRequest{
		Metadata: state.UpdateRequestMetadata{
			CreatedAt: utils.TimeNow(),
		},
		UpdateStateMetadataRequest: &state.UpdateStateMetadataRequest{
			State:  state.EngineStateFinished,
			Status: state.EngineStatusSuccess,
		},
	}
	return
}

func (e *engine) printStateStore() error {
	statebytes, err := e.statev1.StateBytes()
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(path.Join(e.stateDir, "state.yaml"), statebytes, os.ModePerm)
	if err != nil {
		e.logger.Error("Failed to store state to file")
		return err
	}

	eventbytes, err := e.statev1.EventBytes()
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(path.Join(e.stateDir, "events.yaml"), eventbytes, os.ModePerm)
	if err != nil {
		e.logger.Error("Failed to store state to file")
		return err
	}
	return nil
}

func (e *engine) printGraph() {
	s, _ := e.statev1.Copy()
	g := e.graphBuilder.Build(s)
	ioutil.WriteFile(path.Join(e.stateDir, "graph.dot"), g, os.ModePerm)
}

func (e *engine) concludeStatus(err error) string {
	status := state.TaskStatusSuccess
	if err != nil {
		e.logger.Error("Task exited with error", "err", err.Error())
		status = state.TaskStatusFailed
	}
	return status
}
