VERSION = $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# Public OpenAPI spec lives in the sibling platform/ repo. The CLI's Go
# client (internal/publicapi/client.gen.go) is generated from it via the
# `openapi` target below; freshness is enforced by a unit test.
PUBLIC_SPEC ?= ../platform/api/openapi/public.json

# Pinned generator. Matches the platform repo so the same tool produces
# the same output regardless of which repo's `openapi` target runs.
OAPI_CODEGEN := github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.0

.PHONY: build test vet fmt lint vulncheck tidy release-dry clean openapi openapi\:check

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

# Regenerate internal/publicapi/client.gen.go from the platform's
# public.json. Requires a sibling platform/ checkout (or override
# PUBLIC_SPEC). Commit the result.
openapi:
	cd internal/publicapi && GOFLAGS=-mod=mod go run $(OAPI_CODEGEN) -config gen.yaml ../../$(PUBLIC_SPEC)

# Fast loop for verifying the committed client matches the spec, without
# pulling in the rest of the test suite.
openapi\:check:
	go test -run TestPublicAPIClientIsFresh ./internal/publicapi/...
