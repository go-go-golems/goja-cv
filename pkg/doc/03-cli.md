---
Title: Using goja-cv on the command line
Slug: cli
Short: run, eval, ops, help and mcp, with their flags, the workspace, look files, preludes, JSON output and exit codes
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
- root
- read
- prelude
- json
- timeout-ms
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
printed as YAML; looks are written as image files (`look-<n>-<slug>.png|jpg`) into `--out` (a new temporary
directory if omitted); their paths are printed under `files:`. Files the code writes with `cv.save` land in the
workspace (below).

```bash
goja-cv eval --image scan.tif --out looks -e 'const s = await img.stats(); return { meanL: s.meanL, p99: s.p99, after: img.levels({ black: s.p01, white: s.p99 }).look("levels") }'
```

```yaml
{meanL: 0.52726, p99: 0.92244, after: '[look 1: levels (64×48, 1 image px = 1.0 doc px)]'}

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

`--json` prints `{ status, result, error, console, files: [{ caption, mimeType, bytes, path, dropped }] }` with the
YAML text in `result` and the look files in `files`.

## The workspace: --root and --read

`--root DIR` is the workspace (default: the current directory): `cv.save(path, x)` writes there, and `cv.open` reads
relative paths there first. Each `--read DIR` (repeatable) is a directory `cv.open` may also read, by absolute path
or by a relative path the workspace does not have (in flag order); nothing is ever written to it. `--image` must lie
in the workspace or a read directory (a usage error otherwise). `..` and symlinks may not leave a directory, a save
never follows a symlink, and it never replaces a file the session opened.

```bash
goja-cv eval --json --root /tmp/specks --read ~/tiles --image ~/tiles/TILE.tif \
  -e 'return await cv.save("specks.csv", img.detect.specks({ polarity: "bright", maxArea: 2000 }))'
```

`--prelude FILE` (repeatable) runs a script in the session before the code:
helpers it puts on `mem` are there for the eval (`--prelude 84-cv-helpers.js -e 'return await mem.hot(2)'`); a
failing prelude exits 1 with `prelude FILE:LINE: …`. `--timeout-ms` raises the 60 s eval limit.

## Exit codes

| code | meaning |
|---|---|
| 0 | ok |
| 1 | a cv error (the structured error is still printed, and looks shown before it are still written) |
| 2 | a usage error: unknown command or flag, missing script, missing `--image` file or one outside the roots, a `--root` or `--read` that is not a directory, `mcp` without `--root` |

```bash
goja-cv eval --image scan.tif -e 'return img.exposre(1)'
```

```yaml
error:
  code: E_UNKNOWN_METHOD
  message: Image has no method "exposre".
  where: 'line 1: return img.exposre(1)'
  path: open(scan.tif) → ✗ exposre
  hint: Did you mean "exposure"?
  fix: img.exposure(…)
```

## ops and help

```bash
goja-cv ops
goja-cv ops --kind Image
goja-cv ops --json
goja-cv help levels
goja-cv help regions
goja-cv help Image.overlay
goja-cv help levels --json
goja-cv help concepts
goja-cv help eval
```

`goja-cv help NAME` resolves NAME as a guide page (this page is `cli`), then as a command, then as a cv op, kind
(`Image`), `Kind.method`, cv function or concept (`mask`, `regions`, `space`), exactly like `cv.help(NAME)` inside eval. An unknown name prints
`{ found: false, didYouMean }` and exits 1. `goja-cv help --list` lists the guide pages.

## mcp

`goja-cv mcp` serves the eval session over MCP on stdin/stdout (for Claude Code, Codex, pi and other clients):

```bash
goja-cv mcp --root ~/cv-work --read /path/to/images --image /path/to/images/scan.tif
goja-cv mcp --root ./work --read ./scans --read ./refs --timeout-ms 120000
goja-cv mcp --http 127.0.0.1:3092 --root ./work --image ./work/scan.tif
```

- `--root DIR` (required) is the workspace: the one directory the agent may write (`cv.save`), where `cv.open` reads
  relative paths first. An MCP server is started by its client in a directory of the client's choosing, so goja-cv
  does not default to it.
- `--read DIR` (repeatable) is a directory `cv.open` may also read, never write.
- `--image FILE` opens the image in the default session and binds it to `img`; it must lie in a root.
- `--prelude FILE` (repeatable) runs in every new session, the default one and those made with `create_session`.
- `--http ADDR` serves streamable HTTP instead of stdio; non-loopback addresses need `--allow-remote` (there is
  no authentication).
- `--log-level debug` logs to stderr (stdout carries the protocol).

See `goja-cv help agents` for what the agent gets, and the setup pages for each client.
