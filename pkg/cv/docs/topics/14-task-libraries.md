# Task libraries

A task library is a JavaScript file of helpers for one task; a pipeline script does one job and returns its result. Run either with `await cv.load(path, args)` instead of pasting it into an eval.

## Loading a file

`await cv.load("dust/helpers.js", args)` reads the file like `cv.open` reads images (the workspace, then each `--read` directory) and runs it as the body of `async function (args) { … }`: top-level `await` and `return` work, `const` and `let` stay in the file, `img`, `cv` and `mem` are the session's. It resolves to what the file returns. `cv.scripts()` lists the `.js` files you can load, with their first comment line.

```js
const ready = await cv.load("dust/helpers.js")
const m = await mem.probe(200.5, 46.5)
return { ready, snr: m.snr > 2, scripts: cv.scripts().map(s => s.path) }
```

Every call reads and runs the file again (no cache): a second load picks up an edited file and redefines its functions; a pipeline runs again with new `args`. Nodes are hash-consed, so a rerun costs little. A file may load others by the same path rule; loads nest up to 32 deep.

```js
const w = await cv.load("dust/thread-width.js", { a: [192, 46.5], b: [209, 46.5] })
return { width: w.width > 3 && w.width < 8, at: w.at }
```

## Writing a library

- Start the file with a comment line: `cv.scripts()` shows it.
- Keep what must survive on `mem` (`mem.probe = async (x, y) => …`); everything else is private to the file.
- Open each function's body with a comment: it is the function's doc line in the context block. `mem.probe = async (x, y) => { // the best point within 2 px` — a comment above the function or in the middle of its body is not.
- Return a short string or an object; return rows, not joined strings.

```js fragment
// dust/helpers.js: helpers for annotator agents
mem.probe = async (x, y, r = 3) => { // core/ring measure at (x, y): { cx, cy, step, spread, snr }
  const [m] = await img.as("linear").measure([{ cx: x, cy: y, r }])
  return m
}
return "dust helpers ready"
```

## Errors, warnings and waiting

- An error in a loaded file names the file and line, and the eval line that called into it: `where: 'dust/helpers.js:6: return nope.y'`, `called from: 'line 2: return mem.bad()'`. A file that does not compile is a `SyntaxError` whose message starts with `dust/helpers.js:12:5:`.
- Warnings raised by calls in the file carry `dust/helpers.js:57`.
- Looks a file creates are delivered like the eval's own.
- The eval does not end before the loads (and saves) it started have settled, awaited or not, so nothing a file does leaks into a later eval. A load nobody awaited that failed is an `UNHANDLED` warning with the file's line; await what you load.

## Preludes

`goja-cv mcp --read DIR --prelude DIR/dust/helpers.js` loads the file in every new session before any agent code (also `eval` and `run`). The file must lie in `--root` or a `--read` directory. Looks a prelude creates are discarded, and a failing prelude fails the session as `prelude dust/helpers.js:57: message`.
