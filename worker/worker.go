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
	Stats     *Stats
	TaskCount int
}

func (w *Worker) GetTasks() []*task.Task {
	tasks := []*task.Task{}
	for _, t := range w.Db {
		tasks = append(tasks, t)
	}
	return tasks
}

func (w *Worker) AddTask(t task.Task) {
	w.Queue.Enqueue(t)
}

func (w *Worker) CollectStats() {
	for {
		log.Println("[Worker] Collecting stats")
		w.Stats = GetStats()
		w.Stats.TaskCount = w.TaskCount

		time.Sleep(15 * time.Second)
	}
}

func (w *Worker) runTask() task.DockerResult {
	t := w.Queue.Dequeue()
	if t == nil {
		log.Println("[Worker] No tasks in the queue")
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
		err := fmt.Errorf("invalid transition from %v to %v",
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
		log.Printf("[Worker] Could not run task %v: %v\n", t.ID, result.Error)

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
		log.Printf("[Worker] Could not stop container [%s]: %v\n", t.ContainerID, result.Error)
	}

	t.FinishTime = time.Now().UTC()
	t.State = task.Completed
	w.Db[t.ID] = &t

	log.Printf("[Worker] Stopped and removed container [%s] for task %v\n", t.ContainerID, t.ID)
	return result
}

func (w *Worker) InspectTask(t task.Task) task.DockerInspectResponse {
	config := task.NewConfig(&t)
	d := task.NewDocker(config)
	return d.Inspect(t.ContainerID)
}

func (w *Worker) updateTasks() {
	// Worker checks its tasks to be running
	// uses DockerApi.Inspect to check if each task is in running state
	// if a task is not running, it is marked as failed

	updateFailedTask := func(id uuid.UUID) {
		w.Db[id].State = task.Failed
	}

	for id, t := range w.Db {
		if t.State == task.Running {
			resp := w.InspectTask(*t)
			if resp.Error != nil {
				log.Printf("[Worker] Error: %v\n", resp.Error)
			}

			if resp.ContainerData == nil {
				log.Printf("[Worker] ContainerData not fetched for running task: %s\n", id)
				updateFailedTask(id)
			}

			if resp.ContainerData.State.Status == "exited" {
				log.Printf("[Worker] ContainerData for task: %s in non-running state: %s\n", id, resp.ContainerData.State.Status)
				updateFailedTask(id)
			}

			// updating exposed hostPorts of the running task
			w.Db[id].HostPorts = resp.ContainerData.NetworkSettings.NetworkSettingsBase.Ports
		}
	}
}

func (w *Worker) UpdateTasks() {
	for {
		log.Println("[Worker] Checking status of tasks")
		w.updateTasks()
		log.Println("[Worker] Task statuses updated")
		log.Println("[Worker] Sleeping for 15s while updating tasks")
		time.Sleep(15 * time.Second)
	}
}

func (w *Worker) RunTasks() {
	for {
		if w.Queue.Len() != 0 {
			result := w.runTask()
			if result.Error != nil {
				log.Printf("[Worker] Could not run task: %v\n", result.Error)
			}
		} else {
			log.Printf("[Worker] No tasks to process currently\n")
		}

		log.Println("[Worker] Sleeping for 10s while running tasks")
		time.Sleep(10 * time.Second)
	}
}
