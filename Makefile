.PHONY: run build test coverage lint clean

# Run the server
run:
	go run ./cmd/server

# Build the binary
build:
	go build -o bin/support-agent ./cmd/server

# Run all tests
test:
	go test ./... -v

# Run tests with coverage report
coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with coverage summary in terminal
coverage-summary:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

# Run the race detector
race:
	go test -race ./...

# Format code
fmt:
	go fmt ./...

# Tidy dependencies
tidy:
	go mod tidy

# Clean build artifacts
clean:
	rm -rf bin/ coverage.out coverage.html