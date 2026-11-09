# Data out

The YAML result is for you to read and is capped; data for programs leaves as JSON or CSV content (cv.json, cv.csv) or as files saved under the out-root (node.save, data.save), with images keeping their encoding and frame.

## Text for you, data for programs

The result of an eval comes back as YAML capped at about 9000 characters and 48 items per list. Return summaries there. A program that needs every row gets them from a data item: `cv.json(x)` or `cv.csv(x)` returned from the eval is delivered as content (MCP: text; CLI: a file named after it in `--out`). Awaited Regions hold every item (up to 100000), so code may loop over `items` freely.

```js
const specks = img.detect.specks()
return {
  count: await specks.count(),
  table: cv.csv(specks, { name: "specks" }),
  probe: cv.json(await img.measure([[200.5, 46.5]]), { name: "probe" }),
}
```

## cv.json and cv.csv

`x` is a Value or Regions node (evaluated at scale 1 when delivered, every item) or plain data (written as you see it, edits included). Region items and measurement rows keep their schema order (`id, x, y, w, h, cx, cy, area, aspect, …`).

- `name` is a plain name (letters, digits, `-`, `_`): the item becomes `name.csv` or `name.json`. Without a name it is `data-<N>`, N its content number.
- CSV has a header row; `columns: ["id", "cx", "cy", "snr"]` selects and orders; a profile gives one row per sample; nested values are JSON in their cell.
- At most 4 looks and 4 data items per eval; over MCP an item larger than 256 KiB is replaced by a note to save it.

## Saving files

`node.save(path, { depth })` writes an Image or Mask, `cv.csv(x).save(path)` a data item, under the out-root: the directory the session was started with (`--out-root DIR`). Without it saving is `E_ACCESS`. Paths are relative to the out-root; `..` and symlinks may not leave it. The promise resolves to `{ path, format, depth, encoding, frame, bytes, replaced }`.

| extension | depths (default first) | records |
|---|---|---|
| `.tif` | 16, 8, 32 (float) | values, encoding, frame origin |
| `.png` | 16, 8 | values, encoding (gAMA/sRGB), frame origin |
| `.npy` | 32 (float), 16 | values only: reopens as linear at (0, 0) |
| `.jpg` | 8 | sRGB pictures only |

Images are evaluated at scale 1 and written as their encoded values; masks as linear. Integer depths clamp values outside [0, 1] (a `CLAMP` warning: a stretched view saturates above white, which is usually fine for a picture); signed images go to `{ depth: 32 }` TIFF or `.npy`. A saved TIFF opens again with the same numbers and coordinates: `cv.open(saved.path)`.

```js warns=CLAMP
const v = await img.view("speck").save("views/tile-17-speck.tif")
const back = await cv.open(v.path)
const t = await cv.csv(img.detect.specks()).save("tables/tile-17.csv")
return { saved: [v.encoding, v.frame], reopened: [back.encoding, back.frame], rows: t.rows }
```

## From the command line

`goja-cv eval --json --out DIR` writes looks as `look-<n>-<slug>.png` and data items as `<name>.csv` / `<name>.json` into DIR and lists them under `files` with `path`, `mimeType`, `bytes` and, for data, `rows`. `--prelude helpers.js` (repeatable) runs a script in the session before the code, so helpers on `mem` need not be pasted into every call; `--out-root DIR` enables saving.

```fragment
goja-cv eval --json --out /tmp/specks --image TILE.tif \
  -e 'return cv.csv(img.detect.specks({ polarity: "bright", maxArea: 2000 }), { name: "specks" })'
```
