FRONTEND_DIR = ./web
BACKEND_DIR = .

.PHONY: all build-frontend build-backend start-backend test-backend

all: build-frontend start-backend

build-frontend:
	@echo "Building frontend..."
	@cd $(FRONTEND_DIR) && bun install && DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$(cat VERSION) bun run build

build-backend:
	@echo "Building backend..."
	@cd $(BACKEND_DIR) && go build -ldflags "-X 'github.com/NookMux/NookMux/internal/common.Version=$(shell git rev-parse HEAD)'" -o NookMux ./cmd/server

test-backend:
	@echo "Testing backend..."
	@cd $(BACKEND_DIR) && ./scripts/go-test-backend.sh

start-backend:
	@echo "Starting backend dev server..."
	@cd $(BACKEND_DIR) && go run ./cmd/server &

setup-hooks:
	@chmod +x scripts/ci-check.sh .githooks/pre-commit
	@git config core.hooksPath .githooks
	@echo "Git pre-commit hooks configured successfully."

check-ci:
	@./scripts/ci-check.sh
