# goja-cv

**A lazy, measurable image API in JavaScript, built for agents that have an eval tool.**

goja-cv is an image-analysis and editing engine written in Go and exposed to JavaScript as `require("cv")`
through [go-go-goja](https://github.com/go-go-golems/go-go-goja). An agent (or you) sends a few lines of
JavaScript; goja-cv builds a lazy graph of operations, evaluates it in Go, and answers with YAML and images
("looks"). It measures (statistics, histograms, dust detection), selects (masks, regions), edits (exposure,
levels, masked edits, healing, inpainting) and shows (numbered looks, overlays, comparisons). The pixels never
enter JavaScript, and the runtime has no filesystem, process or network access.

```js
const s = await img.stats()
const dust = img.detect.specks().where({ score: { gte: 0.5 } })
return {
  meanL: s.meanL, p99: s.p99,
  dust: await dust.count(),
  look: img.overlay(dust, { labels: "id" }).look("dust"),
}
```

On goja-cv's synthetic test scan the answer is (the look arrives as an image next to the text):

```yaml
{meanL: 0.22726, p99: 0.45755, dust: 40, look: '[look 1: dust (256×192, 1 image px = 1.0 doc px)]'}

context:
  images: img = planted-256x192-16bit.tif (256×192, 16-bit TIFF)
```

The full guidebook ships inside the binary: `goja-cv help` (pages listed [below](#documentation)).

## Install

Tagged releases publish `goja-cv` (Linux and macOS, amd64 and arm64) to the go-go-golems Homebrew tap and as
`.deb`/`.rpm` packages:

```bash
brew tap go-go-golems/go-go-go
brew install --cask goja-cv
```

From source (needs cgo, i.e. a C compiler: go-go-goja's session links tree-sitter and SQLite):

```bash
go install github.com/go-go-golems/goja-cv/cmd/goja-cv@latest
```

or from a checkout:

```bash
git clone https://github.com/go-go-golems/goja-cv && cd goja-cv
go build -o goja-cv ./cmd/goja-cv
```

Go 1.26 or newer. Inputs: PNG, JPEG, TIFF (8/16-bit integer, 32-bit float; LZW, Deflate, PackBits), LinearRaw DNG and NumPy `.npy`; each file has an encoding (`srgb` or `linear`) that `cv.open(path, { encoding })` can override.

## Quick start on the CLI

Code is the **body of an async function**: `await` what you need and `return` what you want to see. `img` is
the image given with `--image`; looks are written as files into `--out` and their paths printed.

```bash
goja-cv eval --image scan.tif --out looks -e 'const s = await img.stats(); return { meanL: s.meanL, p99: s.p99, fixed: img.levels({ black: s.p01, white: s.p99 }).look("levels") }'
goja-cv run fix-exposure.js --image scan.tif --out looks
goja-cv run fix-exposure.js --image scan.tif --json
goja-cv ops --kind Regions
goja-cv help levels
goja-cv help Mask
goja-cv help concepts
```

- Output is YAML (`--json` for a JSON envelope). Exit codes: **0** ok, **1** cv error (the structured error is
  still printed, with a literal `fix:`), **2** usage error.
- `goja-cv help NAME` shows a guide page, a command's help, or a cv op / kind / function (`levels`, `Mask`,
  `Regions.filter`), in that order.
- `--timeout-ms` raises the 60 s limit per eval; `--log-level debug` logs to stderr.

## Use with coding agents

`goja-cv mcp` serves the eval session as an MCP server over stdio. The agent gets an `eval` tool (plus `open`
and session tools), the resources `cv://reference`, `cv://docs/<id>` and `cv://guide/<slug>`, and the prompt
`cv-agent`. Two flags matter:

- `--image FILE` opens an image at start and binds it to `img` (optional; the agent can call the `open` tool or
  `img = cv.open(path)`).
- `--root DIR` (repeatable) limits which files `cv.open` may read. Use it.

### Claude Code

```bash
claude mcp add cv -- goja-cv mcp --image scan.tif --root /path/to/images
claude mcp add -s project cv -- goja-cv mcp --root ./images
```

The second form writes a project `.mcp.json` you can commit:

```json
{
  "mcpServers": {
    "cv": {
      "command": "goja-cv",
      "args": ["mcp", "--image", "scan.tif", "--root", "/path/to/images"]
    }
  }
}
```

**Claude Desktop** uses the same `mcpServers` entry in `claude_desktop_config.json` (macOS:
`~/Library/Application Support/Claude/`); use absolute paths there. Details: `goja-cv help setup-claude-code`.

### Codex

```bash
codex mcp add cv -- goja-cv mcp --image /path/to/scan.tif --root /path/to/images
```

or in `~/.codex/config.toml`:

```toml
[mcp_servers.cv]
command = "goja-cv"
args = ["mcp", "--image", "/path/to/scan.tif", "--root", "/path/to/images"]
startup_timeout_sec = 20
```

`codex mcp list` shows what is configured. Use absolute paths. Details: `goja-cv help setup-codex`.

### pi coding agent

pi needs the MCP adapter extension:

```bash
pi install npm:pi-mcp-adapter
```

Project `.mcp.json`:

```json
{
  "mcpServers": {
    "cv": {
      "command": "goja-cv",
      "args": ["mcp", "--image", "scan.tif", "--root", "/path/to/images"],
      "lifecycle": "keep-alive",
      "directTools": ["eval"]
    }
  }
}
```

- `"lifecycle": "keep-alive"` keeps one server process for the conversation, so `mem`, `img`, params and look
  numbers persist between calls.
- `"directTools": ["eval"]` exposes the eval tool directly (the model sees `cv_eval`) instead of behind the
  adapter's search proxy — important for small models.

Headless (scripts, CI, evaluations):

```bash
pi -p -a --mcp-project-servers allow --no-builtin-tools --model provider/model \
   "Is scan.tif exposed well? How many dust specks are there?" < /dev/null
```

`-a` trusts the project's files for this run and `--mcp-project-servers allow` starts its MCP servers without
asking; `--no-builtin-tools` makes every answer go through cv. **Close stdin** (`< /dev/null`) in scripts — in
print mode pi otherwise waits for input — and `unset CDPATH` in scripts that `cd` into relative paths. Details:
`goja-cv help setup-pi`.

### Tips for prompting agents

These come from test runs with a small model (pi + deepseek-4.1-flash, five questions about a scan; errors went
from six per task to two, every one corrected on the next call):

- Tell the model: "Use the cv eval tool for every measurement; use `cv.docs.search(…)` and `cv.help(name)` when
  unsure; apply the `fix:` of an error."
- **Keep nodes, await only for data**: `const specks = img.detect.specks()` is a node; `await specks` is data
  (`{ count, items }`). Awaited data still accepts node methods, but `r.count` is a number.
- **Use `mem`** for anything needed in a later call; `const`/`let` do not survive.
- **Return looks** to see results (`node.look("label")`, `show(node)`, `cv.compare([a, b])`), and measure inside
  regions with `img.stats({ box: [x, y, w, h] })` or `img.stats({ mask })`.

## Embedding

- **Go:** `pkg/cvsession` — `cvsession.New(ctx, Options{Image, Roots, EvalTimeout})`, `App.Evaluate(ctx, id, code)`,
  `cvsession.NewMCPServer(ctx, app)`. See `goja-cv help embedding`.
- **xgoja:** `pkg/xgoja/providers/cv` (provider package `goja-cv`, module `cv`) and
  `examples/xgoja/cv-agent/xgoja.yaml`. Provider ids must equal package IDs.

## Documentation

In the binary (`goja-cv help <slug>`; also MCP resources `cv://guide/<slug>`):

| slug | page |
|---|---|
| `what-is-goja-cv` | what it is, when to use it, what is implemented |
| `concepts` | kinds, lazy nodes, doc px and scales, masks vs regions, lambdas, looks, params, errors |
| `cli` | every command and flag, look files, JSON output, exit codes |
| `agents` | what an agent sees over MCP, the eval contract, `cv.docs`, limits, security |
| `setup-claude-code`, `setup-codex`, `setup-pi` | client setup |
| `recipes` | exposure, dust, measuring inside a box, masked edits, comparisons, params |
| `embedding` | Go API and xgoja |
| `troubleshooting` | errors, sessions, timeouts, the goja interrupt caveat |

For agents, from inside eval: `cv.docs.list()`, `cv.docs.search(text)`, `cv.docs.read(id, { section })` (nine
topics, every example tested), and `cv.help(name)`.

## Layout and development

- `pkg/cv/{buffer,parallel,kernels,graph,ops,look,docs}` — engine, ops, looks, agent guides
- `pkg/gojamod/cv` — the go-go-goja module `cv` and its `prelude.js`
- `pkg/cvsession` — eval session, result processor, MCP server
- `pkg/doc` — the guidebook (Glazed help pages)
- `cmd/goja-cv` — the CLI
- `pkg/xgoja/providers/cv` — xgoja provider

```bash
make test       # go test ./... -count=1 (make test-race adds -race, as CI does)
make lint       # pinned golangci-lint (.golangci-lint-version, installed to .bin/) and glazed-lint
make smoke      # build dist/goja-cv and exercise help, ops and eval on a generated image
make logcopter-generate   # regenerate the per-package loggers; make logcopter-check verifies them
make vet
make testdata   # regenerate the synthetic test images
go run ./internal/testimage/gen -out /tmp/cv -planted   # the planted-fault dust scan used by tests and docs
lefthook install          # pre-commit: lint + test; pre-push: also a snapshot goreleaser build
make goreleaser           # local snapshot release for this platform (dist/)
```

CI (`.github/workflows`) runs unit tests with `-race`, the smoke test, golangci-lint, govulncheck, gosec, CodeQL
and secret scanning. Releases are cut by pushing a `v*` tag; `release.yaml` builds Linux and macOS natively and
publishes through the shared go-go-golems publisher with Vault/OIDC credentials (no repository secrets).

The tests run every code example of the agent topics and the guidebook in a real session, and parse every
`goja-cv` command shown in the guide and this README. go-go-goja is pinned to a commit of
go-go-golems/go-go-goja#108; to work against a local checkout add
`replace github.com/go-go-golems/go-go-goja => ../go-go-goja` (do not commit it).
