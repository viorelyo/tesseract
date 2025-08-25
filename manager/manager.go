package manager

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
	"github.com/viorelyo/tesseract/task"
	"github.com/viorelyo/tesseract/worker"
)

type Manager struct {
	Workers       []string
	LastWorker    int
	Pending       queue.Queue
	TaskDb        map[uuid.UUID]*task.Task
	EventDb       map[uuid.UUID]*task.TaskEvent
	WorkerTaskMap map[string][]uuid.UUID
	TaskWorkerMap map[uuid.UUID]string
}

func New(workers []string) *Manager {
	taskDb := make(map[uuid.UUID]*task.Task)
	evendDb := make(map[uuid.UUID]*task.TaskEvent)
	workerTaskMap := make(map[string][]uuid.UUID)
	taskWorkerMap := make(map[uuid.UUID]string)

	for w := range workers {
		workerTaskMap[workers[w]] = []uuid.UUID{}
	}

	return &Manager{
		Workers:       workers,
		LastWorker:    0,
		Pending:       *queue.New(),
		TaskDb:        taskDb,
		EventDb:       evendDb,
		WorkerTaskMap: workerTaskMap,
		TaskWorkerMap: taskWorkerMap,
	}
}

// Round-robin naive scheduling
func (m *Manager) SelectWorker() string {
	var newWorker int
	if m.LastWorker+1 < len(m.Workers) {
		newWorker = m.LastWorker + 1
		m.LastWorker++
	} else {
		newWorker = 0
		m.LastWorker = 0
	}

	return m.Workers[newWorker]
}

func (m *Manager) updateTasks() {
	for _, w := range m.Workers {
		log.Printf("[Manager] Checking worker [%v] for task updates\n", w)
		url := fmt.Sprintf("http://%s/tasks", w)
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("[Manager] Could not connect to worker [%v]: %v\n", w, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			log.Printf("[Manager] Could not send request: %v\n", err)
			continue
		}

		d := json.NewDecoder(resp.Body)
		var tasks []*task.Task
		err = d.Decode(&tasks)
		if err != nil {
			log.Printf("[Manager] Could not unmarshall tasks: %s\n", err.Error())
		}

		for _, t := range tasks {
			log.Printf("[Manager] Attempting to update task [%v]\n", t.ID)

			_, ok := m.TaskDb[t.ID]
			if !ok {
				log.Printf("[Manager] Task [%v] was not found\n", t.ID)
				continue
			}

			if m.TaskDb[t.ID].State != t.State {
				m.TaskDb[t.ID].State = t.State
			}

			m.TaskDb[t.ID].ContainerID = t.ContainerID
			m.TaskDb[t.ID].StartTime = t.StartTime
			m.TaskDb[t.ID].FinishTime = t.FinishTime
		}
	}
}

func (m *Manager) scheduleTask(te task.TaskEvent, _worker string) (*task.Task, error) {
	data, err := json.Marshal(te)
	if err != nil {
		errMsg := fmt.Sprintf("[Manager] Could not marshal task object: %v", te.Task)
		log.Println(errMsg)
		return nil, errors.New(errMsg)
	}

	url := fmt.Sprintf("http://%s/tasks", _worker)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		errMsg := fmt.Sprintf("[Manager] Could not connect to [%v]: %v", _worker, err)
		log.Println(errMsg)

		// retry later
		m.Pending.Enqueue(te)

		return nil, errors.New(errMsg)
	}

	d := json.NewDecoder(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		e := worker.ErrResponse{}
		err := d.Decode(&e)
		if err != nil {
			errMsg := fmt.Sprintf("[Manager] Could not decode response: %s", err.Error())
			log.Println(errMsg)
			return nil, errors.New(errMsg)
		}

		errMsg := fmt.Sprintf("[Manager] Response error (%d): %s", e.HTTPStatusCode, e.Message)
		log.Println(errMsg)
		return nil, errors.New(errMsg)
	}

	t := new(task.Task)
	err = d.Decode(t)
	if err != nil {
		errMsg := fmt.Sprintf("[Manager] Could not decode response: %s", err.Error())
		log.Println(errMsg)
		return nil, errors.New(errMsg)
	}

	log.Printf("[Manager] Task sent: %v\n", t)

	return t, nil
}

