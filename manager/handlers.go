package manager

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

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
		msg := fmt.Sprintf("[ManagerAPI] Could not decode request body: %v\n", err)
		log.Print(msg)

		w.WriteHeader(400)
		e := ErrResponse{
			HTTPStatusCode: 400,
			Message:        msg,
		}
		json.NewEncoder(w).Encode(e)
		return
	}

	a.Manager.AddTaskEvent(taskEvent)
	log.Printf("[ManagerAPI] Added taskEvent [%v]\n", taskEvent.Task.ID)
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(taskEvent.Task)
}

func (a *Api) GetTasksHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(a.Manager.GetTasks())
}

func (a *Api) StopTaskHandler(w http.ResponseWriter, req *http.Request) {
	taskID := chi.URLParam(req, "taskID")
	if taskID == "" {
		log.Printf("[ManagerAPI] No taskID passed in request\n")
		w.WriteHeader(400)
		return
	}

	tID, _ := uuid.Parse(taskID)
	taskToStop, ok := a.Manager.TaskDb[tID]
	if !ok {
		log.Printf("[ManagerAPI] No task with ID [%v] found\n", tID)
		w.WriteHeader(404)
		return
	}

	taskEvent := task.TaskEvent{
		ID:        uuid.New(),
		State:     task.Completed,
		Timestamp: time.Now(),
	}

	taskCopy := *taskToStop
	taskCopy.State = task.Completed
	taskEvent.Task = taskCopy
	a.Manager.AddTaskEvent(taskEvent)

	log.Printf("[ManagerAPI] Added task [%v] to stop container [%v]\n", taskToStop.ID, taskToStop.ContainerID)
	w.WriteHeader(204)
}
