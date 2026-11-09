# Encodings and spaces

Every image has an encoding (srgb or linear: how its stored values relate to the quantity) and a space (encoded or linear: which numbers its pixels hold). Images open in file units; img.as("linear") gives light; an op works in its receiver's space, except physical ops (linear light) and pictures (encoded).

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

`cv.open(path, { encoding, box, origin, as })`: `box: [x, y, w, h]` reads only that window of the file and keeps the file's coordinates; `origin: [x, y]` declares where a file that is itself a window starts; `as` binds the image to a global name (see "Frames and names" below).

## The space of an image

An image's **space** says which numbers its pixels hold: `encoded` (the stored values, what a look shows) or `linear` (the encoding undone: light, or the quantity). An opened image is in `encoded` space: its numbers are the file's. `img.as("linear")` gives the same image in linear light, `x.as("encoded")` converts back; `await` and `cv.explain` show `space`. For a `linear`-encoded image the two are the same numbers and `as` changes nothing. For an `srgb` image they differ by the sRGB curve: encoded 0.5 is linear 0.214.

| space | use it when equal steps should count equally… |
|---|---|
| `encoded` | …to the eye: thresholds read off a look, contrast, visible specks |
| `linear` | …in the physical quantity: averaging light, densities, physical units |

## The receiver decides

An op works in the space of its receiver, and its result is in that space: `img.gaussian(2)` blurs file units, `img.as("linear").gaussian(2)` blurs light. Other images passed to it are converted to the receiver's space (`a.sub(b)` subtracts b in a's space); masks are weights and are never converted. A derived image (`sub`, `chroma`, `toLab`, `highpass`) is linear-encoded, so as a receiver it works in linear: put the photograph first (`img.add(d)`, not `d.add(img)`).

Two kinds of ops do not follow:

- **Physical** ops compute in linear light whatever the receiver, and return their result in the receiver's space: `exposure` (+1 EV doubles the light), `heal`, `inpaint`, `toLab`, `encode` and `toGray("rec709")`. `img.exposure(1)` is `img.as("linear").exposure(1).as("encoded")`.
- **Pictures** (`overlay`, `annotate`, `compare`, `sheet`) draw in encoded space.

`cv.explain` shows the conversions the graph inserts (captions hide them):

```js
return cv.explain(img.exposure(0.5).gaussian(2)).graph
```

## One luminance

`meanL`, `p01`, `p99` of `stats`, `mask.luminance`, `refine`, `detect.specks`, the histogram's L and the `meanL` of measurements all read the same luminance: `img.toGray()`, Rec. 709 luminance of linear light expressed in the receiver's space. So a number measured on an image is a threshold for an op on the same image:

```js
const s = await img.stats()
return await img.mask.luminance({ above: s.p99 }).coverage()   // about 1 % of the pixels
```

For physical units (light, density), measure `img.as("linear")`. Mixing images of different encodings in encoded space gives an `ENCODING` warning; its fix is `img.as("linear").op(…)`.

## Looks, masks and derived images

A look shows **encoded** values: an sRGB photo looks as usual; a linear-encoded image shows its stored values directly (density v as grey level v); a **mask** shows its weight (0.5 is mid-grey); a **derived** image shows its value as grey level.

A linear photograph (a raw scan) looks dark when shown as stored. `img.encode("srgb")` keeps its linear values and gives it the sRGB encoding, so looks and encoded-space ops see the curve:

```js
const raw = cv.open("planted.npy")
return { raw: raw.look("as stored"), photo: raw.encode("srgb").look("encoded as srgb") }
```

## Channels and false colour

`img.channel("g")` and `const [r, g, b] = img.split()` select planes. `img.chroma({ relative })` is the channel spread max − min (0 for neutral pixels: dust on colour film is neutral). `colormap("viridis" | "magma" | "gray" | "diverging", { range, center })` shows a one-channel image in false colour; without `range` it uses the image's p01..p99.

```js
const c = img.chroma({ relative: true })
const { meanL, p99 } = await c.stats()
return { neutral: { meanL, p99 }, look: c.colormap("viridis").look("chroma") }
```

## Frames and names

An image's **frame** is the rectangle it covers in the coordinates of its file: a window read with `box` keeps the scan's coordinates, so boxes, points and region centroids are absolute. Several images can be open under **names**: `cv.open(path, { as: "ref" })` binds `ref` as a global that persists between evals, next to `img`; the context block lists every name with its file, frame and encoding. Images combined by one op must share a frame (`E_FRAME`).

```js
cv.open("planted.npy", { as: "ref", origin: [0, 0] })
return { same: (await ref).frame, mean: (await img.as("linear").blend(ref, { opacity: 0.5 }).stats()).meanL }   // two encodings: blend light
```
