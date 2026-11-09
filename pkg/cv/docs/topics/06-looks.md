# Looks and images

A look is an image you return: node.look(label) in the result, or show(node). Looks are numbered, carry their scale in the caption, are rendered after the eval, and at most four arrive per eval.

## Returning looks

`node.look("label")` or `node.look({ label, maxSide })` returns a look object. Put it anywhere in the returned value; the YAML shows a placeholder with the caption and the image arrives next to the text.

```js
const s = img.stats()
const fixed = img.levels({ black: s.p01, white: s.p99 })
return { meanL: await fixed.stats().meanL, before: img.look("before"), after: fixed.look({ label: "after", maxSide: 800 }) }
```

```yaml
meanL: 0.39417
before: '[look 1: before (256×192, 1 image px = 1.0 doc px)]'
after: '[look 2: after (256×192, 1 image px = 1.0 doc px)]'
```

## show()

`show(node, opts)` creates the same look, queues it, and returns it. Queued looks are listed under `shown:` and are delivered even if the eval throws afterwards, which helps when you are not sure the rest of the code works.

```js
show(img, "before")
const s = await img.stats()
return { meanL: s.meanL }
```

## Captions and coordinates

`look 3: specks (1568×1045, 1 image px = 4.0 doc px)`: the number (also burned into the top-left corner, so you can refer to it later), the label, the size, and the scale. Multiply coordinates read off the look by the doc-px factor to get document coordinates for `mask.box`, `mask.flood` and friends.

## What a look shows

A look shows **encoded** values: an sRGB photo looks as usual, a linear-encoded image (`cv.open(path, { encoding: "linear" })`, `.npy`, DNG) shows its stored values directly, a mask shows its weight (0.5 is mid-grey), and a derived image (`chroma`, `toLab`) shows its value as grey level. `img.encode("srgb")` makes a linear photograph look normal. See `cv.docs.read("encodings-and-spaces")`.

## What can be looked at

- Image and Mask nodes (masks and gray images arrive as PNG, colour as JPEG); one-channel maps in false colour with `colormap("viridis")`;
- `histogram()` Value nodes render as a bar chart;
- Regions: draw them with `img.overlay(regions, { labels: "id" })`;
- several images side by side: `cv.compare([a, b], { mode: "side" | "split" | "diff" })`.

```js
const fixed = img.exposure(0.4)
show(cv.compare([img, fixed], { mode: "side" }), { label: "before / after" })
show(fixed.histogram())
return await fixed.stats().pick("meanL", "clipHigh")
```

Other Value nodes have no look (`E_WRONG_KIND`): await them instead.
