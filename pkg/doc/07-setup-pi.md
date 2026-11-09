---
Title: Setting up goja-cv in the pi coding agent
Slug: setup-pi
Short: pi-mcp-adapter, a project .mcp.json with keep-alive and directTools, and headless runs
Topics:
- goja-cv
- agents
- pi
Commands:
- mcp
IsTopLevel: false
IsTemplate: false
ShowPerDefault: true
SectionType: Tutorial
---

pi reaches MCP servers through the `pi-mcp-adapter` extension. This is the setup used for goja-cv's own agent
test runs (a small model answering questions about a scan through cv only).

## Install the adapter

```bash
pi install npm:pi-mcp-adapter
```

## Project .mcp.json

```json
{
  "mcpServers": {
    "cv": {
      "command": "goja-cv",
      "args": ["mcp", "--root", "/path/to/cv-work", "--read", "/path/to/images", "--image", "/path/to/images/scan.tif"],
      "lifecycle": "keep-alive",
      "directTools": ["eval"]
    }
  }
}
```

- `"lifecycle": "keep-alive"` keeps one server process for the whole conversation, so the cv session — `mem`,
  `img`, look numbers — persists between calls. Without it a server may be restarted and `mem` lost.
- `"directTools": ["eval"]` exposes goja-cv's eval as a first-class tool (`cv_eval`). Otherwise the model only
  sees the adapter's search/proxy tool and has to discover `eval` first, which costs calls and confuses small
  models.

pi asks before starting project-local servers; `-a` (`--approve`) trusts the project's files for one run and
`--mcp-project-servers allow` starts its MCP servers without asking.

## Headless runs

```bash
pi -p -a --mcp-project-servers allow --no-builtin-tools \
   --model provider/model \
   --append-system-prompt "You have a cv image-analysis MCP server. Use its eval tool for every measurement." \
   "How many dust specks are on the scan, and which bright feature is not dust?" < /dev/null
```

- `--no-builtin-tools` removes pi's file and shell tools, so every answer has to go through cv.
- **Close stdin** (`< /dev/null`) in scripts: in print mode with an open stdin pi waits for input forever.
- `--mode json` writes an event stream (one JSON object per line) you can grade or replay; `--session-dir`
  keeps the session files together with the run.
- In shell scripts that `cd` into relative paths, `unset CDPATH` first: with `CDPATH` set, `cd` echoes the
  directory and `$(cd … && pwd)` captures two lines.

## What helps a small model

From the test runs: tell it to use `cv.docs.search` and `cv.help`; it then reads the guides in the first few
calls and follows the recipes. Errors carry a literal `fix:`, and the models in the runs applied it on the next
call.
