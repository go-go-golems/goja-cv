# Getting started

How an eval runs: your code is the body of an async function, you `return` what you want to see, `img` is the open image (more can be open under names), `mem` keeps values between evals, and looks bring images back.

## The eval contract

- The code is the **body of an async function**. Use `await` anywhere; end with `return`. The last expression is not returned.
- The returned value comes back as **YAML**, capped at 9000 characters. Return small things: pick fields, slice arrays.
- After the value come `warnings:` (RANGE, UNUSED_NODE, with your line number) and a `context:` block (the images by name with file, frame and encoding, `mem` keys).
- Options are **literal values**. A measurement that feeds an option is awaited first: `img.levels({ white: (await img.stats()).p99 })`.
- `const` and `let` do **not** survive to the next eval. Keep anything you need later on `mem`.
- One eval may run 60 s. Heavy work runs in Go; JavaScript only builds graphs and handles small data.
- A task library or a saved pipeline is a file: `await cv.load("dir/file.js", args)` runs it in the session, `cv.scripts()` lists them (`cv.docs.read("task-libraries")`). Never paste a file into an eval.

```js
const s = await img.stats()
return { meanL: s.meanL, p01: s.p01, p99: s.p99 }
```

## The globals

| name | what it is |
|---|---|
| `img` | the open image (an Image node); `img = cv.open("scan.tif")` rebinds it |
| names | more images: `cv.open(path, { name: "ref" })` binds `ref` as a global that persists like `img`; the context block lists every name |
| `cv` | the module: `cv.open(path, { encoding, box, origin, name })`, `cv.load(path, args)`, `cv.help(op)`, `cv.docs`, `cv.compare([...])`, `cv.explain(node)` |
| `mem` | a plain object that persists between evals |

```js
const { meanL, p99 } = await img.stats()
mem.base = { meanL, p99 }
const info = await img      // file, format, bitDepth, encoding, frame [x, y, w, h], …
return { base: mem.base, file: info.file, bitDepth: info.bitDepth, encoding: info.encoding, frame: info.frame }
```

## Seeing images

A node is lazy; to see it, return a **look** in the result (alone, or inside an object or array):

```js
const brighter = img.exposure(0.5)
return { before: img.look("before"), after: brighter.look("after") }
```

Each look is numbered (the number is burned into its top-left corner) and its caption gives the scale, e.g. `look 2: after (256×192, 1 image px = 1.0 doc px)`. At most four images arrive per eval. See `cv.docs.read("looks")`.

## Finding things

- `cv.docs.list()` lists the topics; `cv.docs.search("dust heal")` finds sections; `cv.docs.read("recipes", { section: "Remove dust" })` reads one.
- `cv.help()` lists the ops per kind; `cv.help("levels")` describes one op (arguments, units, defaults, ranges); `cv.help("Image")` lists a kind's methods, `cv.help("mask")` and `cv.help("regions")` explain a concept; `cv.help("Image.overlay")` one method of a kind. It never throws: unknown names return `{ found: false, didYouMean }`.
- Errors carry a literal `fix:`; apply it (see `cv.docs.read("errors")`).
- These topics are written for you, the agent. The longer guidebook for the people who set goja-cv up (CLI, MCP clients, embedding) is `goja-cv help` on the command line and the MCP resources `cv://guide/<slug>`.

```js
return { topics: cv.docs.list().map(t => t.id), hits: cv.docs.search("heal specks", { limit: 3 }) }
```
