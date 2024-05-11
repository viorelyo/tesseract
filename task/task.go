package task

import (
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
)

type State int

const (
	Pending State = iota
	Scheduled
	Running
	Completed
	Failed
)

type Task struct {
	ID            uuid.UUID
	Name          string
	State         State
	Image         string // Docker image
	Memory        int
	Disk          int
	ExposedPorts  nat.PortSet       // used by Docker to ensure the machine allocates the proper network ports for the task and that it is available on the network
	PortBindings  map[string]string // used by Docker
	RestartPolicy string            // will tell the system what to do when a task stops or fails unexpectedly
	StartTime     time.Time
	FinishTime    time.Time
}

type TaskEvent struct {
	ID        uuid.UUID
	State     State
	Timestamp time.Time // the time the event was requested
	Task      Task
}
