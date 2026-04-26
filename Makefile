.PHONY: build docker-build docker-push helm-package lint fmt test clean help run-local run-streamer

# Variables
BINARY_NAME=telemetry-streamer
DOCKER_IMAGE?=telemetry-streaming
DOCKER_TAG?=0.0.1
DOCKER_REGISTRY?=docker.io
CHART_NAME=telemetry-streaming
CHART_VERSION?=1.0.0

# Local run configuration
CSV_FILE_PATH?=data1.csv
READ_SPEED?=5
BROKER_ADDRESSES?=localhost:9091
TOPIC?=telemetry-data
REPLICATION_FACTOR?=3
BATCH_SIZE?=50

# Build targets
build: ## Build the Go binary
	@echo "Building $(BINARY_NAME)..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o bin/$(BINARY_NAME) ./cmd/telemetry-streamer
	@echo "Build complete: bin/$(BINARY_NAME)"

build-local: ## Build for local machine
	@echo "Building $(BINARY_NAME) for local..."
	go build -o bin/$(BINARY_NAME) ./cmd/telemetry-streamer
	@echo "Build complete: bin/$(BINARY_NAME)"

# Docker targets
docker-build: ## Build Docker image
	@echo "Building Docker image: $(DOCKER_IMAGE):$(DOCKER_TAG)"
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest
	@echo "Docker image built successfully"

docker-push: docker-build ## Push Docker image to registry
	@echo "Pushing Docker image to registry..."
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):$(DOCKER_TAG)
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):latest
	docker push $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):latest
	@echo "Push complete"

# docker-run: docker-build ## Run Docker container locally
# 	@echo "Running Docker container..."
# 	docker run -d \
# 		-p 8080:8080 \
# 		-v $(PWD)/data:/data \
# 		-e LOG_LEVEL=debug \
# 		-e READ_SPEED=10 \
# 		-e CSV_FILE_PATH=/data/sample.csv \
# 		--name $(BINARY_NAME) \
# 		$(DOCKER_IMAGE):$(DOCKER_TAG)
# 	@echo "Container started. Access health at http://localhost:8080/health"

docker-run: docker-build ## Run Docker container in e2e network
	@echo "Running Docker container in tsdb-network..."
	docker run -d --name telemetry-streaming --network=tsdb-network \
		-p 8081:8080 \
		-v .:/app/data \
		-e LOG_LEVEL=debug \
		-e READ_SPEED=$(READ_SPEED) \
		-e CSV_FILE_PATH=/app/data/$(CSV_FILE_PATH) \
		-e BROKER_ADDRESSES=messagebroker:9092 \
		-e TOPIC=$(TOPIC) \
		-e REPLICATION_FACTOR=$(REPLICATION_FACTOR) \
		-e BATCH_SIZE=$(BATCH_SIZE) \
		$(DOCKER_IMAGE):$(DOCKER_TAG)
	@echo "Container started in docker network"
	@echo "  Service: telemetry-streaming"
	@echo "  Network: tsdb-network"
	@echo "  Health: http://localhost:8080/health"

docker-stop: ## Stop running Docker container
	docker stop telemetry-streaming || true
	docker rm telemetry-streaming || true

# Helm targets
helm-package: ## Package Helm chart
	@echo "Packaging Helm chart: $(CHART_NAME)-$(CHART_VERSION).tgz"
	cd deployment && helm package --version $(CHART_VERSION) ./helm/$(CHART_NAME)
	@echo "Helm chart packaged successfully"

helm-lint: ## Lint Helm chart
	@echo "Linting Helm chart..."
	cd deployment/helm && helm lint $(CHART_NAME)
	@echo "Helm chart linting complete"

helm-install: helm-package ## Install Helm chart to Kubernetes
	@echo "Installing Helm chart..."
	helm install $(CHART_NAME) deployment/helm/$(CHART_NAME) \
		--namespace telemetry \
		--create-namespace \
		--values deployment/helm/$(CHART_NAME)/values.yaml
	@echo "Helm chart installed"

helm-upgrade: helm-package ## Upgrade Helm chart in Kubernetes
	@echo "Upgrading Helm chart..."
	helm upgrade $(CHART_NAME) deployment/helm/$(CHART_NAME) \
		--namespace telemetry \
		--values deployment/helm/$(CHART_NAME)/values.yaml
	@echo "Helm chart upgraded"

# Local run targets (for development and testing)
run-local: build-local ## Build and run streamer locally
	@echo "Starting telemetry-streaming service..."
	@echo "CSV File: $(CSV_FILE_PATH)"
	@echo "Read Speed: $(READ_SPEED) msgs/sec"
	@echo "Broker: $(BROKER_ADDRESSES)"
	@echo "Topic: $(TOPIC)"
	@CSV_FILE_PATH=$(CSV_FILE_PATH) READ_SPEED=$(READ_SPEED) BROKER_ADDRESSES=$(BROKER_ADDRESSES) TOPIC=$(TOPIC) REPLICATION_FACTOR=$(REPLICATION_FACTOR) BATCH_SIZE=$(BATCH_SIZE) ./bin/$(BINARY_NAME)

run-streamer: build-local ## Run streamer (same as run-local)
	@CSV_FILE_PATH=$(CSV_FILE_PATH) READ_SPEED=$(READ_SPEED) BROKER_ADDRESSES=$(BROKER_ADDRESSES) TOPIC=$(TOPIC) REPLICATION_FACTOR=$(REPLICATION_FACTOR) BATCH_SIZE=$(BATCH_SIZE) ./bin/$(BINARY_NAME)

# Code quality targets
fmt: ## Format Go code
	@echo "Formatting Go code..."
	go fmt ./...
	@echo "Format complete"

lint: ## Run Go linter
	@echo "Running Go linter..."
	golangci-lint run ./...
	@echo "Linting complete"

test: ## Run tests
	@echo "Running tests..."
	go test -v ./...
	@echo "Tests complete"

# Setup targets
mod-tidy: ## Tidy Go modules
	@echo "Tidying Go modules..."
	go mod tidy
	@echo "Modules tidied"

mod-download: ## Download Go modules
	@echo "Downloading Go modules..."
	go mod download
	@echo "Modules downloaded"

# Cleanup
clean: docker-stop ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f *.tgz
	@echo "Clean complete"

# Help target
help: ## Display this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "%-20s %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
