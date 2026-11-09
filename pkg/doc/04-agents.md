---
Title: Using goja-cv with agents
Slug: agents
Short: What an agent sees over MCP — tools, resources, prompt, the eval contract, cv.docs, limits and security
Topics:
- goja-cv
- agents
- mcp
Commands:
- mcp
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

`goja-cv mcp` is an MCP server whose main tool is `eval`: the agent sends JavaScript, goja-cv runs it in a
persistent session and answers with YAML text plus image content. Everything else (docs, help, errors) is
designed to be reachable through that one tool, because many clients only show tools to the model.

## What the agent gets

| kind | name | purpose |
|---|---|---|
| tool | `eval` | run a JavaScript async-function body; `code`, optional `sessionId` (omit it) |
| tool | `open` | open an image path and bind it to `img`; returns its summary and a first look |
| tool | `create_session`, `list_sessions`, `history`, `bindings` | session management (go-go-goja replmcp) |
| resource | `cv://reference` | every op with arguments, units and defaults (generated from the registry) |
| resource | `cv://docs/<id>` | the agent guides (the same topics as `cv.docs`) |
| resource | `cv://guide/<slug>` | these guidebook pages |
| prompt | `cv-agent` | the instructions followed by the full reference |

The server **instructions** (sent at initialize, under 4 KB) state the eval contract and the most used calls,
and point to `cv.docs` and `cv.help`. They are deliberately short: some clients truncate or hide them.

## The eval contract

- The code is the **body of an async function**: use `await`, end with `return`. The last expression is not
  returned.
- The value comes back as **YAML**, capped at 9000 characters (`# CUT` note), followed by `warnings:` and a
  `context:` block (open images with size and bit depth, `mem` keys).
- `const`/`let` do not survive between calls; `mem` is a plain object that does.
- `img` is the open image (`img = cv.open(path)` rebinds it).
- `await` evaluates: measurements at full resolution; an awaited Image is its summary (nothing computed);
  awaited regions are a plain array of items, filtered and sorted with JavaScript and passed back to ops.
- Options are literal values: a measurement that feeds an option is awaited first.
- Images come back as **looks** (`node.look()`): every look created in an eval is delivered, at most four (a
  `LOOKS` warning names the others).
- Files: `await cv.save(path, x)` writes in the workspace (`--root`); `cv.open` reads it and the `--read`
  directories. Every save of an eval completes before it returns.
- Errors are YAML with a literal `fix:`; the agent should apply it.

## Docs from inside eval

Agents read documentation through the channel they are using. `cv.docs` is in the eval scope:

```js
return {
  topics: cv.docs.list().map(t => t.id),
  hits: cv.docs.search("heal dust", { limit: 2 }).map(h => [h.id, h.section]),
  help: Object.keys(cv.help("Image").methods).slice(0, 5),
}
```

`cv.docs.read(id, { section })` returns Markdown; `cv.help(name)` takes ops, kinds (`"Image"`), `Kind.method`,
cv functions and concepts (`"regions"`), and never throws. In test runs with a small model (deepseek-4.1-flash via pi), making the
docs reachable from eval and making every error carry a fix cut the calls per task from 30 to about 20, with
every remaining error corrected on the next call.

## Limits

| limit | value | change it with |
|---|---|---|
| one eval | 60 s | `--timeout-ms` |
| result text | 9000 characters YAML (20000 with console and notes) | — |
| images per eval | 4, 8 MiB total | — |
| sessions | in memory, one default session per server process | restart the server to reset |

## Security

- The eval runtime loads **only** the `cv` module and `timer`: no `fs`, `exec`, network or other modules.
- Files: the agent reads and writes only in the workspace (`--root DIR`, required for `mcp`) and reads the
  `--read DIR` directories; paths, `..` and symlinks that leave them fail with `E_ACCESS`. A save never follows a
  symlink and never replaces a file the session opened.
- `--http` without `--allow-remote` only listens on loopback; the HTTP transport has no authentication, and all
  HTTP clients share the default session.
- Nothing else writes files: `cv.save` writes in the workspace, the CLI its `--out` look files; the MCP server
  returns images inline.

## What a good agent prompt adds

- "Use cv.docs.search and cv.help(name) when unsure; apply the `fix:` of an error."
- "Keep nodes in variables; await only to read data; store what you need later on mem."
- "Return looks to see results; back judgements with img.measure(items) (regions, boxes, circles, or points with coreRadius)."
- "When a program needs every row, save it: await cv.save(\"rows.csv\", items); reopen saved images with cv.open(saved.path)."
