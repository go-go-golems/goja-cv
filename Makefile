.PHONY: all test test-race vet fmt lint build testdata check

all: check

test:
	go test ./... -count=1

test-race:
	go test ./... -count=1 -race

vet:
	go vet ./...

fmt:
	gofmt -w $(shell git ls-files '*.go')

lint:
	GOWORK=off golangci-lint run -v

build:
	go build ./...

# Regenerate the synthetic images under testdata/.
testdata:
	go run ./internal/testimage/gen -out testdata

check: vet test
