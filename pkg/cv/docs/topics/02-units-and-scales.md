# Units, coordinates and scales

Every length, box and point is in doc px (pixels of the file, absolute in the image's frame) at every scale; data is measured at full resolution and looks render at look scale; region detection runs at a fixed analysisScale; numbers are in the space of the image they were measured on.

## Doc px

A box `[x, y, w, h]`, a point `[x, y]`, a blur width `sigma`, a morphology `radius`: all are measured in pixels of the **original** image, whatever scale an evaluation runs at. Areas (`maxArea`, `minArea`, region `area`) are in doc px². The engine rescales lengths for each evaluation, so a look (rendered at look scale) and a full-size result agree.

```js
const corner = img.mask.box([0, 0, 64, 48])        // doc px
const soft = corner.feather(8)                      // 8 doc px wide ramp
return { coverage: await soft.coverage(), graph: cv.explain(soft).graph }
```

`cv.explain(node).graph` shows each node's call with its doc-px lengths.

## Scales

| when | scale |
|---|---|
| `await node` (a Value or Regions) | 1: full resolution, exact numbers |
| `await node` (an Image) | nothing is computed: its summary |
| a look | look scale: long side at most 1568 px, or `maxSide` (zoom renders above 1) |

An awaited Image gives a summary, never pixels: `{ kind, w, h, encoding, space, frame }` (`w`, `h` are its size in doc px; `frame` is `[x, y, w, h]` in doc px); an opened file adds `file`, `format`, `bitDepth` (8, 16, 32 or 64 bits per sample in the file), `sample` (`uint` or `float`), `fileChannels` and `notes` (`encoding` is `srgb` or `linear`, see `encodings-and-spaces`; ICC profiles are not read). Awaiting an Image is free, even on a large scan. Statistics are computed at full resolution: crop large scans before measuring.

```js
const s = await img.stats()
return { w: s.w, summary: await img }
```

A look caption says `1 image px = S doc px`: multiply coordinates you read off a look by S to get doc px.

## analysisScale

`detect.specks`, `check.dust` and `components` analyse their input at a fixed `analysisScale` (default 1), whatever the evaluation scale. So region ids, boxes and areas are the same in a look and a full-size result. Lower it (e.g. 0.5) to detect faster on big scans; coordinates stay in doc px.

```js
const fast = img.detect.specks({ analysisScale: 0.5 })
return { full: (await img.detect.specks()).length, half: (await fast).length }
```

## Frames

An image's **frame** is the rectangle it covers in doc px, `[x, y, w, h]`: `(await img).frame`. A whole file starts at (0, 0). A window read with `cv.open(path, { box: [x, y, w, h] })` keeps the file's coordinates, and `cv.open(path, { origin: [x, y] })` declares where a file that is itself a window starts. Boxes and points you pass and region boxes (`x, y, w, h`) and centroids (`cx, cy`) you read are absolute in that frame; a box outside it is `E_ARG_RANGE` with the nearest valid box as the fix. Images, masks and regions combined by one op must share a frame (`E_FRAME`).

```js
const f = (await img).frame
const { meanL, area } = await img.stats({ box: [f[0] + 10, f[1] + 10, 32, 32] })
return { frame: f, inside: { meanL, area } }
```

## Spaces

Images open in file units (`space: "encoded"`, what looks show); an op works in its receiver's space and returns it, so a level measured on an image fits ops on the same image. `img.as("linear")` gives light (or density); `cv.docs.read("encodings-and-spaces")` explains encodings, spaces, physical ops and looks.
