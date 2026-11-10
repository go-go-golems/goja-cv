# Task libraries

A task library is a JavaScript file of helpers for one task; a pipeline script does one job and returns its result. Run either with `await cv.load(path, args)` instead of pasting it into an eval.

## Loading a file

`await cv.load("dust/helpers.js", args)` reads the file like `cv.open` reads images (the workspace, then each `--read` directory) and runs it as the body of `async function (args) { … }`: top-level `await` and `return` work, `const` and `let` stay in the file, `img`, `cv` and `mem` are the session's. It resolves to what the file returns. `cv.scripts()` lists the `.js` files you can load, with their first comment line.

```js
const ready = await cv.load("dust/helpers.js")
const spots = await mem.spots({ snr: 2 })   // peaks measured on the speck view: spStep
return { ready, top: spots.slice(0, 3).map(({ id, cx, cy, snr, spStep }) => ({ id, cx, cy, snr, spStep })), scripts: cv.scripts().map(s => s.path) }
```

Every call reads and runs the file again (no cache): a second load picks up an edited file and redefines its functions; a pipeline runs again with new `args`. Nodes are hash-consed, so a rerun costs little. A file may load others by the same path rule; loads nest up to 32 deep.


## Writing a library

- Start the file with a comment line: `cv.scripts()` shows it.
- Keep what must survive on `mem` (`mem.spots = async (opts) => …`); everything else is private to the file.
- Open each function's body with a comment: it is the function's doc line in the context block. `mem.spots = async (opts) => { // peaks with the speck view's step` — a comment above the function or in the middle of its body is not.
- Return a short string or an object; return rows, not joined strings.

```js fragment
// dust/helpers.js: helpers for annotator agents
mem.lin = img.as("linear")
mem.spots = async ({ snr = 1, radius = [2, 4, 8] } = {}) => { // peaks with the speck view's step: sp = spStep
  const pk = await mem.lin.peaks({ radius, snr })
  return await mem.lin.view("speck").measure(pk, { prefix: "sp" })
}
return "dust helpers ready"
```

## A pipeline: the width of a line or thread

`dust/thread-width.js` measures the full width at half height of a bright line across the segment `a → b` (both
ends on either side of it) from a `profile`. Run it at two or three places along the thread:

```js
const w = await cv.load("dust/thread-width.js", { a: [192, 46.5], b: [209, 46.5] })
return { width: w.width > 3 && w.width < 8, at: w.at }
```

```js fragment
// dust/thread-width.js: the width of a bright thread across the segment a → b (doc px), full width at half height
const p = await img.as("linear").profile(args.a, args.b, { step: 0.25, width: args.along ?? 3 })
const v = p.rows.map(r => r.meanL), t = p.rows.map(r => r.t), n = v.length
const base = (v[0] + v[n - 1]) / 2
let k = 0
for (let i = 1; i < n; i++) if (v[i] > v[k]) k = i
const half = base + (v[k] - base) / 2
const cross = d => { // where the profile falls below half, walking from the peak
  let i = k
  while (i + d >= 0 && i + d < n && v[i + d] > half) i += d
  const j = i + d
  return j < 0 || j >= n ? null : t[i] + (t[j] - t[i]) * (v[i] - half) / (v[i] - v[j])
}
const lo = cross(-1), hi = cross(1)
return { width: lo === null || hi === null ? null : hi - lo, height: v[k] - base, at: [p.rows[k].cx, p.rows[k].cy] }
```

## Errors, warnings and waiting

- An error in a loaded file names the file and line, and the eval line that called into it: `where: 'dust/helpers.js:6: return nope.y'`, `called from: 'line 2: return mem.bad()'`. A file that does not compile is a `SyntaxError` whose message starts with `dust/helpers.js:12:5:`.
- Warnings raised by calls in the file carry `dust/helpers.js:57`.
- Looks a file creates are delivered like the eval's own.
- The eval does not end before the loads (and saves) it started have settled, awaited or not, so nothing a file does leaks into a later eval. A load nobody awaited that failed is an `UNHANDLED` warning with the file's line; await what you load.

## Preludes

`goja-cv mcp --read DIR --prelude DIR/dust/helpers.js` loads the file in every new session before any agent code (also `eval` and `run`). The file must lie in `--root` or a `--read` directory. Looks a prelude creates are discarded, and a failing prelude fails the session as `prelude dust/helpers.js:57: message`.
