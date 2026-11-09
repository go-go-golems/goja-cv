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
| E_UNKNOWN_METHOD / E_NAMESPACE | wrong method name or kind (`specks.heal()` → `img.heal(specks)`); see `fix` |
| E_UNKNOWN_OPTION | option misspelt or named differently (`min` → `above`, `radius` → `sigma` on a blur) |
| E_ARG_TYPE / E_ARG_RANGE | wrong type or outside hard limits; `cv.help(op)` lists units and ranges |
| E_WRONG_KIND | a Mask where Regions were expected, or plain data where a node was expected |
| E_LAZY_COMPARE | a node used as a number: `(await node) > x` |
| E_NOT_ARRAY | awaited Regions used as an array: they are `{ count, items }`; use `.items` |
| TypeError with a hint | a JavaScript error on cv data, translated (`r.count()` after await → `r.count`) |
| E_NO_IMAGE | nothing open: `img = cv.open("scan.tif")` or start with `--image` |
| E_ACCESS | the path is outside `--root` |
| E_NO_DOCUMENT | `doc` (history/apply/export) is not implemented yet |
| E_TIMEOUT | the eval ran longer than the limit (60 s; `--timeout-ms`) |

`cv.help(name)` never throws and suggests names; `cv.docs.search(text)` finds the guide section.

## The server does not start or the tools do not appear

- Run the exact command by hand: `goja-cv mcp --image scan.tif --root /path` should wait silently on stdin
  (stop it with Ctrl-D). A usage error exits 2 with a message on stderr.
- `--image` must exist and be PNG/JPEG/TIFF; each `--root` must be a directory.
- Clients often start servers from another working directory: use absolute paths.
- `--log-level debug` prints logs to stderr; stdout must stay clean for the protocol.

## Sessions

- One server process has one default session; `mem`, `img`, params and look numbers live there. Restarting the
  server (or a client that restarts it) loses them — in pi use `"lifecycle": "keep-alive"`.
- Agents should omit `sessionId`. An invented id gets an error that lists the existing sessions.
- `create_session` makes a fresh session without `--image`; open an image there with `img = cv.open(path)`.

## Timeouts and the interrupt caveat

Kernels are cancelled when an eval times out. JavaScript that loops forever is interrupted too, but goja
cannot always recover: an interrupt inside a resumed async function (a busy loop **after** an `await`) leaves
that runtime unable to run promises. goja-cv detects this, reports it in the timeout error, and fails later evals
in that session at once with "Start a new session". Use `create_session` or restart the server; `mem` is lost.

## Results are cut or images missing

- Results over 9000 characters end with `# CUT …`: return less (pick fields, slice arrays).
- More than four looks in one eval: the extra ones are not delivered; they are marked `[not delivered: …]` and the
  `LOOKS` warning names them by label. Split the work over several calls, or return fewer looks (one `cv.sheet`).
  Data items beyond four get the same treatment with a `DATA` warning.
- A look of a Value or Regions node is `E_WRONG_KIND`: use `img.overlay(regions).look()` or a histogram look.
