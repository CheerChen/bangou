REGISTRY_URL ?= 192.168.0.110:5000
IMAGE_NAME ?= bangou
TAG ?= latest
PLATFORM ?= linux/arm64
IMAGE ?= $(REGISTRY_URL)/$(IMAGE_NAME):$(TAG)

.PHONY: help test build run frontend dev docker-build docker-push release compose-up compose-down clean

help:
	@echo "Targets:"
	@echo "  make test          - Run Go tests"
	@echo "  make build         - Build frontend + Go binary"
	@echo "  make run           - Run app locally (serves built frontend)"
	@echo "  make frontend      - Build frontend only"
	@echo "  make dev           - Start Go backend + Vite dev server"
	@echo "  make docker-build  - Build Docker image ($(IMAGE))"
	@echo "  make docker-push   - Push Docker image ($(IMAGE))"
	@echo "  make release       - Build and push image to registry"
	@echo "  make compose-up    - Start services with docker compose"
	@echo "  make compose-down  - Stop services"
	@echo "  make clean         - Remove build artifacts"

test:
	go test ./checker/ ./committed/ ./executor/ ./parser/ ./provider/ ./scanner/ ./staging/ ./store/ ./web/

frontend:
	cd frontend && npm ci && npm run build

build: frontend
	go build -o bangou .

run: build
	./bangou --db ./bangou.db --listen :8080

dev:
	@echo "Starting Go backend on :8080 and Vite dev server on :5173"
	@echo "Open http://localhost:5173"
	@go run . --db ./bangou.db --listen :8080 & \
	cd frontend && npm run dev

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

clean:
	rm -f bangou
	rm -rf frontend/dist frontend/node_modules
