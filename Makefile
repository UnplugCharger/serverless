.PHONY: build run clean test submit execute list get-function delete-function

# Build variables
BINARY_NAME=serverless

# Server variables
SERVER_PORT=8080
SERVER_URL=http://localhost:$(SERVER_PORT)

# Default values for commands
ZIP_FILE?=image_processor.zip
FUNCTION_ID?=df937958-82f3-48e4-a855-ffaf16d95247

build:
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BINARY_NAME)

run: build
	@echo "Starting server on port $(SERVER_PORT)..."
	@./$(BINARY_NAME)

dev:
	@echo "Starting development server with auto-reload..."
	@go run main.go

clean:
	@echo "Cleaning up..."
	@rm -f $(BINARY_NAME)
	@go clean

test:
	@echo "Running tests..."
	@go test ./... -v

# Function management commands
submit:
	@echo "Submitting function from $(ZIP_FILE)..."
	@curl -X POST -F "code=@$(ZIP_FILE)" -F "name=test-function" $(SERVER_URL)/api/submit

execute:
	@echo "Executing function $(FUNCTION_ID)..."
	@curl -s "$(SERVER_URL)/api/execute?functionId=$(FUNCTION_ID)"

execute-post:
	@echo "Executing function $(FUNCTION_ID) with POST..."
	@curl -s -X POST -H "Content-Type: application/json" \
		-d '{"functionId":"$(FUNCTION_ID)","input":{"param1":"value1","param2":"value2"}}' \
		$(SERVER_URL)/api/execute

list:
	@echo "Listing all functions..."
	@curl -s $(SERVER_URL)/api/functions

get-function:
	@echo "Getting function $(FUNCTION_ID)..."
	@curl -s $(SERVER_URL)/api/functions/$(FUNCTION_ID)

delete-function:
	@echo "Deleting function $(FUNCTION_ID)..."
	@curl -s -X DELETE $(SERVER_URL)/api/functions/$(FUNCTION_ID)

health:
	@echo "Checking server health..."
	@curl -s $(SERVER_URL)/health

help:
	@echo "YouTube Serverless Platform - Makefile commands:"
	@echo ""
	@echo "  make build              - Build the application"
	@echo "  make run                - Build and run the application"
	@echo "  make dev                - Run the application in development mode"
	@echo "  make clean              - Clean up build artifacts"
	@echo "  make test               - Run tests"
	@echo ""
	@echo "Function Management:"
	@echo "  make submit ZIP_FILE=file.zip       - Submit a function"
	@echo "  make execute FUNCTION_ID=id         - Execute a function (GET)"
	@echo "  make execute-post FUNCTION_ID=id    - Execute a function (POST with input)"
	@echo "  make list                           - List all functions"
	@echo "  make get-function FUNCTION_ID=id    - Get function details"
	@echo "  make delete-function FUNCTION_ID=id - Delete a function"
	@echo "  make health                         - Check server health"
	@echo ""
