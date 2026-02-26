.PHONY: build run lint fmt clean test

BINARY=mark-guard

build:
	go build -o bin/$(BINARY) ./cmd/mark-guard

run:
	go run ./cmd/mark-guard format

test:
	@if find . -name '*_test.go' | grep -q .; then \
		go test ./...; \
	else \
		echo "No test files found — skipping tests"; \
	fi

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

clean:
	rm -rf bin/