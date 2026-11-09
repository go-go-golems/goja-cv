# Params

A param is a named value the person can move (a slider, toggle or choice). Params are nodes, so they go anywhere a value goes; the context block reports when the person moved one.

## Declaring

```js
const ev = P.slider("ev", { min: -2, max: 2, step: 0.05, value: 0.3, label: "Exposure", unit: "EV" })
const on = P.toggle("heal", { value: true })
const mode = P.choice("mode", { options: ["patch", "diffuse"], value: "patch" })
const brighter = img.exposure(ev)
return { meanL: await brighter.stats().meanL, ev: ev.value, on: on.value, mode: mode.value }
```

- Names are unique per session: declaring `P.slider("ev", …)` again returns the existing param, and the person's value wins (`ev.touched` is true once they moved it). Pass `{ reset: true }` to re-declare with a different range.
- `ev.value`, `ev.touched`, `ev.name` read the current state; `P.list()` lists all params.
- Out-of-range values are errors, not clamps.

## Params inside graphs

Only nodes downstream of a param recompute when it moves. `where` predicates take params too:

```js
const minScore = P.slider("minScore", { min: 0, max: 1, value: 0.5 })
const dust = img.detect.specks().where({ score: { gte: minScore } })
const before = await dust.count()
P.set("minScore", 0.9)            // simulates the person moving the slider
return { before, after: await dust.count() }
```

## The context block

After each eval the context lists params with their values and marks those the person moved; when a value changed between two of your evals, a `person:` line says so:

```yaml
context:
  params: ev = 0.3, minScore = 0.9 (moved by the person)
  person: 'since your last eval: moved minScore 0.5 → 0.9'
```

There is no slider UI in the MCP server yet; `P.set(name, value)` moves a param as the person would. Proposals (`doc.propose`) arrive with the document layer.
