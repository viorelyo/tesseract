package main

import (
	"fmt"
	"os"
	"time"

	"github.com/docker/docker/client"
	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
	"github.com/viorelyo/tesseract/manager"
	"github.com/viorelyo/tesseract/node"
	"github.com/viorelyo/tesseract/task"
	"github.com/viorelyo/tesseract/worker"
)

func main() {
	t := task.Task{
		ID:     uuid.New(),
		Name:   "Task-1",
		State:  task.Pending,
		Image:  "Image-1",
		Memory: 1024,
		Disk:   1,
	}

	te := task.TaskEvent{
		ID:        uuid.New(),
		State:     task.Pending,
		Timestamp: time.Now(),
		Task:      t,
	}

	fmt.Printf("task: %v\n", t)
	fmt.Printf("task-event: %v\n", te)

	w := worker.Worker{
		Name:  "worker-1",
		Queue: *queue.New(),
		Db:    make(map[uuid.UUID]*task.Task),
	}
	fmt.Printf("worker: %v\n", w)
	w.CollectStats()
	w.RunTask()
	w.StartTask()
	w.StopTask()

	m := manager.Manager{
		Pending: *queue.New(),
		TaskDb:  make(map[string][]*task.Task),
		EventDb: make(map[string][]*task.TaskEvent),
		Workers: []string{w.Name},
	}

	fmt.Printf("manager: %v\n", m)
	m.SelectWorker()
	m.UpdateTasks()
	m.SendWork()

	n := node.Node{
		Name:   "Node-1",
		Role:   "worker",
		Ip:     "192.168.1.1",
		Cores:  4,
		Memory: 1024,
		Disk:   25,
	}

	fmt.Printf("node: %v\n", n)

	fmt.Printf("Creating a test container\n")
	docker, result := createContainer()
	if result.Error != nil {
		fmt.Printf("%v\n", result.Error)
		os.Exit(1)
	}

	time.Sleep(time.Second * 5)
	fmt.Printf("Stopping test container\n")
	_ = stopContainer(docker, result.ContainerId)
}

func createContainer() (*task.Docker, *task.DockerResult) {
	config := task.Config{
		Name:  "test-container-1",
		Image: "postgres:13",
		Env: []string{
			"POSTGRES_USER=test",
			"POSTGRES_PASSWORD=secret",
		},
	}

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		fmt.Printf("%v\n", err)
		return nil, nil
	}

	docker := task.Docker{
		Client: cli,
		Config: config,
	}

	result := docker.Run()
	if result.Error != nil {
		fmt.Printf("%v\n", result.Error)
		return nil, nil
	}

	fmt.Printf("Container [%s] is running with config %v\n", result.ContainerId, config)
	return &docker, &result
}

func stopContainer(docker *task.Docker, id string) *task.DockerResult {
	result := docker.Stop(id)
	if result.Error != nil {
		fmt.Printf("%v\n", result.Error)
		return nil
	}

	fmt.Printf("Container [%s] has been stopped and removed\n", result.ContainerId)
	return &result
}
