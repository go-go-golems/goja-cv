.PHONY: all test test-race vet fmt lint staticcheck build testdata check

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

staticcheck:
	go run honnef.co/go/tools/cmd/staticcheck@2025.1.1 ./...

build:
	go build ./...

# Regenerate the synthetic images under testdata/.
testdata:
	go run ./internal/testimage/gen -out testdata

check: vet test
