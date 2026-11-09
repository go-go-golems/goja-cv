---
Title: Using goja-cv on the command line
Slug: cli
Short: run, eval, ops, help and mcp, with their flags, look and data files, saving, preludes, JSON output and exit codes
Topics:
- goja-cv
- cli
Commands:
- goja-cv
- run
- eval
- ops
- help
- mcp
Flags:
- image
- out
- out-root
- prelude
- json
- timeout-ms
- root
- http
- allow-remote
- log-level
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: Tutorial
---

The CLI runs cv code once (`eval`, `run`), lists the ops (`ops`), explains things (`help`), and serves the eval
session to agents (`mcp`). It never prompts; output is YAML by default, JSON with `--json`.

## eval and run

`eval -e CODE` and `run SCRIPT.js` evaluate one async function body with `img` bound to `--image`. The result is
printed as YAML; looks are written as image files (`look-<n>-<slug>.png|jpg`) and data items returned with
`cv.csv(x, { name })` / `cv.json(x, { name })` as `<name>.csv` / `<name>.json` into `--out` (a new temporary
directory if omitted); their paths are printed under `files:`.

```bash
goja-cv eval --image scan.tif --out looks -e 'const s = await img.stats(); return { meanL: s.meanL, p99: s.p99, after: img.levels({ black: s.p01, white: s.p99 }).look("levels") }'
```

```yaml
{meanL: 0.29507, p99: 0.83255, after: '[look 1: levels (64×48, 1 image px = 1.0 doc px)]'}

context:
  images: img = scan.tif (64×48, 16-bit TIFF, srgb)

files:
    'look 1: levels (64×48, 1 image px = 1.0 doc px)': /abs/path/looks/look-1-levels.png
```

A script file is the same body; `return` what you want printed:

```bash
goja-cv run fix-exposure.js --image scan.tif --out looks
goja-cv run fix-exposure.js --image scan.tif --json
```

`--json` prints `{ status, result, error, console, files: [{ caption, mimeType, bytes, rows, path, dropped }] }`
with the YAML text in `result`; `rows` counts the records of a CSV or the items of a JSON array. A program reads the
file whose `mimeType` is `text/csv`:

```bash
goja-cv eval --json --out /tmp/specks --image TILE.tif \
  -e 'return cv.csv(img.detect.specks({ polarity: "bright", maxArea: 2000 }), { name: "specks" })'
```

`--out-root DIR` is the only directory `node.save(path)` and `data.save(path)` may write under (paths are relative
to it; without it saving is disabled). `--prelude FILE` (repeatable) runs a script in the session before the code:
helpers it puts on `mem` are there for the eval (`--prelude 84-cv-helpers.js -e 'return await mem.hot(2)'`); a
failing prelude exits 1 with `prelude FILE:LINE: …`. `--timeout-ms` raises the 60 s eval limit.

## Exit codes

| code | meaning |
|---|---|
| 0 | ok |
| 1 | a cv error (the structured error is still printed, and looks shown before it are still written) |
| 2 | a usage error: unknown command or flag, missing script, missing `--image` file, bad `--root` |

```bash
goja-cv eval --image scan.tif -e 'return img.brightness(1)'
```

```yaml
error:
  code: E_UNKNOWN_METHOD
  message: Image has no method "brightness".
  where: 'line 1: return img.brightness(1)'
  path: open(scan.tif) → ✗ brightness
  hint: Did you mean "exposure"?
  fix: img.exposure(…)
```

## ops and help

```bash
goja-cv ops
goja-cv ops --kind Regions
goja-cv ops --json
goja-cv help levels
goja-cv help Mask
goja-cv help Regions.filter
goja-cv help levels --json
goja-cv help concepts
goja-cv help eval
```

`goja-cv help NAME` resolves NAME as a guide page (this page is `cli`), then as a command, then as a cv op, kind
(`Mask`), `Kind.method` or cv function, exactly like `cv.help(NAME)` inside eval. An unknown name prints
`{ found: false, didYouMean }` and exits 1. `goja-cv help --list` lists the guide pages.

## mcp

`goja-cv mcp` serves the eval session over MCP on stdin/stdout (for Claude Code, Codex, pi and other clients):

```bash
goja-cv mcp --image scan.tif --root /path/to/images
goja-cv mcp --root ./scans --root ./refs --timeout-ms 120000
goja-cv mcp --http 127.0.0.1:3092 --image scan.tif
```

- `--image FILE` opens the image in the default session and binds it to `img`.
- `--root DIR` (repeatable) limits the files `cv.open` may read; without it any readable path can be opened.
- `--out-root DIR` is the only directory `save` may write under (separate from the read roots; default: saving is
  disabled). Data items over 256 KiB are not sent to the client; the agent is told to save them.
- `--prelude FILE` (repeatable) runs in every new session, the default one and those made with `create_session`.
- `--http ADDR` serves streamable HTTP instead of stdio; non-loopback addresses need `--allow-remote` (there is
  no authentication).
- `--log-level debug` logs to stderr (stdout carries the protocol).

See `goja-cv help agents` for what the agent gets, and the setup pages for each client.
