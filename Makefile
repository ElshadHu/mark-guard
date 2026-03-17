.PHONY: build run run-init run-generate generate generate-write lint fmt clean test test-v cover vet check

BINARY=mark-guard

build:
	go build -o bin/$(BINARY) ./cmd/mark-guard

run:
	go run ./cmd/mark-guard $(if $(ARGS),$(ARGS),format)

run-init:
	go run ./cmd/mark-guard init $(ARGS)

run-generate:
	go run ./cmd/mark-guard generate $(ARGS)

generate:
	go run ./cmd/mark-guard generate $(ARGS)

generate-write:
	go run ./cmd/mark-guard generate --write $(ARGS)

test:
	@if find . -name '*_test.go' | grep -q .; then \
		go test ./...; \
	else \
		echo "No test files found — skipping tests"; \
	fi

test-v:
	go test -v ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@rm -f coverage.out

vet:
	go vet ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

check: vet lint test

clean:
	rm -rf bin/