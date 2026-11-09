---
Title: Setting up goja-cv in Claude Code and Claude Desktop
Slug: setup-claude-code
Short: claude mcp add, a project .mcp.json, and the Claude Desktop mcpServers entry
Topics:
- goja-cv
- agents
- claude
Commands:
- mcp
IsTopLevel: false
IsTemplate: false
ShowPerDefault: true
SectionType: Tutorial
---

goja-cv speaks MCP over stdio, so Claude starts it as a subprocess. Put the `goja-cv` binary on your `PATH`
(or use an absolute path in the configs below).

## Claude Code, one command

```bash
claude mcp add cv -- goja-cv mcp --image scan.tif --root /path/to/images
```

Everything after `--` is the server command. `--image` binds `img` at start (optional: the agent can also call
the `open` tool or `img = cv.open(path)`); `--root` limits what `cv.open` may read (repeat it for several
directories). Add `-s project` to write the entry to the project's `.mcp.json` instead of your local config,
or `-s user` for every project. Check with `claude mcp list`.

## Claude Code, project .mcp.json

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

Relative paths are resolved against the directory Claude Code starts the server in (the project root). The
model sees the tools as `mcp__cv__eval`, `mcp__cv__open`, … and the resources `cv://reference`,
`cv://docs/<id>`, `cv://guide/<slug>`.

## Claude Desktop

Add the same entry to `claude_desktop_config.json` (macOS: `~/Library/Application Support/Claude/`), with
absolute paths because Desktop does not start servers in a project directory:

```json
{
  "mcpServers": {
    "cv": {
      "command": "/usr/local/bin/goja-cv",
      "args": ["mcp", "--root", "/Users/me/Pictures/scans"]
    }
  }
}
```

## A prompt that helps

```text
You have a cv image-analysis MCP server. Use its eval tool for every measurement and edit.
Read cv.docs.search(...) / cv.help(name) when unsure, keep nodes in variables, await only to read data,
and return looks to see results.
```

See `goja-cv help agents` for the eval contract and `goja-cv help troubleshooting` if the server does not start.
