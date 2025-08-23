# Frequent commands

## tesseract
`TESSERACT_HOST=localhost TESSERACT_PORT=5555 go run main.go`

### tesseract.worker
`curl http://localhost:5555/tasks | jq .`
`curl -v --request DELETE "localhost:5555/tasks/390b481c-a45e-41db-ab94-27a8e07b97de"`

## Docker
`docker ps --format "table {{.ID}}\t{{.Image}}\t{{.Status}}\t{{.Names}}"`
`docker inspect -f '{{.State.Pid}}' ContainerID`