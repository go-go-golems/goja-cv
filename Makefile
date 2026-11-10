.PHONY: all lint lintmax golangci-lint-install glazed-lint-build glazed-lint staticcheck test test-race vet fmt fmt-check build build-bin smoke testdata check clean tidy docker-lint gosec govulncheck logcopter-generate logcopter-check goreleaser goreleaser-check tag-major tag-minor tag-patch release bump-go-go-golems install

BINARY ?= goja-cv
MODULE ?= github.com/go-go-golems/goja-cv
CMD_DIR ?= ./cmd/$(BINARY)
GO_PACKAGES ?= ./...
LOGCOPTER_PACKAGES ?= ./cmd/... ./pkg/...

GORELEASER_ARGS ?= --skip=sign --snapshot --clean
GORELEASER_TARGET ?= --single-target
GOLANGCI_LINT_VERSION ?= $(shell cat .golangci-lint-version)
GOLANGCI_LINT_BIN ?= $(CURDIR)/.bin/golangci-lint
GLAZED_LINT_BIN ?= $(CURDIR)/.bin/glazed-lint
GLAZED_LINT_PKG ?= github.com/go-go-golems/glazed/cmd/tools/glazed-lint
GLAZED_VERSION ?= $(shell GOWORK=off go list -m -f '{{.Version}}' github.com/go-go-golems/glazed 2>/dev/null)
GLAZED_LINT_DIRS ?= ./cmd/... ./pkg/...
# Known exception: the goja-cv commands still declare their flags with raw Cobra
# (cmd.Flags().StringVar...) instead of Glazed fields, and ops.go extends Glazed's
# own help command with --json. Porting them to cmds.WithFlags(fields.New(...))
# changes the CLI surface (Glazed adds its command-settings flags) and is
# tracked as a follow-up; until then these four files are skipped. pkg/ and new
# files under cmd/ are still checked.
GLAZED_LINT_FLAGS ?= -glazedclilint.allow-paths=cmd/goja-cv/eval.go,cmd/goja-cv/mcp.go,cmd/goja-cv/ops.go,cmd/goja-cv/root.go

# Scratch directory for the smoke test (generated image, looks, ops/eval output).
SMOKE_DIR ?= $(CURDIR)/dist/smoke

all: lint test build

docker-lint:
	docker run --rm -v $(shell pwd):/app -w /app golangci/golangci-lint:$(GOLANGCI_LINT_VERSION) golangci-lint run -v $(GO_PACKAGES)

golangci-lint-install:
	mkdir -p $(dir $(GOLANGCI_LINT_BIN))
	GOBIN=$(dir $(GOLANGCI_LINT_BIN)) GOWORK=off go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

glazed-lint-build:
	@echo "Building glazed-lint from the selected Glazed module..."
	@mkdir -p $(dir $(GLAZED_LINT_BIN))
	@if [ -n "$(GLAZED_VERSION)" ] && [ "$(GLAZED_VERSION)" != "(devel)" ]; then \
		echo "Installing $(GLAZED_LINT_PKG)@$(GLAZED_VERSION)"; \
		GOBIN=$(dir $(GLAZED_LINT_BIN)) GOWORK=off go install $(GLAZED_LINT_PKG)@$(GLAZED_VERSION); \
	else \
		echo "Installing $(GLAZED_LINT_PKG) from workspace/module"; \
		GOBIN=$(dir $(GLAZED_LINT_BIN)) GOWORK=off go install $(GLAZED_LINT_PKG); \
	fi

glazed-lint: glazed-lint-build
	GOWORK=off go vet -vettool=$(GLAZED_LINT_BIN) $(GLAZED_LINT_FLAGS) $(GLAZED_LINT_DIRS)

lint: golangci-lint-install glazed-lint-build
	GOWORK=off $(GOLANGCI_LINT_BIN) run -v $(GO_PACKAGES)
	GOWORK=off go vet -vettool=$(GLAZED_LINT_BIN) $(GLAZED_LINT_FLAGS) $(GLAZED_LINT_DIRS)

lintmax: golangci-lint-install glazed-lint-build
	GOWORK=off $(GOLANGCI_LINT_BIN) run -v --max-same-issues=100 $(GO_PACKAGES)
	GOWORK=off go vet -vettool=$(GLAZED_LINT_BIN) $(GLAZED_LINT_FLAGS) $(GLAZED_LINT_DIRS)

staticcheck:
	GOWORK=off go run honnef.co/go/tools/cmd/staticcheck@2025.1.1 $(GO_PACKAGES)

