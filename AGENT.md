# Agent Guidelines for goja-cv

goja-cv is a lazy image-analysis and editing engine in Go, exposed to JavaScript as `require("cv")`
through go-go-goja, with a CLI (`cmd/goja-cv`) and an MCP server (`goja-cv mcp`).

## Build Commands

- Run the CLI: `GOWORK=off go run ./cmd/goja-cv ...` (instead of build + ./goja-cv).
- Build: `make build`; distributable binary: `make build-bin` (writes `dist/goja-cv`).
- Test: `make test` (CI also runs `make test-race`); one test: `GOWORK=off go test ./pkg/path -run TestName`.
- End-to-end CLI check: `make smoke`.
- Lint: `make lint` (pinned golangci-lint from `.golangci-lint-version` in `.bin/`, plus glazed-lint).
- Logging: `make logcopter-generate` after adding a package under `cmd/` or `pkg/`; `make logcopter-check` verifies.
- Format: `make fmt`.
- Release config: `goreleaser check --config .goreleaser.yaml`; local snapshot: `make goreleaser`.
- Test images: `go run ./internal/testimage/gen -out DIR [-planted]`.

The README and the guidebook (`pkg/doc`) are tested: every `js` block runs in a real session and every
`goja-cv` command shown must parse. Keep them working when you change commands or ops.

go-go-goja is pinned to a pseudo-version (a PR commit). Do not bump it casually and do not commit a
`replace` directive.

IMPORTANT: To run a server and interact with it (e.g. `goja-cv mcp --http`), use tmux, so it is easy to kill.
Use capture-pane to read the output.

## Code Style Guidelines

- **Imports**: standard library first, third-party next, local packages last.
- **Naming**: CamelCase for exported, camelCase for unexported.
- **Error handling**: return errors with context (`github.com/pkg/errors`); log with zerolog (the generated
  per-package `log` from logcopter, or an injected `*zerolog.Logger`).
- **Commands**: Cobra; help pages are Glazed help sections in `pkg/doc`.
- **Types**: clear interfaces; assert implementations with `var _ Interface = &Foo{}`.

## Project Structure

- `cmd/goja-cv/`: the CLI (run, eval, ops, mcp, help).
- `pkg/cv/`: engine (buffer, parallel, kernels, graph, ops, look, docs).
- `pkg/gojamod/cv/`: the go-go-goja module `cv` and its prelude.
- `pkg/cvsession/`: eval session, result processor, MCP server.
- `pkg/doc/`: the guidebook (Glazed help pages).
- `pkg/xgoja/providers/cv/`: xgoja provider.
- `internal/testimage/`: synthetic test images.
- `examples/`: example configurations.
- `ttmp/`: docmgr ticket workspaces.

<goGuidelines>
- When implementing go interfaces, use the var _ Interface = &Foo{} to make sure the interface is always implemented correctly.
- Always use a context argument when appropriate.
- Use cobra for command-line applications.
- Use the "defaults" package name, instead of "default" package name, as it's reserved in go.
- Use github.com/pkg/errors for wrapping errors.
- When starting goroutines, use errgroup.
- Only use the toplevel go.mod, don't create new ones.
- When using go:embed, import embed as `_ "embed"`.
- When using build tagged features, make sure the software compiles without the tag as well.
</goGuidelines>

<debuggingGuidelines>
If me or you the LLM agent seem to go down too deep in a debugging/fixing rabbit hole in our conversations, remind me to take a breath and think about the bigger picture instead of hacking away. Say: "I think I'm stuck, let's TOUCH GRASS".  IMPORTANT: Don't try to fix errors by yourself more than twice in a row. Then STOP. Don't do anything else.
</debuggingGuidelines>

<generalGuidelines>
Don't add backwards compatibility layers or adapters unless explicitly asked. If you think there is a need for a backwards compatibility or adapting to an existing interface, STOP AND ASK ME IF THAT IS NECESSARY. Usually, I don't need backwards compatibility.

If it looks like your edits aren't applied, stop immediately and say "STOPPING BECAUSE EDITING ISN'T WORKING".
</generalGuidelines>

## Release boundary

A `v*` tag runs `.github/workflows/release.yaml`: split GoReleaser Pro builds on Linux (with the aarch64
cross-compiler; goja-cv needs cgo) and macOS. Builders may read only the build license through the Vault role
`release-goja-cv-builder`; the shared publisher workflow uses `release-goja-cv-publisher`, the caller's
`GITHUB_TOKEN`, and a short-lived GitHub App token for the Homebrew tap.

Before the first tag, add this repository's immutable ID and exact workflow ref to Terraform's
`release_publishers` allowlist and apply a reviewed plan. Never add repository Action secrets for GoReleaser,
Homebrew, Fury, signing keys, Vault tokens, or arbitrary Vault-path inputs.
