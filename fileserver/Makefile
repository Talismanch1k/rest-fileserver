IMAGE_NAME  ?= file-server
IMAGE_TAG   ?= latest
CONTAINER   ?= file-server
PORT        ?= 1235

.PHONY: lint build run stop restart logs clean

lint:
	golangci-lint run -E gocritic -v ./...

build:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

up: build
	docker run -d --name $(CONTAINER) -p $(PORT):$(PORT) -e FILESERVER_PORT=$(PORT) $(IMAGE_NAME):$(IMAGE_TAG)

down:
	docker stop $(CONTAINER) && docker rm $(CONTAINER)

restart: down up

logs:
	docker logs -f $(CONTAINER)

clean: stop
	docker rmi $(IMAGE_NAME):$(IMAGE_TAG)

