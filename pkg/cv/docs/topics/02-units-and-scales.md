# Units, coordinates and scales

Every length, box and point is in doc px (pixels of the original image) at every scale; pixels are linear-light float32; evaluation happens at preview scale; region detection runs at a fixed analysisScale.

## Doc px

A box `[x, y, w, h]`, a point `[x, y]`, a blur radius `r`, a grow distance: all are measured in pixels of the **original** image, whatever scale an evaluation runs at. Areas (`maxArea`, `minArea`, region `area`) are in doc px². The engine rescales lengths for each evaluation, so a preview and a full-size result agree.

```js
const corner = img.mask.box([0, 0, 64, 48])        // doc px
const soft = corner.feather(8)                      // 8 doc px wide ramp
return { coverage: await soft.coverage(), explain: soft.explain() }
```

`explain()` shows how a doc-px length maps at the evaluation scale (`r: 8 px → 2 at scale 0.25` on a large image).

## Scales

| when | scale |
|---|---|
| `await node` | preview: long side about 1600 px (1.0 for smaller images) |
| a look | look scale: long side at most 1568 px, or `maxSide` |
| `cv.evaluate(node, { scale })` | a number in (0, 1], `"preview"`, `"look"` or `"full"` |

An awaited Image gives a summary, never pixels: `{ kind, w, h, channels, scale, docW, docH }`; an opened file adds `file`, `format`, `bitDepth` (8 or 16 bits per sample in the file), `fileChannels` and `colorSpace` (sRGB is assumed; ICC profiles are not read). Statistics are computed at the scale they are evaluated at; for exact full-size numbers use `cv.evaluate(node, { scale: "full" })`.

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

## Linear and perceptual

Pixels are linear-light float32: 0 is black, 1 is white, values above 1 can occur while editing. Ops declare their space and convert themselves:

- linear: `exposure` (×2^ev), `gaussian`, `blend`, `within`, `heal`, `stats` (meanL, p01, p99 are linear).
- perceptual (sRGB-encoded): `levels` midtones and `gamma`, `contrast`, `histogram` bins, `mask.luminance` thresholds.
- `levels({ black, white })` takes **linear** input levels, the same space as `stats()`, so `img.levels({ black: s.p01, white: s.p99 })` works directly.
