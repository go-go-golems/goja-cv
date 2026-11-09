---
Title: Embedding goja-cv in Go and in xgoja binaries
Slug: embedding
Short: The cvsession API (sessions, evaluation, MCP server) and the xgoja provider for the cv module
Topics:
- goja-cv
- go
- xgoja
Commands:
- goja-cv
IsTopLevel: false
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## The eval session: pkg/cvsession

`cvsession` is a go-go-goja `replapi` app whose runtimes load only the cv module, with the cv result
processor (YAML, warnings, context block, looks as image content).

```go
app, err := cvsession.New(ctx, cvsession.Options{
	Image:       "scan.tif",              // bound to img in every new session
	Roots:       []string{"/data/scans"}, // cv.open may only read here
	EvalTimeout: 60 * time.Second,
})
if err != nil { return err }
defer app.Close(context.Background())

s, _ := app.CreateSession(ctx)
resp, err := app.Evaluate(ctx, s.ID, `return (await img.stats()).meanL`)
// resp.Cell.Execution.Result is the YAML text, resp.Content the rendered looks
```

`App.Evaluate` takes the agent's code (an async function body) and serializes evaluations per session.
`cvsession.NewMCPServer(ctx, app)` builds the MCP server `goja-cv mcp` serves (`.Server` is a go-sdk
`*mcp.Server`; run it with `replmcp.ServeStdio` or `replmcp.NewStreamableHTTPHandler`).

Lower-level pieces, if you assemble your own runtime: `cvsession.NewRuntimeFactory(opts)`,
`cvsession.NewInitializer(opts)`, `cvsession.NewResultProcessor(opts)`, `cvsession.SessionPolicy(timeout)`.
The cv module itself is `pkg/gojamod/cv` (`cvmod.Registrar`, `cvmod.New()`); the engine is `pkg/cv/graph`.

## xgoja: the cv module in generated binaries

`pkg/xgoja/providers/cv` registers provider package **`goja-cv`** with module `cv` (module config: `baseDir`,
`cacheBytes`, `previewLong`, `lookLong`). Combined with go-go-goja's `mcp` command set:

```yaml
schema: xgoja/v2
name: cv-agent
go: { module: xgoja.generated/cv-agent, version: "1.26" }
providers:
  - { id: goja-cv, import: github.com/go-go-golems/goja-cv/pkg/xgoja/providers/cv, register: Register }
  - { id: go-go-goja-mcp, import: github.com/go-go-golems/go-go-goja/pkg/xgoja/providers/mcp, register: Register }
runtime:
  modules:
    - { provider: goja-cv, name: cv, as: cv }
commands:
  - { id: run, type: builtin.run, name: run }
  - { id: mcp, type: provider.command-set, provider: go-go-goja-mcp, name: mcp, mount: mcp }
artifacts:
  - { id: binary, type: binary, output: dist/cv-agent }
```

**Provider ids must equal the provider package IDs** (`goja-cv`, `go-go-goja-mcp`): runtime modules and command
sets are resolved by package ID, and `xgoja doctor` does not catch a mismatch (the binary fails with
`unknown provider module`). A complete example is `examples/xgoja/cv-agent/xgoja.yaml`.

The generic `mcp` command set gives `require("cv")` and image content, but not the cv session (no YAML
results, lints, `cv.docs` in scope, `img`/`mem` globals). For agents, `goja-cv mcp` is the full experience.
