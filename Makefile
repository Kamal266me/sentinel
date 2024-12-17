.PHONY: build test clean docker deploy-local lint

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
GOFLAGS := -trimpath

# Binary output
BINDIR := bin

# All commands
CMDS := telemetry predictor

all: build

build: $(CMDS)

$(CMDS):
	@echo "Building $@..."
	@mkdir -p $(BINDIR)
	CGO_ENABLED=0 go build $(GOFLAGS) $(LDFLAGS) -o $(BINDIR)/$@ ./cmd/$@

# Build for Linux ARM64 (edge devices)
build-arm64:
	@echo "Building for linux/arm64..."
	@mkdir -p $(BINDIR)/linux-arm64
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(GOFLAGS) $(LDFLAGS) -o $(BINDIR)/linux-arm64/telemetry ./cmd/telemetry
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(GOFLAGS) $(LDFLAGS) -o $(BINDIR)/linux-arm64/predictor ./cmd/predictor

# Build for Linux AMD64
build-amd64:
	@echo "Building for linux/amd64..."
	@mkdir -p $(BINDIR)/linux-amd64
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(GOFLAGS) $(LDFLAGS) -o $(BINDIR)/linux-amd64/telemetry ./cmd/telemetry
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(GOFLAGS) $(LDFLAGS) -o $(BINDIR)/linux-amd64/predictor ./cmd/predictor

build-all: build-arm64 build-amd64

test:
	go test -v -race ./...

test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BINDIR)
	rm -f coverage.out coverage.html

# Docker builds
DOCKER_REPO ?= ghcr.io/aqstack/aeop
DOCKER_TAG ?= $(VERSION)

docker-build:
	docker build -t $(DOCKER_REPO)/telemetry:$(DOCKER_TAG) -f deploy/docker/Dockerfile.telemetry .
	docker build -t $(DOCKER_REPO)/predictor:$(DOCKER_TAG) -f deploy/docker/Dockerfile.predictor .

docker-push: docker-build
	docker push $(DOCKER_REPO)/telemetry:$(DOCKER_TAG)
	docker push $(DOCKER_REPO)/predictor:$(DOCKER_TAG)

# Local K3s deployment
deploy-local:
	kubectl apply -k deploy/helm/aeop

undeploy-local:
	kubectl delete -k deploy/helm/aeop

# Development helpers
run-telemetry: build
	./$(BINDIR)/telemetry -node=dev-node

run-predictor: build
	./$(BINDIR)/predictor -node=dev-node

# Generate training data from running cluster
collect-training-data:
	@echo "Collecting training data..."
	@mkdir -p data/training
	curl -s localhost:9101/metrics/latest >> data/training/metrics-$$(date +%s).json

# Chaos testing
chaos-test:
	@echo "Running chaos tests..."
	go test -v -tags=chaos ./test/chaos/...

# Documentation
docs:
	@echo "Generating documentation..."
	go doc -all ./pkg/... > docs/api.txt

.DEFAULT_GOAL := build
