---
Title: Setting up goja-cv in Codex
Slug: setup-codex
Short: codex mcp add and the [mcp_servers.cv] table in ~/.codex/config.toml
Topics:
- goja-cv
- agents
- codex
Commands:
- mcp
IsTopLevel: false
IsTemplate: false
ShowPerDefault: true
SectionType: Tutorial
---

Codex starts stdio MCP servers from `~/.codex/config.toml`. You can add goja-cv with the CLI or by editing the
file.

## One command

```bash
codex mcp add cv -- goja-cv mcp --root /path/to/cv-work --read /path/to/images --image /path/to/images/scan.tif
```

The syntax is `codex mcp add [OPTIONS] <NAME> -- <COMMAND>...`; `--env KEY=VALUE` sets environment variables
for the server. `codex mcp list` and `codex mcp get cv` show what is configured; `codex mcp remove cv` removes it.

## config.toml

```toml
[mcp_servers.cv]
command = "goja-cv"
args = ["mcp", "--root", "/path/to/cv-work", "--read", "/path/to/images", "--image", "/path/to/images/scan.tif"]
startup_timeout_sec = 20

[mcp_servers.cv.env]
# optional environment for the server process
```

Use absolute paths: Codex may start the server from another directory. One-off overrides work with
`-c`, for example `codex -c 'mcp_servers.cv.args=["mcp","--root","/tmp/cv-work","--read","/tmp/scans"]'`.
`--root` is required: it is the one directory the agent writes; `--read` directories are read-only.

## Notes

- Codex lists the server's tools to the model; the eval tool's description and the server instructions carry
  the eval contract, and `cv.docs` is reachable from inside eval.
- Long analyses: raise goja-cv's own limit with `--timeout-ms` in `args` (the default per eval is 60 s).