func (m *Manager) SendWork() {
	if m.Pending.Len() > 0 {
		w := m.SelectWorker()

		e := m.Pending.Dequeue()
		te := e.(task.TaskEvent)
		t := te.Task

		log.Printf("[Manager] Pulled %v off pending queue\n", t)

		m.EventDb[te.ID] = &te
		m.WorkerTaskMap[w] = append(m.WorkerTaskMap[w], te.Task.ID)
		m.TaskWorkerMap[t.ID] = w

		t.State = task.Scheduled
		m.TaskDb[t.ID] = &t

		newTask, err := m.scheduleTask(te, w)
		if err != nil {
			return
		}

		// update the task
		t = *newTask
	} else {
		log.Println("[Manager] No work in the queue")
	}
}

func (m *Manager) UpdateTasks() {
	for {
		log.Println("[Manager] Checking for task updates from workers")
		m.updateTasks()
		log.Println("[Manager] Tasks updated")
		log.Println("[Manager] Sleeping for 15s while updating tasks")
		time.Sleep(15 * time.Second)
	}
}

func (m *Manager) ProcessTasks() {
	for {
		log.Println("[Manager] Processing tasks from the queue")
		m.SendWork()
		log.Println("[Manager] Sleeping for 10s while processing tasks")
		time.Sleep(10 * time.Second)
	}
}

func (m *Manager) AddTaskEvent(te task.TaskEvent) {
	m.Pending.Enqueue(te)
}

func (m *Manager) GetTasks() []*task.Task {
	tasks := []*task.Task{}
	for _, t := range m.TaskDb {
		tasks = append(tasks, t)
	}
	return tasks
}

func (m *Manager) checkTaskHealth(t task.Task) error {
	// Calls the health check directly on the task
	// To find out the healthCheckUrl, it requires the address of the worker where task is running

	getHostPort := func(ports nat.PortMap) *string {
		for k, _ := range ports {
			return &ports[k][0].HostPort
		}
		return nil
	}

	worker := m.TaskWorkerMap[t.ID]
	workerSchema := strings.Split(worker, ":")
	hostPort := getHostPort(t.HostPorts)
	healthUrl := fmt.Sprintf("http://%s:%s%s", workerSchema[0], *hostPort, t.HealthCheckUrl)

	log.Printf("[Manager] Checking health for task: %s - [%s]\n", t.ID, healthUrl)
	resp, err := http.Get(healthUrl)
	if err != nil {
		msg := fmt.Sprintf("[Manager] Error connecting to health check url [%s]", healthUrl)
		log.Println(msg)
		return errors.New(msg)
	}

	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("[Manager] Error health check for task: %s - [%d]", t.ID, resp.StatusCode)
		log.Println(msg)
		return errors.New(msg)
	}

	return nil
}

func (m *Manager) performHealthChecks() {
	// Checking health on each running task
	// Restarts up-to 3 times the failed tasks

	for _, t := range m.GetTasks() {
		if t.State == task.Running && t.RestartCount < 3 {
			err := m.checkTaskHealth(*t)
			if err != nil {
				if t.RestartCount < 3 {
					m.restartTask(t)
				}
			}
		} else if t.State == task.Failed && t.RestartCount < 3 {
			m.restartTask(t)
		}
	}
}

func (m *Manager) restartTask(t *task.Task) {
	// Reschedules a task on the same worker

	w := m.TaskWorkerMap[t.ID]
	t.State = task.Scheduled
	t.RestartCount++

	// update the internal DB
	m.TaskDb[t.ID] = t

	te := task.TaskEvent{
		ID:        uuid.New(),
		State:     task.Running,
		Timestamp: time.Now(),
		Task:      *t,
	}

	_, _ = m.scheduleTask(te, w)
}

func (m *Manager) PerformHealthChecks() {
	for {
		log.Println("[Manager] Performing task health checks")
		m.performHealthChecks()
		log.Println("[Manager] Performed task health checks")
		log.Println("[Manager] Sleeping for 60s while performing task health checks")
		time.Sleep(60 * time.Second)
	}
}
