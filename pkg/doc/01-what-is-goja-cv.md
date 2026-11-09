---
Title: What goja-cv is and when to use it
Slug: what-is-goja-cv
Short: A lazy, measurable image API in JavaScript, built for agents that have an eval tool
Topics:
- goja-cv
- overview
Commands:
- goja-cv
IsTopLevel: true
IsTemplate: false
ShowPerDefault: true
SectionType: GeneralTopic
---

goja-cv is an image-analysis and editing engine written in Go and exposed to JavaScript as `require("cv")`
through [go-go-goja](https://github.com/go-go-golems/go-go-goja). You (or an LLM agent) write a few lines of
JavaScript; goja-cv builds a lazy graph of image operations, evaluates it in Go at the right scale, and returns
numbers as YAML and images as numbered "looks". The pixels never enter JavaScript.

It is built for one situation in particular: **an agent with an `eval` tool** that has to answer questions about
an image ("is this scan exposed well?", "how many dust specks, and where?") and then propose or make an edit,
with measurements instead of guesses.

## What it does

- **Measure.** `stats` (mean, percentiles, clipping, per channel; also inside a box or mask), `histogram`,
  `check.dust`, `detect.specks` (regions with id, box, area, contrast, sharpness, a dust score), `coverage`.
- **Select.** Masks from luminance, colour, boxes, ellipses, gradients and flood fill; set algebra and
  morphology; regions from masks; declarative `where` filters.
- **Edit.** `exposure` (in linear space), `levels`, `contrast`, `gaussian`, `blend`, `within(mask, …)`, `heal`
  (patch-based dust removal), `inpaint`.
- **Look.** `node.look("label")` returns an image the agent sees, with its number burned in and its op chain and
  scale in the caption; `look({ zoom, ruler })` magnifies a crop with absolute coordinates; `img.view("speck")`
  and `highpass`/`dog` make small things stand out; `cv.sheet` lays many crops out in one image;
  `cv.compare` puts images side by side; `overlay` and `annotate` draw regions, masks and marks.
- **Explain itself.** `cv.help(name)` and `cv.docs` (guides searchable from inside eval), errors with a literal
  `fix:`, warnings for lazy nodes that were never used.

```js
const s = await img.stats()
const dust = img.detect.specks().where({ score: { gte: 0.5 } })
return {
  exposure: { meanL: s.meanL, p01: s.p01, p99: s.p99, clipHigh: s.clipHigh },
  dust: await dust.count(),
  look: img.overlay(dust, { labels: "id" }).look("dust"),
}
```

## When to use it (and when not)

| you want | use |
|---|---|
| an agent to inspect and edit images through code, with structured answers and images back | **goja-cv** (MCP server or `goja-cv eval`) |
| a quick measurement or an edit you can reproduce as a script | goja-cv (`goja-cv run script.js --image …`) |
| format conversion, resizing, thumbnails in bulk | ImageMagick, libvips |
| research pipelines, ML models, NumPy-style array work | Python (OpenCV, scikit-image, NumPy) |
| raw-file development, ICC-managed printing | a raw developer / colour-managed editor |

Why not just give the agent Python? goja-cv is a small, safe surface: the eval runtime has **only** the `cv`
module (no filesystem, no processes, no network), `cv.open` is limited to the directories you allow, every
argument is checked with a useful error, and results come back small and structured. Agents make fewer
mistakes and you do not hand them a shell.

## What is implemented (and what is not yet)

Implemented: tone (exposure, levels, contrast, gray), blur, statistics and histograms, masks and regions,
`within`/`blend`, dust detection and healing, inpainting, overlays and comparisons, params, looks, the eval
session (YAML results, warnings, context block), the `cv.docs` guides, the CLI, the MCP server, an xgoja
provider.

Not yet: the document layer (history, `doc.apply`, `doc.propose`, undo, export of an edit history), sampled
lambdas (`curve(fn)`, LUTs), `cv.search`, ICC profiles and EXIF orientation, raw formats. Inputs are PNG, JPEG
and TIFF (8 or 16 bit), assumed sRGB.

## Where to go next

- `goja-cv help concepts` — the ideas in one page (lazy nodes, doc px, masks vs regions, looks, errors).
- `goja-cv help cli` — every command and flag.
- `goja-cv help agents` — what an agent sees over MCP, and the `cv.docs` guides.
- `goja-cv help setup-claude-code`, `goja-cv help setup-codex`, `goja-cv help setup-pi` — client setup.
- `goja-cv help recipes` — worked examples.
- `goja-cv help embedding` and `goja-cv help troubleshooting`.
