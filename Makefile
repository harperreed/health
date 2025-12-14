.PHONY: build test test-race test-coverage install clean run-mcp lint

build:
	go build -o health ./cmd/health

test:
	go test ./internal/... -v && go test ./test/... -v

test-race:
	go test -race ./...

test-coverage:
	go test -coverprofile=coverage.out -covermode=atomic ./internal/...
	go tool cover -html=coverage.out -o coverage.html

install:
	go install ./cmd/health

clean:
	rm -f health coverage.out coverage.html
	go clean

run-mcp: build
	./health mcp

lint:
	golangci-lint run

.DEFAULT_GOAL := build
