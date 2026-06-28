.PHONY: build test lint run help docker-build clean

# Variables
APP_NAME=skypanel
CMD_DIR=./cmd/panel
MAIN_FILE=$(CMD_DIR)/main.go

help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the application
	go build -o bin/$(APP_NAME) $(MAIN_FILE)

run: ## Run the application locally
	go run $(MAIN_FILE)

test: ## Run all tests
	go test -v ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

fmt: ## Run gofmt
	gofmt -w .

docker-build: ## Build the Docker image
	docker compose build

clean: ## Clean build files
	rm -rf bin/
