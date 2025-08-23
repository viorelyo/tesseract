# Frequent commands

## tesseract
`TESSERACT_WORKER_HOST=localhost TESSERACT_WORKER_PORT=5555 TESSERACT_MANAGER_HOST=localhost TESSERACT_MANAGER_PORT=5556 go run main.go`

### tesseract.worker
`curl http://localhost:5556/tasks | jq .`
`curl -v --request POST --header 'Content-Type: application/json' --data @task_event.json localhost:5556/tasks`
`curl -v --request DELETE "localhost:5556/tasks/390b481c-a45e-41db-ab94-27a8e07b97de"`

## Docker
`docker ps --format "table {{.ID}}\t{{.Image}}\t{{.Status}}\t{{.Names}}"`
`docker inspect -f '{{.State.Pid}}' ContainerID`
`docker rm -f %id%` : stop and remove the container with %id%