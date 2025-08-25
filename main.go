package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
	"github.com/viorelyo/tesseract/manager"
	"github.com/viorelyo/tesseract/task"
	"github.com/viorelyo/tesseract/worker"
)

func main() {
	workerHost := os.Getenv("TESSERACT_WORKER_HOST")
	workerPort, _ := strconv.Atoi(os.Getenv("TESSERACT_WORKER_PORT"))

	mgrHost := os.Getenv("TESSERACT_MANAGER_HOST")
	mgrPort, _ := strconv.Atoi(os.Getenv("TESSERACT_MANAGER_PORT"))

	// #region WorkerApi
	fmt.Println("Starting tesseract worker.")
	w := worker.Worker{
		Queue: *queue.New(), // todo use concurrent-safe queue ?
		Db:    make(map[uuid.UUID]*task.Task),
	}
	workerApi := worker.Api{
		Address: workerHost,
		Port:    workerPort,
		Worker:  &w,
	}

	go w.RunTasks()
	go w.CollectStats()
	go w.UpdateTasks()
	go workerApi.Start()
	// #endregion

	// #region ManagerApi
	fmt.Println("Starting tesseract manager.")
	workers := []string{fmt.Sprintf("%s:%d", workerHost, workerPort)}

	mgr := manager.New(workers)
	mgrApi := manager.Api{
		Address: mgrHost,
		Port:    mgrPort,
		Manager: mgr,
	}

	go mgr.ProcessTasks()
	go mgr.UpdateTasks()
	go mgr.PerformHealthChecks()
	mgrApi.Start()
	// #endregion

	// #region Test for manually added mock tasks
	// for i := 0; i < 3; i++ {
	// 	t := task.Task{
	// 		ID:    uuid.New(),
	// 		Name:  fmt.Sprintf("test-container-%d", i),
	// 		State: task.Scheduled,
	// 		Image: "strm/helloworld-http",
	// 	}
	// 	te := task.TaskEvent{
	// 		ID:    uuid.New(),
	// 		State: task.Running,
	// 		Task:  t,
	// 	}
	// 	mgr.AddTaskEvent(te)
	// 	mgr.SendWork()
	// }

	// for {
	// 	for _, t := range mgr.TaskDb {
	// 		fmt.Printf("Manager Task: id: %s, state: %d\n", t.ID, t.State)
	// 		time.Sleep(15 * time.Second)
	// 	}
	// }
	// #endregion
}
