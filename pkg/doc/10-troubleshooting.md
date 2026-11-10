---
Title: Troubleshooting goja-cv
Slug: troubleshooting
Short: Common errors and what they mean, sessions, timeouts, and the goja interrupt caveat
Topics:
- goja-cv
- troubleshooting
Commands:
- goja-cv
- mcp
IsTopLevel: false
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

## Reading an error

Every cv error is YAML with `code`, `message`, `where` (the line of the eval code), `path` (the graph path, `✗`
marks the failing call), `hint` and usually `fix` (literal code). Apply the fix with your own variable names.

| code | usually means |
|---|---|
| E_UNKNOWN_METHOD / E_NAMESPACE | wrong method name or kind (`img.gausian(2)` → `img.gaussian(2)`, `img.luminance` → `img.mask.luminance`); see `fix` |
| E_UNKNOWN_OPTION | option misspelt or named differently (`min` → `above` on mask.luminance); `valid` lists the options |
| E_ARG_TYPE / E_ARG_RANGE | wrong type or outside hard limits; `cv.help(op)` lists units and ranges |
| E_WRONG_KIND | a colour image where a mask (one channel) is expected (fix `img.toGray()`), regions where a mask is expected (`img.mask.regions(regions)`), or plain data where a node was expected |
| E_NOT_AWAITED | a field of, or a number from, an unawaited measurement or region set: `(await img.stats()).p99`; regions have no methods: `img.heal(regions)`, `(await regions).filter(…)` |
| E_ARG_TYPE "item 3 was moved" | the box of an awaited region item was edited: pass a plain box `{ x, y, w, h }` for new geometry |
| ITEMS warning | objects that look like region items lost their link (a `map` that built new objects): spread them, `{ ...r, note }` |
| TypeError with a hint | a JavaScript error on cv data, translated (`r.count()` after await → `r.length`) |
| E_NO_IMAGE | nothing open: `img = cv.open("scan.tif")` or start with `--image` |
| E_ACCESS | the path is outside the workspace (`--root`) and the `--read` directories, a save outside the workspace or through a symlink, or a save over a file the session opened |
| E_DUPLICATE | one eval saves the same path twice: save another name, or save once |
| E_EVAL "changed since it was opened" | the file changed before its pixels were read: open it again |
| LOOKS warning | more than four looks in one eval: the others are named by line and not delivered |
| E_TIMEOUT | the eval ran longer than the limit (60 s; `--timeout-ms`) |

`cv.help(name)` never throws and suggests names; `cv.docs.search(text)` finds the guide section.

## The server does not start or the tools do not appear

- Run the exact command by hand: `goja-cv mcp --root /path/work --read /path/scans --image /path/scans/scan.tif`
  should wait silently on stdin (stop it with Ctrl-D). A usage error exits 2 with a message on stderr.
- `mcp` needs `--root`; `--root` and each `--read` must be directories; `--image` must exist and lie in one of them.
- Clients often start servers from another working directory: use absolute paths.
- `--log-level debug` prints logs to stderr; stdout must stay clean for the protocol.

## Task libraries and preludes

- `--prelude … is outside --root … and the --read directories; add --read DIR`: a prelude is a `cv.load` at
  session start and must lie in the workspace or a read directory.
- `prelude dust/helpers.js:57: …`: the prelude failed at that line; the session is not created. Run the file with
  `goja-cv eval --read DIR -e 'return await cv.load("dust/helpers.js")'` to see the full error.
- `E_TOO_MANY` from `cv.load` ("nested 32 deep"): a file loads itself, directly or through another file.
- `UNHANDLED line N: a promise was rejected and never handled`: an unawaited `cv.load` or async helper failed;
  await it. The eval itself waits for every load and save it started.

## Sessions

- One server process has one default session; `mem`, `img` and look numbers live there. Restarting the
  server (or a client that restarts it) loses them — in pi use `"lifecycle": "keep-alive"`.
- Agents should omit `sessionId`. An invented id gets an error that lists the existing sessions.
- `create_session` makes a fresh session without `--image`; open an image there with `img = cv.open(path)`.

## Timeouts and the interrupt caveat

Kernels are cancelled when an eval times out. JavaScript that loops forever is interrupted too, but goja
cannot always recover: an interrupt inside a resumed async function (a busy loop **after** an `await`) leaves
that runtime unable to run promises. goja-cv detects this, reports it in the timeout error, and fails later evals
in that session at once with "Start a new session". Use `create_session` or restart the server; `mem` is lost.

## Results are cut or images missing

- Results over 9000 characters end with `# CUT …`: return less (pick fields, slice arrays), and save complete data
  with `await cv.save("x.json", x)`.
- More than four looks in one eval: the extra ones are not delivered; they are marked `[not delivered: …]` and the
  `LOOKS` warning names them by line. Split the work over several calls, or return fewer looks (one `cv.sheet`).
- A look of a Value or Regions node is `E_WRONG_KIND`: use `img.overlay(regions).look()` or a histogram look.
