# Encodings and working spaces

Every image has an encoding (srgb or linear: how its stored values relate to the quantity), every pixel op has a working space (linear or encoded) chosen per call with { space }, and looks show encoded values.

## Encodings

An **encoding** says how the values a file stores relate to the quantity they measure. goja-cv knows two:

- `srgb`: the values are sRGB-encoded light (photographs: PNG, JPEG, ordinary TIFF). Undoing the curve gives linear light.
- `linear`: the stored value is the quantity (float TIFF, DNG raw scans, `.npy` arrays, density maps, depth maps).

Decoders say what a file implies; `await img` shows it as `encoding`. Override it when you know better: a 16-bit TIFF that stores density v directly is `linear`, although the TIFF default is `srgb`.

```js
const scan = cv.open("planted-linear.tif", { encoding: "linear", as: "scan" })
const data = cv.open("planted.npy")    // .npy is linear
return { img: (await img).encoding, scan: (await scan).encoding, npy: (await data).encoding }
```

`cv.open(path, { encoding, box, origin, as })`: `box: [x, y, w, h]` reads only that window of the file and keeps the file's coordinates; `origin: [x, y]` declares where a file that is itself a window starts; `as` binds the image to a global name (see "Names" below).

## The two working spaces

A **working space** is where an op does its arithmetic:

| space | the numbers are | use it when equal steps should count equally… |
|---|---|---|
| `linear` | the encoding undone (light, or the quantity) | …in the physical quantity: averaging, blurring, exposure, blending, measuring |
| `encoded` | the stored values, what a look shows | …to the eye: thresholds read off a look, contrast, visible specks |

For a `linear`-encoded image the two are the same numbers. For an `srgb` image they differ by the sRGB curve: encoded 0.5 is linear 0.214.

Every pixel op has a default and accepts `{ space: "linear" | "encoded" }`:

| default | ops |
|---|---|
| linear | `exposure`, `gaussian`, `blend`, `within`, `heal`, `inpaint`, `toGray`, `stats`, `levels` |
| encoded | `contrast`, `histogram`, `mask.luminance`, `mask.color`, `mask.flood`, `refine`, `detect.specks`, `check.dust`, `overlay`, `compare`, `chroma`, `colormap` |

Luminance ops (`mask.luminance`, `refine`, `detect.specks`, `check.dust`, `histogram` channel L) compute Rec. 709 luminance from linear light and then express it in their space. `explain()` shows the conversions the graph inserts:

```js
return img.mask.luminance({ above: 0.9 }).explain()
```

## Numbers carry their space

`stats()` runs in linear space by default, so `levels({ black: s.p01, white: s.p99 })` with `s = img.stats()` matches. When a level comes from another space than the op that uses it, you get a `SPACE` warning with the fix:

```js warns=SPACE
const s = img.stats()
return await img.mask.luminance({ above: s.p99 }).coverage()
```

```js
const s = img.stats({ space: "encoded" })
return await img.mask.luminance({ above: s.p99 }).coverage()
```

Mixing images of different encodings in encoded space gives an `ENCODING` warning; use `{ space: "linear" }` there.

## Looks, masks and derived images

A look shows **encoded** values: an sRGB photo looks as usual; a linear-encoded image shows its stored values directly (density v as grey level v); a **mask** shows its weight (0.5 is mid-grey); a **derived** image (`chroma`, `toLab`, differences) is linear-encoded, so the value you measure is the grey you see.

A linear photograph (a raw scan) looks dark when shown linearly. `img.encode("srgb")` keeps its linear values and gives it the sRGB encoding, so looks and encoded-space ops see the curve:

```js
const raw = cv.open("planted.npy")
return { raw: raw.look("as stored"), photo: raw.encode("srgb").look("encoded as srgb") }
```

## Channels and false colour

`img.channel("g")` and `const [r, g, b] = img.split()` select planes. `img.chroma({ relative })` is the channel spread max − min (0 for neutral pixels: dust on colour film is neutral). `colormap("viridis" | "magma" | "gray" | "diverging", { range, center })` shows a one-channel image in false colour; without `range` it uses the image's p01..p99.

```js
const c = img.chroma({ relative: true })
return { neutral: await c.stats().pick("meanL", "p99"), look: c.colormap("viridis").look("chroma") }
```

## Frames and names

An image's **frame** is the rectangle it covers in the coordinates of its file: a window read with `box` keeps the scan's coordinates, so boxes, points and region centroids are absolute. Several images can be open under **names**: `cv.open(path, { as: "ref" })` binds `ref` as a global that persists between evals, next to `img`; the context block lists every name with its file, frame and encoding. Images combined by one op must share a frame (`E_FRAME`).

```js
cv.open("planted.npy", { as: "ref", origin: [0, 0] })
return { same: (await ref).frame, diff: await img.blend(ref, { opacity: 0.5 }).stats().pick("meanL") }
```
