package ops

import "github.com/go-go-golems/goja-cv/pkg/cv/graph"

// Functions (GCV-002 §7.10): calls that do something now and return
// something other than a node. look and show share the look options; the
// look package reads them by name (look.OptionsFrom).

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
	}
}

func funcDefs() []graph.FuncDef {
	return []graph.FuncDef{
		{
			Name: "look", On: []string{string(graph.KindImage), string(graph.KindMask), string(graph.KindValue)}, Args: LookArgs(),
			Doc: "An image of the node to return in the result (Image, Mask, histogram). Numbered, with what it shows and its scale in the caption.",
			See: []string{"show", "compare", "overlay"},
		},
		{
			Name: "show", Global: true, Args: LookArgs(), Bespoke: true,
			Usage: "show(node, label | { label, zoom, interp, maxSide }) · show(look)",
			Doc:   "Create a look, queue it and return it; queued looks are delivered even if the eval throws later.",
			See:   []string{"look"},
		},
	}
}
