.PHONY: build run lint fmt clean

BINARY=mark-guard

build:
	go build -o bin/$(BINARY) ./cmd/mark-guard

run:
	go run ./cmd/mark-guard format

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

clean:
	rm -rf bin/