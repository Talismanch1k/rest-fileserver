container_runtime := $(shell which podman || which docker)

$(info using ${container_runtime})

up: down
	${container_runtime} compose up --build -d

down:
	${container_runtime} compose down

run-tests: 
	${container_runtime} run --rm --network=host tests:latest

test:
	make down
	make up
	make run-tests
	make down
	@echo "test finished"

lint:
	make -C hello lint
	make -C fileserver lint

tools:
	go install golang.org/x/tools/cmd/goimports@latest
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $$(go env GOPATH)/bin v2.4.0

