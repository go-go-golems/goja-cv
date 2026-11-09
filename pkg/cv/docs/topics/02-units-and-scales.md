# Units, coordinates and scales

Every length, box and point is in doc px (pixels of the file, absolute in the image's frame) at every scale; evaluation happens at preview scale; region detection runs at a fixed analysisScale; numbers are in the op's working space.

## Doc px

A box `[x, y, w, h]`, a point `[x, y]`, a blur width `sigma`, a grow `radius`: all are measured in pixels of the **original** image, whatever scale an evaluation runs at. Areas (`maxArea`, `minArea`, region `area`) are in doc px². The engine rescales lengths for each evaluation, so a preview and a full-size result agree.

```js
const corner = img.mask.box([0, 0, 64, 48])        // doc px
const soft = corner.feather(8)                      // 8 doc px wide ramp
return { coverage: await soft.coverage(), explain: soft.explain() }
```

`explain()` shows how a doc-px length maps at the evaluation scale (`sigma: 8 px → 2 at scale 0.25` on a large image).

## Scales

| when | scale |
|---|---|
| `await node` | preview: long side about 1600 px (1.0 for smaller images) |
| a look | look scale: long side at most 1568 px, or `maxSide` |
| `cv.evaluate(node, { scale })` | a number in (0, 1], `"preview"`, `"look"` or `"full"` |

An awaited Image gives a summary, never pixels: `{ kind, w, h, channels, scale, frame }` (`w`, `h` are buffer px at that scale; `frame` is `[x, y, w, h]` in doc px); an opened file adds `file`, `format`, `bitDepth` (8, 16, 32 or 64 bits per sample in the file), `sample` (`uint` or `float`), `fileChannels`, `encoding` (`srgb` or `linear`, see `encodings-and-spaces`; ICC profiles are not read) and `notes`. Statistics are computed at the scale they are evaluated at; for exact full-size numbers use `cv.evaluate(node, { scale: "full" })`.

```js
const s = img.stats()
return { preview: (await s).w, full: (await cv.evaluate(s, { scale: "full" })).w, summary: await img }
```

A look caption says `1 image px = S doc px`: multiply coordinates you read off a look by S to get doc px.

## analysisScale

`detect.specks`, `check.dust` and `Mask.components` analyse their input at a fixed `analysisScale` (default 1), whatever the evaluation scale. So region ids, boxes and areas are the same in a preview, a look and a full-size result. Lower it (e.g. 0.5) to detect faster on big scans; coordinates stay in doc px.

```js
const fast = img.detect.specks({ analysisScale: 0.5 })
return { full: await img.detect.specks().count(), half: await fast.count() }
```

## Frames

An image's **frame** is the rectangle it covers in doc px, `[x, y, w, h]`: `(await img).frame`. A whole file starts at (0, 0). A window read with `cv.open(path, { box: [x, y, w, h] })` keeps the file's coordinates, and `cv.open(path, { origin: [x, y] })` declares where a file that is itself a window starts. Boxes and points you pass and region boxes (`x, y, w, h`) and centroids (`cx, cy`) you read are absolute in that frame; a box outside it is `E_ARG_RANGE` with the nearest valid box as the fix. Images, masks and regions combined by one op must share a frame (`E_FRAME`).

```js
const f = (await img).frame
const { meanL, area } = await img.stats({ box: [f[0] + 10, f[1] + 10, 32, 32] })
return { frame: f, inside: { meanL, area } }
```

## Working spaces

Numbers are in the op's working space: `linear` (the encoding undone: exposure, blur, blend, stats, levels) or `encoded` (what looks show: thresholds, contrast, histogram, specks). Pass `{ space }` to change it; `cv.docs.read("encodings-and-spaces")` explains encodings, spaces and looks.
