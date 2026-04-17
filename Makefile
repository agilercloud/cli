VERSION = $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build test vet tidy release-dry clean

build:
	go build -trimpath -ldflags "-s -w -X main.Version=$(VERSION)" -o agiler .

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

release-dry:
	goreleaser release --snapshot --clean

clean:
	rm -rf dist agiler
