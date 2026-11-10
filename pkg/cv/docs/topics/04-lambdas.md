# Lambdas

The one function you pass to cv is a builder lambda (within): it runs once while the graph is built and returns nodes. Data is filtered with JavaScript on awaited arrays. There are no per-pixel JavaScript functions.

## Builder lambdas: within

`img.within(mask, i => …)` applies an edit inside a mask. The function runs **once, immediately**, with a placeholder Image `i`; the nodes it returns become part of the graph and are blended through the mask in the image's space (`img.as("linear").within(…)` blends light). Capturing variables is fine: only the returned nodes are kept.

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

## Data: JavaScript on awaited arrays

Awaited regions are an array of items (`{ id, x, y, w, h, cx, cy, area, aspect, contrast, localNoise, sharpness, score, … }`): select them with JavaScript and pass the result back to an op.

```js
const specks = await img.detect.specks()
const elongated = specks.filter(r => r.aspect > 2 && r.contrast > 3 * r.localNoise)
return { n: elongated.length, of: specks.length, look: img.overlay(elongated).look("elongated") }
```

## Not available

Sampled lambdas (`curve(fn)`, `lut(fn)`, `mask.fromFunction`), `Value.map`, `cv.search` and per-pixel JavaScript are not implemented. Use the ops (`levels`, `contrast`, `exposure`, `mask.*`) instead.
