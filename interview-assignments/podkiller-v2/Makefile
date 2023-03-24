BINARY_NAME=app

build:
	go clean
	rm ${BINARY_NAME}
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -o ${BINARY_NAME} main.go
	eval $(minikube docker-env)
	docker build -t in-cluster:0.1.2 .

