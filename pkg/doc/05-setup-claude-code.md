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
claude mcp add cv -- goja-cv mcp --root ~/cv-work --read /path/to/images --image /path/to/images/scan.tif
```

Everything after `--` is the server command. `--root` (required) is the workspace: the one directory the agent
writes (`cv.save`) and where `cv.open` reads relative paths first. `--read` (repeatable) adds a directory
`cv.open` may read but never write. `--image` binds `img` at start (optional: the agent can also call the `open`
tool or `img = cv.open(path)`); it must lie in one of them. Add `-s project` to write the entry to the project's `.mcp.json` instead of your local config,
or `-s user` for every project. Check with `claude mcp list`.

## A task library for every session

When agents do one task with a set of helpers (a JavaScript file that defines `mem` functions), serve its
directory with `--read` and load the file with `--prelude`, so the agent never pastes it:

```bash
claude mcp add cv -- goja-cv mcp --root ~/cv-work --read /path/to/images --read ~/lib --prelude ~/lib/dust/helpers.js
```

The prelude must lie in `--root` or a `--read` directory. Agents can also load files themselves with
`await cv.load("dust/helpers.js")`; see `goja-cv help agents`.

## Claude Code, project .mcp.json

```json
{
  "mcpServers": {
    "cv": {
      "command": "goja-cv",
      "args": ["mcp", "--root", "cv-work", "--read", "/path/to/images", "--image", "/path/to/images/scan.tif"]
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
      "args": ["mcp", "--root", "/Users/me/cv-work", "--read", "/Users/me/Pictures/scans"]
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
