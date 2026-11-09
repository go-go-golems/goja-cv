# Lambdas

There are two kinds of function you can pass to cv: builder lambdas (within) run once while the graph is built and return nodes; data lambdas (Regions.filter) run at evaluation time on small data. There are no per-pixel JavaScript functions.

## Builder lambdas: within

`img.within(mask, i => …)` applies an edit inside a mask. The function runs **once, immediately**, with a placeholder Image `i`; the nodes it returns become part of the graph and are blended through the mask in linear space (within's working space). Capturing variables is fine: only the returned nodes are kept, and params used inside are tracked like any other.

```js
const top = img.mask.gradient({ from: "top", to: 0.45 })
const darker = img.within(top, i => i.exposure(-0.5))
return { look: darker.look("darker sky") }
```

The function must return a node built from its argument, synchronously:

```js error=E_ARG_TYPE
return img.within(img.mask.box([0, 0, 50, 50]), i => 0.5).look()
```

You can also pass an edited node directly: `img.within(mask, img.exposure(-0.5))`.

## Data lambdas: Regions.filter

`regions.filter(r => …)` calls your function with each region's data (`{ id, box, area, centroid, contrast, localNoise, sharpness, score, … }`) at evaluation time, on the JavaScript thread. Fine for hundreds of regions.

```js
const specks = img.detect.specks()
const elongated = specks.filter(r => r.aspect > 2 && r.contrast > 3 * r.localNoise)
return { n: await elongated.count(), of: await specks.count() }
```

Prefer `where()` when the test can be written as data: it runs in Go and follows params.

```js
const specks = img.detect.specks()
return await specks.where({ score: { gte: 0.5 }, area: { lte: 60 } }).count()
```

## Not available

Sampled lambdas (`curve(fn)`, `lut(fn)`, `mask.fromFunction`), `Value.map`, `cv.search` and per-pixel JavaScript are not implemented. Use the ops (`levels`, `contrast`, `exposure`, `mask.*`) instead.
