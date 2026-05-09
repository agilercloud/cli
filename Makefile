VERSION = $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build test vet fmt lint vulncheck tidy release-dry clean

build:
	go build -trimpath -ldflags "-s -w -X main.Version=$(VERSION)" -o agiler ./cmd/agiler

test:
	go test -race -cover ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

lint:
	golangci-lint run ./...

vulncheck:
	govulncheck ./...

tidy:
	go mod tidy

release-dry:
	goreleaser release --snapshot --clean

clean:
	rm -rf dist agiler
