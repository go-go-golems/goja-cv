package ops

import "github.com/go-go-golems/goja-cv/pkg/cv/graph"

// Functions (GCV-002 §7.10): calls that do something now and return
// something other than a node. look and show share the look options; the
// look package reads them by name (look.OptionsFrom).

// DataKind is the receiver name of functions on Data handles (cv.json,
// cv.csv) in the registry.
const DataKind = "Data"

// dataNameArg is the name option of cv.json and cv.csv.
var dataNameArg = graph.ArgSpec{Name: "name", Type: "string", Optional: true,
	Doc: "the item's name, letters, digits, - and _ (the CLI writes name.csv / name.json); default data-<N>, N its content number"}

// MaxLookSide is the largest look side when fitting (the engine's default
// LookLong).
const MaxLookSide = 1568

// lookRedirects are the look options agents reach for that are content, not
// raster parameters: they are nodes placed in the chain.
var lookRedirects = map[string]graph.Redirect{
	"box":     {Hint: "A window is content: crop the node, then look at the crop.", Fix: "node.crop([x, y, w, h]).look()"},
	"crop":    {Hint: "A window is content: crop the node, then look at the crop.", Fix: "node.crop([x, y, w, h]).look()"},
	"stretch": {Hint: "A stretch is content: put levels({ auto: true }) in the chain where it should apply.", Fix: "node.levels({ auto: true }).look()"},
}

// LookArgs are the options of node.look and show.
func LookArgs() []graph.ArgSpec {
	return []graph.ArgSpec{
		{Name: "label", Type: "string", Positional: true, Optional: true, Doc: "caption text (default: the node's op)"},
		{Name: "zoom", Type: "number", Optional: true, Min: graph.F(1), Max: graph.F(graph.MaxRenderScale),
			Doc: "render scale: each doc px becomes zoom × zoom image px (default: fit to maxSide); exclusive with maxSide", Redirects: lookRedirects},
		{Name: "interp", Type: "enum:nearest,linear", Default: "nearest",
			Doc: "upsampling when zoomed: nearest repeats pixels (exact values, visible pixel edges), linear blends neighbours"},
		{Name: "maxSide", Type: "number", Optional: true, Min: graph.F(16), Max: graph.F(MaxLookSide),
			Doc: "long side in image px when fitting (default 1568)"},
		{Name: "ruler", Type: "number|bool", Unit: graph.DocPx, Optional: true,
			Doc: "coordinate rulers in margins: tick spacing in doc px, or true to pick 1, 2, 4, 8 … so ticks are ≥ 40 image px apart; labels are absolute doc px"},
		{Name: "grid", Type: "number", Unit: graph.DocPx, Optional: true, Min: graph.F(0),
			Doc: "faint lines every grid doc px over the image"},
		{Name: "format", Type: "enum:auto,png,jpeg", Default: "auto",
			Doc:           "auto: PNG for gray, zoomed, sheet and small (≤ 512 × 512) looks, JPEG (q85) otherwise; png is lossless (no ringing around specks)",
			ValueSynonyms: map[string]string{"lossless": "png", "jpg": "jpeg", "lossy": "jpeg", "PNG": "png", "JPEG": "jpeg"}},
	}
}

func funcDefs() []graph.FuncDef {
	return []graph.FuncDef{
		{
			Name: "look", On: []string{string(graph.KindImage), string(graph.KindMask), string(graph.KindValue)}, Args: LookArgs(),
			Doc: "A look of the node: an image delivered after the eval (Image, Mask; a histogram as bars, a profile as lines), whether or not it is returned, up to four per eval (a LOOKS warning names the others). Numbered, with what it shows and its scale in the caption.",
			See: []string{"compare", "overlay", "sheet"},
		},
		{
			Name: "explain", Bespoke: true, Usage: "cv.explain(node)",
			Doc: "A node's id, kind, op, call, frame, encoding, space, path and graph (one line per node: call, kind, size, cached), without evaluating anything.",
			See: []string{"look", "help"},
		},
		{
			Name: "docs", Bespoke: true, Usage: "cv.docs.list() · cv.docs.search(text, { limit }) · cv.docs.read(id, { section })",
			Doc: "The guides: list the topics, search their sections, read a topic or one section as Markdown.",
			See: []string{"help"},
		},
		{
			Name: "help", Bespoke: true, Usage: "cv.help() · cv.help(name)",
			Doc: "The op reference: no argument lists ops per kind; a name describes an op, a kind (\"Mask\"), Kind.method (\"Mask.coverage\") or a cv function. Never throws; unknown names return { found: false, didYouMean }.",
			See: []string{"docs"},
		},

		{
			Name: "json", Bespoke: true, Usage: "cv.json(x, { name })",
			Args: []graph.ArgSpec{
				{Name: "x", Type: "data", Positional: true, Doc: "a Value or Regions node (every item, evaluated at scale 1 when delivered) or plain data (as you see it, edits included)"},
				dataNameArg,
			},
			Doc: "Data for programs as JSON: returned from an eval it is delivered as content (MCP: text; CLI: the file name.json in --out), or saved with .save(path). Region items and measurement rows keep their schema's field order.",
			See: []string{"csv", "save"},
		},
		{
			Name: "csv", Bespoke: true, Usage: "cv.csv(x, { name, columns })",
			Args: []graph.ArgSpec{
				{Name: "x", Type: "data", Positional: true, Doc: "a Value or Regions node or plain data: a list of objects is one row each, a profile one row per sample"},
				dataNameArg,
				{Name: "columns", Type: "string[]", Optional: true, Doc: "the columns to write, in order (default: the schema's fields, or the keys in first-seen order)"},
			},
			Doc: "Data for programs as CSV with a header row: Regions give one row per region in schema order (id, x, y, w, h, cx, cy, area, aspect, …); nested values are JSON in their cell. Delivered as content or saved with .save(path).",
			See: []string{"json", "save"},
		},
		{
			Name: "save", On: []string{string(graph.KindImage), string(graph.KindMask), DataKind},
			Args: []graph.ArgSpec{
				{Name: "path", Type: "string", Positional: true, Doc: "file path relative to the out-root (--out-root); the extension picks the format: .tif, .png, .npy, .jpg for images, .csv or .json for data"},
				{Name: "depth", Type: "number", Optional: true, Doc: "bits per sample: TIFF 16 (default), 8 or 32 (float); PNG 16 or 8; NPY 32 (float) or 16"},
			},
			Usage: "node.save(path, { depth }) · cv.csv(x).save(path)",
			Doc: "Write an image (evaluated at scale 1, its encoding and frame origin recorded in TIFF and PNG) or a Data handle to a file under the out-root; " +
				"resolves to { path, format, depth, encoding, frame, bytes, replaced }. Without --out-root saving is disabled (E_ACCESS).",
			See: []string{"json", "csv", "look"},
		},
	}
}
