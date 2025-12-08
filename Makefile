# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=posthog-proxy
BINARY_PATH=./cmd/server

# Docker parameters
DOCKER_IMAGE=posthog-proxy
DOCKER_TAG=latest

all: deps test build

build:
	$(GOBUILD) -o $(BINARY_NAME) -v $(BINARY_PATH)

test:
	$(GOTEST) -v ./...

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

run: deps build
	./$(BINARY_NAME)

deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Docker commands
docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-run:
	docker run --rm -p 8080:8080 --env-file .env $(DOCKER_IMAGE):$(DOCKER_TAG)

docker-push:
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

# Development
dev:
	$(GOCMD) run $(BINARY_PATH)/main.go

format:
	$(GOCMD) fmt ./...

lint:
	golangci-lint run

# Testing
test-unit:
	$(GOTEST) -short -v ./...

test-integration:
	$(GOTEST) -v ./tests/...

coverage:
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out

# Build for multiple platforms
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_NAME)-linux-amd64 -v $(BINARY_PATH)

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BINARY_NAME)-windows-amd64.exe -v $(BINARY_PATH)

build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BINARY_NAME)-darwin-amd64 -v $(BINARY_PATH)

build-all: build-linux build-windows build-darwin

.PHONY: all build test clean run deps docker-build docker-run docker-push dev format lint test-unit test-integration coverage build-linux build-windows build-darwin build-all