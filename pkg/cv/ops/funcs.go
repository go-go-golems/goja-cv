package ops

import "github.com/go-go-golems/goja-cv/pkg/cv/graph"

// Functions (GCV-002 §7.10): calls that do something now and return
// something other than a node. look and show share the look options; the
// look package reads them by name (look.OptionsFrom).

// DataKind is the receiver name of functions on Data handles (cv.json,
// cv.csv) in the registry.
const DataKind = "Data"

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
			Doc: "An image of the node to return in the result (Image, Mask; a histogram as bars, a profile as lines). Numbered, with what it shows and its scale in the caption.",
			See: []string{"show", "compare", "overlay"},
		},
		{
			Name: "evaluate", Bespoke: true, Usage: "cv.evaluate(node, { scale })",
			Doc: "Evaluate a node at an explicit scale: a number in (0, 1], \"preview\" (the default of await), \"look\" or \"full\" (1.0). Returns what await returns.",
			See: []string{"look", "stats"},
		},
		{
			Name: "describe", Bespoke: true, Usage: "cv.describe(node)",
			Doc: "A node's id, kind, op, call, frame and graph path, without evaluating it.",
			See: []string{"explain"},
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
			Name: "isNode", Bespoke: true, Usage: "cv.isNode(x)",
			Doc: "true if x is a cv node (lazy), false for plain data.",
		},
		{
			Name: "warnings", Bespoke: true, Usage: "cv.warnings({ clear })",
			Doc: "Build-time warnings collected while building nodes (they also appear under warnings: in every result).",
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
		{
			Name: "show", Global: true, Args: LookArgs(), Bespoke: true,
			Usage: "show(node, label | { label, zoom, interp, maxSide, ruler, grid, format }) · show(look)",
			Doc:   "Create a look, queue it and return it; queued looks are delivered even if the eval throws later.",
			See:   []string{"look"},
		},
	}
}
