.PHONY: build run test clean deps lint

BINARY=ia-buscar
VERSION=1.0.0
GO=go

build:
	$(GO) build -o bin/$(BINARY) ./cmd/ia-buscar

run: build
	./bin/$(BINARY) -transport stdio

run-http: build
	./bin/$(BINARY) -transport http -http-addr :8080

test:
	$(GO) test ./...

clean:
	rm -rf bin/$(BINARY)

deps:
	$(GO) mod download
	$(GO) mod tidy

lint:
	golangci-lint run

fmt:
	$(GO) fmt ./...