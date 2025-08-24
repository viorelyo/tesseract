package task

import (
	"context"
	"io"
	"log"
	"math"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
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
	ID             uuid.UUID
	ContainerID    string
	Name           string
	State          State
	Image          string // Docker image
	HealthCheckUrl string
	RestartCount   int
	Cpu            float64
	Memory         int64
	Disk           int64
	ExposedPorts   nat.PortSet // used by Docker to ensure the machine allocates the proper network ports for the task and that it is available on the network
	HostPorts      nat.PortMap
	PortBindings   map[string]string // used by Docker
	RestartPolicy  string            // will tell the Docker daemon what to do when a task (container) stops or fails unexpectedly | TODO make it enum?
	StartTime      time.Time
	FinishTime     time.Time
}

type TaskEvent struct {
	ID        uuid.UUID
	State     State
	Timestamp time.Time // the time the event was requested
	Task      Task
}

// Docker container config
type Config struct {
	Name          string // Container name
	AttachStdin   bool
	AttachStdout  bool
	AttachStderr  bool
	ExposedPorts  nat.PortSet
	Cmd           []string
	Image         string // Container image name
	Cpu           float64
	Memory        int64
	Disk          int64
	Env           []string // Environ varspassed into container
	RestartPolicy string   // https://docs.docker.com/config/containers/start-containers-automatically/#use-a-restart-policy
}

func NewConfig(t *Task) *Config {
	return &Config{
		Name:          t.Name,
		ExposedPorts:  t.ExposedPorts,
		Image:         t.Image,
		Cpu:           t.Cpu,
		Memory:        t.Memory,
		Disk:          t.Disk,
		RestartPolicy: t.RestartPolicy,
	}
}

// TODO maybe refactor struct :/
type DockerResult struct {
	Error       error
	Action      string
	ContainerID string
	Result      string
}

type DockerInspectResponse struct {
	Error         error
	ContainerData *types.ContainerJSON
}

type Docker struct {
	Client *client.Client
	Config Config
}

func NewDocker(c *Config) *Docker {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Printf("[Docker] Could not create docker client: %v\n", err)
		return nil
	}

	return &Docker{
		Client: cli,
		Config: *c,
	}
}

func (d *Docker) Run() DockerResult {
	ctx := context.Background()
	reader, err := d.Client.ImagePull(ctx, d.Config.Image, image.PullOptions{})
	if err != nil {
		log.Printf("[Docker] Could not pull image [%s]: %v\n", d.Config.Image, err)
		return DockerResult{Error: err}
	}
	defer reader.Close()
	io.Copy(os.Stdout, reader)

	restartPolicy := container.RestartPolicy{
		Name: container.RestartPolicyMode(d.Config.RestartPolicy),
	}

	resources := container.Resources{
		Memory:   d.Config.Memory,
		NanoCPUs: int64(d.Config.Cpu * math.Pow(10, 9)),
	}

	containerConfig := container.Config{
		Image:        d.Config.Image,
		Tty:          false,
		Env:          d.Config.Env,
		ExposedPorts: d.Config.ExposedPorts,
	}

	hostConfig := container.HostConfig{
		RestartPolicy:   restartPolicy,
		Resources:       resources,
		PublishAllPorts: true,
	}

	resp, err := d.Client.ContainerCreate(ctx, &containerConfig, &hostConfig, nil, nil, d.Config.Name)
	if err != nil {
		log.Printf("[Docker] Could not create container with image [%s]: %v\n", d.Config.Image, err)
		return DockerResult{Error: err}
	}

	err = d.Client.ContainerStart(ctx, resp.ID, container.StartOptions{})
	if err != nil {
		log.Printf("[Docker] Could not start container [%s]: %v\n", resp.ID, err)
		return DockerResult{Error: err}
	}

	// TODO should ContainerWait?

	out, err := d.Client.ContainerLogs(ctx, resp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		log.Printf("Could not get logs for container [%s]: %v\n", resp.ID, err)
		return DockerResult{Error: err}
	}
	// defer out.Close() // TODO is it required
	stdcopy.StdCopy(os.Stdout, os.Stderr, out)
	return DockerResult{ContainerID: resp.ID, Action: "start", Result: "success", Error: nil}
}

func (d *Docker) Stop(id string) DockerResult {
	log.Printf("[Docker] Stopping container [%s]", id)

	ctx := context.Background()
	err := d.Client.ContainerStop(ctx, id, container.StopOptions{}) // TODO configure timeout if required
	if err != nil {
		log.Printf("Could not stop container [%s]: %v\n", id, err)
		return DockerResult{Error: err}
	}

	err = d.Client.ContainerRemove(ctx, id, container.RemoveOptions{
		RemoveVolumes: true,
		RemoveLinks:   false,
		Force:         false,
	})
	if err != nil {
		log.Printf("[Docker] Could not remove contianer [%s]: %v\n", id, err)
		return DockerResult{Error: err}
	}

	return DockerResult{Action: "stop", Result: "success", Error: nil}
}

func (d *Docker) Inspect(id string) DockerInspectResponse {
	ctx := context.Background()
	response, err := d.Client.ContainerInspect(ctx, id)
	if err != nil {
		log.Printf("[Docker] Error inspecting container: %s\n", err)
		return DockerInspectResponse{Error: err}
	}

	return DockerInspectResponse{ContainerData: &response}
}
