package worker

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/viorelyo/tesseract/task"
)

func (a *Api) StartTaskHandler(w http.ResponseWriter, req *http.Request) {
	d := json.NewDecoder(req.Body)
	d.DisallowUnknownFields()

	taskEvent := task.TaskEvent{}
	err := d.Decode(&taskEvent)
	if err != nil {
		msg := fmt.Sprintf("Could not decode request body: %v\n", err)
		log.Printf(msg)

		w.WriteHeader(400)
		e := ErrResponse{
			HTTPStatusCode: 400,
			Message:        msg,
		}
		json.NewEncoder(w).Encode(e)
		return
	}

	a.Worker.AddTask(taskEvent.Task)
	log.Printf("Added task [%v]\n", taskEvent.Task.ID)
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(taskEvent.Task)
}

func (a *Api) GetTasksHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(a.Worker.GetTasks())
}

func (a *Api) StopTaskHandler(w http.ResponseWriter, req *http.Request) {
	taskID := chi.URLParam(req, "taskID")
	if taskID == "" {
		log.Printf("No taskID passed in reques.\n")
		w.WriteHeader(400)
		return
	}

	tID, _ := uuid.Parse(taskID)
	_, ok := a.Worker.Db[tID]
	if !ok {
		log.Printf("No task with ID [%v] found", tID)
		w.WriteHeader(404)
		return
	}

	taskToStop := a.Worker.Db[tID]
	taskCopy := *taskToStop
	taskCopy.State = task.Completed
	a.Worker.AddTask(taskCopy)

	log.Printf("Added task [%v] to stop container [%v]\n", taskToStop.ID, taskToStop.ContainerID)
	w.WriteHeader(204)
}
