# Data out

The YAML result is for you to read and is capped; files for programs and people are written with `cv.save(path, x)` in the workspace, the format chosen by the extension, and reopen with `cv.open(path)`.

## Text for you, files for programs

The result of an eval comes back as YAML capped at about 9000 characters and 48 items per list. Return summaries there. A program that needs every row reads a file: `await cv.save("specks.csv", regions)` writes every item. Awaited Regions are an array of every item (up to 100000), so code may loop over them freely.

```js
const specks = img.detect.specks()
const t = await cv.save("tables/specks.csv", specks)
const p = await cv.save("probe.json", await img.measure([[200.5, 46.5]], { coreRadius: 3 }))
return { count: (await specks).length, rows: t.rows, file: p.path }
```

## cv.save

`await cv.save(path, x, { depth })` writes one file and resolves to `{ path, format, bytes, rows, depth, encoding, space, frame, replaced }`. The extension picks the format:

| extension | x | records |
|---|---|---|
| `.tif` (depth 16, 8, 32 float) | an Image node | values, encoding, frame origin |
| `.png` (16, 8) | an Image node | values, encoding, frame origin |
| `.npy` (32 float, 16) | an Image node | values only: reopens as linear at (0, 0) |
| `.jpg` (8) | an sRGB Image | a picture |
| `.json` | a Value or Regions node, or plain data | the data in schema order |
| `.csv` | a list of rows (objects): a Regions node, measurement rows, `p.rows` | one row per object, columns in first-seen order |

A node is evaluated at scale 1; plain data is written as you hold it, edits included (item links never reach a file). A mismatch is `E_ARG_TYPE` with the fix: an Image as `.json` (save `.tif`), data as an image (save `.json`), an object as CSV (save `.json`; a profile or histogram keeps its table in `rows`: `cv.save("p.csv", (await p).rows)`). Integer depths clamp values outside [0, 1] (`CLAMP`); signed images go to `{ depth: 32 }` TIFF or `.npy`.

```js warns=CLAMP
const v = await cv.save("views/tile-17-speck.tif", img.view("speck"))
const back = await cv.open(v.path)
const p = await img.profile([190.5, 46.5], [210.5, 46.5])
const t = await cv.save("tables/profile.csv", p.rows)
return { saved: [v.encoding, v.frame], reopened: [back.encoding, back.frame], rows: t.rows }
```

## The workspace

Relative paths are names in the workspace (`--root DIR`; the current directory for `goja-cv eval` and `run`); `cv.open` also reads the read directories (`--read DIR`), which are never written. `..` and symlinks may not leave a directory. One save per path per eval (`E_DUPLICATE`); a save never writes through a symlink and never replaces a file the session opened (`E_ACCESS`); another existing file is replaced (`replaced: true`). The file is written under a temporary name and renamed into place, so a failed save leaves the old one. Every save an eval starts completes before the eval returns, awaited or not; an unawaited save that fails fails the eval.

```js error=E_DUPLICATE
await cv.save("a.json", { n: 1 })
return await cv.save("a.json", { n: 2 })
```

## From the command line

`goja-cv eval --json --out DIR` writes the looks as `look-<n>-<slug>.png` into DIR and lists them under `files`; files the code saves land in the workspace. `--prelude helpers.js` (repeatable) runs a script in the session before the code.

```fragment
goja-cv eval --json --root /tmp/specks --read ~/tiles --image ~/tiles/TILE.tif \
  -e 'return await cv.save("specks.csv", img.detect.specks({ polarity: "bright", maxArea: 2000 }))'
```
