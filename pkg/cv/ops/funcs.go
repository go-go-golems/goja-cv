package ops

import "github.com/go-go-golems/goja-cv/pkg/cv/graph"

// Functions (GCV-002 §7.10): calls that do something now and return
// something other than a node. look and show share the look options; the
// look package reads them by name (look.OptionsFrom).

// MaxLookSide is the largest look side when fitting (the engine's default
// LookLong).
const MaxLookSide = 1568

// lookRedirects are the look options agents typed that are something else
// (GCV-004 §7.13): a window is content (a crop in the chain), a scale is the
// zoom.
var lookRedirects = map[string]graph.Redirect{
	"box":   {Hint: "A window is content: crop the node, then look at the crop.", Fix: "node.crop([x, y, w, h]).look()"},
	"crop":  {Hint: "A window is content: crop the node, then look at the crop.", Fix: "node.crop([x, y, w, h]).look()"},
	"scale": {Hint: "A look's magnification is zoom (render px per doc px).", Fix: "node.look({ zoom: 8 })"},
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
			Doc: "auto: PNG for gray, zoomed, sheet and small (≤ 512 × 512) looks, JPEG (q85) otherwise; png is lossless (no ringing around specks)"},
	}
}

func funcDefs() []graph.FuncDef {
	return []graph.FuncDef{
		{
			Name: "look", On: []string{string(graph.KindImage), string(graph.KindValue)}, Args: LookArgs(),
			Doc: "A look of the node: an image delivered after the eval (an Image or mask; a histogram as bars, a profile as lines), whether or not it is returned, up to four per eval (a LOOKS warning names the others). Numbered, with what it shows and its scale in the caption.",
			See: []string{"compare", "overlay", "sheet"},
		},
		{
			Name: "explain", Bespoke: true, Usage: "cv.explain(node)",
			Doc: "A node's id, kind, op, call, frame, encoding, space, channels, path and graph (one line per node: call, kind, size, cached), without evaluating anything.",
			See: []string{"look", "help"},
		},
		{
			Name: "docs", Bespoke: true, Usage: "cv.docs.list() · cv.docs.search(text, { limit }) · cv.docs.read(id, { section })",
			Doc: "The guides: list the topics, search their sections, read a topic or one section as Markdown.",
			See: []string{"help"},
		},
		{
			Name: "help", Bespoke: true, Usage: "cv.help() · cv.help(name)",
			Doc: "The op reference: no argument lists ops per kind; a name describes an op, a kind (\"Image\"), Kind.method (\"Image.overlay\"), a cv function or a concept (\"regions\"). Never throws; unknown names return { found: false, didYouMean }.",
			See: []string{"docs"},
		},

		{
			Name: "save", Bespoke: true, Usage: "await cv.save(path, x, { depth })",
			Args: []graph.ArgSpec{
				{Name: "path", Type: "string", Positional: true, Doc: "a file name in the workspace (--root); the extension picks the format: .tif, .png, .npy, .jpg for an Image, .json for any data, .csv for a list of rows"},
				{Name: "x", Type: "data", Positional: true, Doc: "an Image node (evaluated at scale 1), a Value or Regions node (its data at scale 1), or plain data as you hold it"},
				{Name: "depth", Type: "number", Optional: true, Doc: "images: bits per sample: TIFF 16 (default), 8 or 32 (float); PNG 16 or 8; NPY 32 (float) or 16"},
			},
			Doc: "Write a file in the workspace and resolve to { path, format, bytes, rows, depth, encoding, space, frame, replaced }; cv.open(saved.path) reopens it. " +
				"One save per path per eval (E_DUPLICATE); a save never follows a symlink or replaces a file the session opened (E_ACCESS); it writes a temporary file and renames it into place. Every save of an eval completes before the eval returns, awaited or not.",
			See: []string{"open", "look"},
		},
	}
}
