package main

import (
	"fmt"
	"time"

	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
	"github.com/viorelyo/tesseract/task"
	"github.com/viorelyo/tesseract/worker"
)

func main() {
	db := make(map[uuid.UUID]*task.Task)

	t := task.Task{
		ID:    uuid.New(),
		Name:  "test-container-1",
		State: task.Scheduled,
		Image: "strm/helloworld-http",
	}

	w := worker.Worker{
		Name:  "worker-1",
		Queue: *queue.New(),
		Db:    db,
	}

	fmt.Println("Starting task")
	w.AddTask(t)
	result := w.RunTask()
	if result.Error != nil {
		panic(result.Error)
	}

	t.ContainerID = result.ContainerID
	fmt.Printf("Task %s is running in container %s\n", t.ID, t.ContainerID)

	time.Sleep(time.Second * 10)

	fmt.Printf("Stopping task %s\n", t.ID)
	t.State = task.Completed
	w.AddTask(t)
	result = w.RunTask()
	if result.Error != nil {
		panic(result.Error)
	}

}
