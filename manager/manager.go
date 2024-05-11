package manager

import (
	"fmt"

	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
	"github.com/viorelyo/tesseract/task"
)

type Manager struct {
	Pending       queue.Queue
	Workers       []string
	WorkerTaskMap map[string][]uuid.UUID
	TaskDb        map[string][]*task.Task
	EventDb       map[string][]*task.TaskEvent
	TaskWorkerMap map[uuid.UUID]string
}

func (m *Manager) SelectWorker() {
	fmt.Println("Selecting worker")
}

func (m *Manager) UpdateTasks() {
	fmt.Println("Updating tasks")
}

func (m *Manager) SendWork() {
	fmt.Println("Sending work")
}
