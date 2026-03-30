REGISTRY_URL ?= 192.168.0.110:5000
IMAGE_NAME ?= bangou
TAG ?= latest
PLATFORM ?= linux/arm64
IMAGE ?= $(REGISTRY_URL)/$(IMAGE_NAME):$(TAG)

.PHONY: help test build run docker-build docker-push release compose-up compose-down

help:
	@echo "Targets:"
	@echo "  make test          - Run Go tests"
	@echo "  make build         - Build local binary"
	@echo "  make run           - Run app locally"
	@echo "  make docker-build  - Build Docker image ($(IMAGE))"
	@echo "  make docker-push   - Push Docker image ($(IMAGE))"
	@echo "  make release       - Build and push image to registry"
	@echo "  make compose-up    - Start services with docker compose"
	@echo "  make compose-down  - Stop services"

test:
	go test ./...

build:
	go build -o bangou .

run:
	go run . --db ./bangou.db --listen :8080

docker-build:
	docker build --platform $(PLATFORM) -t $(IMAGE) .

docker-push:
	docker push $(IMAGE)

release: docker-build docker-push
	@echo "Released: $(IMAGE)"

compose-up:
	IMAGE=$(IMAGE) docker compose up -d --build

compose-down:
	docker compose down
