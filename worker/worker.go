package worker

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
	"github.com/viorelyo/tesseract/task"
)

type Worker struct {
	Name      string
	Queue     queue.Queue // TODO replace with generic queue impl
	Db        map[uuid.UUID]*task.Task
	TaskCount int
}

func (w *Worker) AddTask(t task.Task) {
	w.Queue.Enqueue(t)
}

func (w *Worker) CollectStats() {
	fmt.Println("Collecting stats")
}

func (w *Worker) RunTask() task.DockerResult {
	t := w.Queue.Dequeue()
	if t == nil {
		log.Println("No tasks in the queue")
		return task.DockerResult{Error: nil}
	}

	taskQueued := t.(task.Task)
	taskPersisted := w.Db[taskQueued.ID]
	if taskPersisted == nil {
		taskPersisted = &taskQueued
		w.Db[taskQueued.ID] = &taskQueued
	}

	var result task.DockerResult
	if task.IsValidStateTransition(taskPersisted.State, taskQueued.State) {
		switch taskQueued.State {
		case task.Scheduled:
			result = w.StartTask(taskQueued)
		case task.Completed:
			result = w.StopTask(taskQueued)
		default:
			result.Error = errors.New("sheesh o_o")
			// fixme panic here?
		}
	} else {
		err := fmt.Errorf("Invalid transition from %v to %v",
			taskPersisted.State, taskQueued.State)
		result.Error = err
	}
	return result
}

func (w *Worker) StartTask(t task.Task) task.DockerResult {
	t.StartTime = time.Now().UTC()

	config := task.NewConfig(&t)
	docker := task.NewDocker(config)
	result := docker.Run()
	if result.Error != nil {
		log.Printf("Could not run task %v: %v\n", t.ID, result.Error)

		t.State = task.Failed
		w.Db[t.ID] = &t
		return result
	}

	t.ContainerID = result.ContainerID
	t.State = task.Running
	w.Db[t.ID] = &t

	return result
}

func (w *Worker) StopTask(t task.Task) task.DockerResult {
	config := task.NewConfig(&t)
	docker := task.NewDocker(config)
	result := docker.Stop(t.ContainerID)
	if result.Error != nil {
		log.Printf("Could not stop container [%s]: %v\n", t.ContainerID, result.Error)
	}

	t.FinishTime = time.Now().UTC()
	t.State = task.Completed
	w.Db[t.ID] = &t

	log.Printf("Stopped and removed container [%s] for task %v\n", t.ContainerID, t.ID)
	return result
}
