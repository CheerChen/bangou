REGISTRY_URL ?= 192.168.0.110:5000
TAG ?= latest
PLATFORM ?= linux/arm64
API_IMAGE ?= $(REGISTRY_URL)/bangou-api:$(TAG)
WEB_IMAGE ?= $(REGISTRY_URL)/bangou-web:$(TAG)
RELEASE_SHA := $(shell git rev-parse --short HEAD)

.PHONY: help test build run dev docker-build docker-push release release-sha compose-up compose-down clean

help:
	@echo "Targets:"
	@echo "  make test          - Run Go tests"
	@echo "  make build         - Build Go binary"
	@echo "  make run           - Run Go API server locally"
	@echo "  make dev           - Start Go API + Vite dev server"
	@echo "  make docker-build  - Build Docker images (API + Web)"
	@echo "  make docker-push   - Push Docker images"
	@echo "  make release       - Build and push both images"
	@echo "  make release-sha   - Build and push both images tagged with git SHA"
	@echo "  make compose-up    - Start services with docker compose"
	@echo "  make compose-down  - Stop services"
	@echo "  make clean         - Remove build artifacts"

test:
	go test ./checker/ ./committed/ ./executor/ ./parser/ ./provider/ ./scanner/ ./staging/ ./store/ ./web/

build:
	go build -o bangou .

run:
	go run . --db ./bangou.db --listen :8080

dev:
	@echo "Go API on :8080, Vite on :5173 (proxy /api -> :8080)"
	@echo "Open http://localhost:5173"
	@go run . --db ./bangou.db --listen :8080 & \
	cd frontend && npm run dev

docker-build:
	DOCKER_BUILDKIT=1 docker build --platform $(PLATFORM) -t $(API_IMAGE) -f Dockerfile .
	DOCKER_BUILDKIT=1 docker build --platform $(PLATFORM) -t $(WEB_IMAGE) -f Dockerfile.frontend .

docker-push:
	docker push $(API_IMAGE)
	docker push $(WEB_IMAGE)

release: docker-build docker-push
	@echo "Released: $(API_IMAGE) $(WEB_IMAGE)"

release-sha:
	$(MAKE) release TAG=$(RELEASE_SHA)

compose-up:
	API_IMAGE=$(API_IMAGE) WEB_IMAGE=$(WEB_IMAGE) docker compose up -d --build

compose-down:
	docker compose down

clean:
	rm -f bangou
	rm -rf frontend/dist frontend/node_modules