fmt:
	gofmt -w $(shell git ls-files '*.go')

fmt-check: golangci-lint-install
	$(GOLANGCI_LINT_BIN) fmt --diff

vet:
	GOWORK=off go vet $(GO_PACKAGES)

test:
	GOWORK=off go test $(GO_PACKAGES) -count=1

test-race:
	GOWORK=off go test $(GO_PACKAGES) -count=1 -race

build:
	GOWORK=off go generate ./...
	GOWORK=off go build $(GO_PACKAGES)

build-bin:
	mkdir -p ./dist
	GOWORK=off go build -o ./dist/$(BINARY) $(CMD_DIR)

# End-to-end check of the real CLI: help pages, the op catalogue, an eval on a
# generated test image that must write a look file (the path the CLI reports
# in its --json files list), and the cv-error exit code.
smoke: build-bin
	rm -rf $(SMOKE_DIR) && mkdir -p $(SMOKE_DIR)
	GOWORK=off go run ./internal/testimage/gen -out $(SMOKE_DIR)
	./dist/$(BINARY) --help >/dev/null
	./dist/$(BINARY) help --list >/dev/null
	./dist/$(BINARY) help concepts >/dev/null
	./dist/$(BINARY) ops --json > $(SMOKE_DIR)/ops.json
	grep -q '"name": "levels"' $(SMOKE_DIR)/ops.json
	./dist/$(BINARY) eval --root $(SMOKE_DIR) --image $(SMOKE_DIR)/gradient-64x48-8bit.png --out $(SMOKE_DIR)/looks --json \
		-e 'const s = await img.stats(); return { meanL: s.meanL, fixed: img.levels({ black: s.p01, white: s.p99 }).look("levels") }' \
		> $(SMOKE_DIR)/eval.json
	jq -e '.status == "ok" and (.files | length) == 1' $(SMOKE_DIR)/eval.json >/dev/null
	test -s "$$(jq -r '.files[0].path' $(SMOKE_DIR)/eval.json)"
	@status=0; ./dist/$(BINARY) eval -e 'return cv.nope()' >/dev/null || status=$$?; \
		if [ "$$status" -ne 1 ]; then echo "expected exit 1 for a cv error, got $$status"; exit 1; fi
	@status=0; ./dist/$(BINARY) mcp </dev/null >/dev/null 2>$(SMOKE_DIR)/mcp.err || status=$$?; \
		if [ "$$status" -eq 0 ] || ! grep -q -- '--root' $(SMOKE_DIR)/mcp.err; then echo "expected mcp without --root to be a usage error, got $$status"; exit 1; fi
	@echo "smoke ok"

# Regenerate the synthetic images under testdata/.
testdata:
	GOWORK=off go run ./internal/testimage/gen -out testdata

check: vet test

clean:
	rm -rf ./dist ./.bin

tidy:
	GOWORK=off go mod tidy

gosec:
	GOWORK=off go install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec -exclude-generated -exclude=G101,G304,G301,G306 -exclude-dir=.history ./...

govulncheck:
	GOWORK=off go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

logcopter-generate:
	GOWORK=off go generate ./...

logcopter-check:
	GOWORK=off go tool logcopter-gen -area-prefix go-go-golems.goja-cv -strip-prefix $(MODULE) -check $(LOGCOPTER_PACKAGES)

goreleaser:
	GOWORK=off goreleaser release $(GORELEASER_ARGS) $(GORELEASER_TARGET)

goreleaser-check:
	goreleaser check --config .goreleaser.yaml

tag-major:
	git tag $(shell svu major)

tag-minor:
	git tag $(shell svu minor)

tag-patch:
	git tag $(shell svu patch)

release:
	git push origin --tags
	GOWORK=off GOPROXY=proxy.golang.org go list -m $(MODULE)@$(shell svu current)

bump-go-go-golems:
	@deps="$$(awk '/^require[[:space:]]+github\.com\/go-go-golems\// { print $$2 } /^[[:space:]]*github\.com\/go-go-golems\// { print $$1 }' go.mod | sort -u)"; \
	if [ -z "$$deps" ]; then \
		echo "No github.com/go-go-golems dependencies in go.mod"; \
	else \
		echo "Bumping go-go-golems dependencies:"; \
		echo "$$deps"; \
		for dep in $$deps; do GOWORK=off go get "$${dep}@latest"; done; \
	fi
	GOWORK=off go mod tidy

install:
	GOWORK=off go install $(CMD_DIR)
