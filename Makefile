# ====================================================================================
# VARIABLES
# ====================================================================================

# Set the name of the binary
BINARY_NAME=go-shop

# Get the module path
GO_MODULE=$(shell go list -m)


# ====================================================================================
# HELP
# ====================================================================================

.PHONY: help
help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)


# ====================================================================================
# DEVELOPMENT
# ====================================================================================

.PHONY: run
run: ## Run the web application
	@echo ">> Starting application..."
	@pnpm tailwindcss -i ./ui/static/css/input.css -o ./ui/static/css/main.css &
	go run ./cmd/web

.PHONY: tidy
tidy: ## Tidy and verify Go modules
	@echo ">> Tidying Go modules..."
	go mod tidy
	go mod verify

.PHONY: test
test: ## Run all tests
	@echo ">> Running tests..."
	go test ./... -v

.PHONY: coverage
coverage: ## Run tests and show coverage
	@echo ">> Running tests with coverage..."
	go test ./... -cover

.PHONY: vet
vet: ## Vet the Go code for common issues
	@echo ">> Vetting code..."
	go vet ./...


# ====================================================================================
# DATABASE
# ====================================================================================

.PHONY: db/reset
db/reset: ## Delete the database and re-create schema and seed data
	@echo ">> Resetting database..."
	@rm -f ecommerce.db
	@sqlite3 ecommerce.db < schema.sql
	@sqlite3 ecommerce.db < seed.sql
	@echo ">> Database reset and seeded successfully."

.PHONY: db/seed
db/seed: ## Run the seed script on the existing database
	@echo ">> Seeding database..."
	@sqlite3 ecommerce.db < seed.sql
	@echo ">> Seeding complete."

.PHONY: db/console
db/console: ## Open a console to the SQLite database
	@echo ">> Opening database console..."
	sqlite3 ecommerce.db


# ====================================================================================
# BUILD
# ====================================================================================

.PHONY: build
build: ## Build the application binary
	@echo ">> Building application binary..."
	pnpm tailwindcss -i ./ui/static/css/input.css -o ./ui/static/css/main.css --minify
	go build -o $(BINARY_NAME) ./cmd/web
	@echo ">> Build complete: ./$(BINARY_NAME)"
