package ops

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

// High-pass views and presets (GCV-002 §7.5, §7.11). highpass, dog and view
// are macros: their expansion's nodes are the same (hash-consed) nodes as
// the hand-written chain, and their macro node names the view in captions.
// localContrast is a kernel (it needs a square and a square root).

var polarityArg = graph.ArgSpec{Name: "polarity", Type: "enum:both,bright,dark", Default: "both",
	Doc:           "both: the signed residual; bright: only what is brighter than its surroundings (d.max(0)); dark: only what is darker, as positive values (d.mul(-1).max(0))",
	ValueSynonyms: map[string]string{"positive": "bright", "light": "bright", "white": "bright", "negative": "dark", "black": "dark", "signed": "both", "all": "both"}}

// polarity applies the polarity option to a signed residual d.
func polarity(x *graph.Expander, d *graph.Node, p, space string) ([]*graph.Node, error) {
	sp := map[string]any{graph.SpaceArg: space}
	switch p {
	case "bright":
		n, err := x.Call("max", d, 0.0, sp)
		return []*graph.Node{n}, err
	case "dark":
		neg, err := x.Call("mul", d, -1.0, sp)
		if err != nil {
			return nil, err
		}
		n, err := x.Call("max", neg, 0.0, sp)
		return []*graph.Node{n}, err
	}
	return []*graph.Node{d}, nil
}

func expandHighpass(x *graph.Expander, a *graph.Node, args graph.Args) ([]*graph.Node, error) {
	sp := args.String(graph.SpaceArg) // resolved: encoded unless the call said otherwise
	g, err := x.Call("gaussian", a, x.Arg(args, "sigma"), map[string]any{graph.SpaceArg: sp})
	if err != nil {
		return nil, err
	}
	d, err := x.Call("sub", a, graph.NodeArg{ID: g.ID}, map[string]any{graph.SpaceArg: sp})
	if err != nil {
		return nil, err
	}
	return polarity(x, d, args.String("polarity"), sp)
}

func expandDoG(x *graph.Expander, a *graph.Node, args graph.Args) ([]*graph.Node, error) {
	sp := args.String(graph.SpaceArg)
	o := map[string]any{graph.SpaceArg: sp}
	g1, err := x.Call("gaussian", a, x.Arg(args, "sigma1"), o)
	if err != nil {
		return nil, err
	}
	g2, err := x.Call("gaussian", a, x.Arg(args, "sigma2"), o)
	if err != nil {
		return nil, err
	}
	d, err := x.Call("sub", g1, graph.NodeArg{ID: g2.ID}, o)
	if err != nil {
		return nil, err
	}
	return polarity(x, d, args.String("polarity"), sp)
}

func literal(a graph.Args, name string) (float64, bool) {
	v, ok := a.Raw(name).(float64)
	return v, ok
}

func filterViewOps() []graph.OpDef {
	sigma := func(name, doc string, def any) graph.ArgSpec {
		return graph.ArgSpec{Name: name, Type: "number", Unit: graph.DocPx, Positional: true, Default: def, Min: graph.F(0.1), Max: graph.F(500),
			Usual: [2]float64{0.3, 100}, Doc: doc}
	}
	return []graph.OpDef{
		{
			Name: "highpass", On: graph.KindImage, Returns: graph.KindImage, Space: graph.SpaceRule{Default: buffer.SpaceEncoded},
			Args:   []graph.ArgSpec{sigma("sigma", "σ of the blur subtracted, doc px: keeps what differs from its surroundings at about this size", nil), polarityArg},
			Halo:   func(a graph.Args) float64 { return 3 * a.DocLength("sigma") },
			Expand: expandHighpass,
			Doc:    "What differs from its surroundings at size σ: a − G_sigma(a), signed (positive where brighter), in the working space (encoded by default). A derived image: show it with .mul(4).add(0.5), a diverging colormap, or view(\"signed\").",
			See:    []string{"dog", "localContrast", "view", "sub"},
		},
		{
			Name: "dog", On: graph.KindImage, Returns: graph.KindImage, Space: graph.SpaceRule{Default: buffer.SpaceEncoded},
			Args: []graph.ArgSpec{
				sigma("sigma1", "σ of the first blur, doc px: suppresses the finest grain", nil),
				sigma("sigma2", "σ of the second blur, doc px (> sigma1): the size of the surroundings", nil),
				polarityArg,
			},
			Validate: func(a graph.Args, _ []*graph.Node) error {
				s1, ok1 := literal(a, "sigma1")
				s2, ok2 := literal(a, "sigma2")
				if ok1 && ok2 && !(s2 > s1) {
					lo, hi := min(s1, s2), max(s1, s2)
					if lo == hi {
						hi = 4 * lo
					}
					return graph.Errorf(graph.EArgRange, "dog(%g, %g): sigma2 must be larger than sigma1 (the first blur is the finer one).", s1, s2).
						WithFix(fmt.Sprintf("img.dog(%g, %g)", lo, hi))
				}
				return nil
			},
			Halo:   func(a graph.Args) float64 { return 3 * a.DocLength("sigma2") },
			Expand: expandDoG,
			Doc:    "Difference of Gaussians G_sigma1(a) − G_sigma2(a) with sigma1 < sigma2, signed, in the working space (encoded by default): the first blur suppresses grain, so small specks rank higher than with highpass. dog(1, 4) for 1–3 px specks.",
			See:    []string{"highpass", "view"},
		},
		{
			Name: "localContrast", On: graph.KindImage, Returns: graph.KindImage, Space: graph.SpaceRule{Default: buffer.SpaceEncoded}, Result: graph.DerivedEnc,
			Args: []graph.ArgSpec{sigma("sigma", "σ of the Gaussian window, doc px, for the local mean and spread", nil)},
			Halo: func(a graph.Args) float64 { return 3 * a.DocLength("sigma") },
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
				out, err := kernels.LocalContrast(ctx, src.Buf, a.Length("sigma", src.Buf.Scale))
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out), nil
			},
			Doc: "Local z-scores (a − G_sigma(a)) / (local spread + 0.001), in the working space (encoded by default): 3 means three local standard deviations above the local mean, in a dark corner and a bright one alike. A derived image.",
			See: []string{"highpass", "view"},
		},
	}
}

// Preset is one named view: a description (help, the views topic) and its
// expansion on image a with target size s (doc px) in working space sp.
type Preset struct {
	Name   string
	Doc    string
	Expand func(x *graph.Expander, a *graph.Node, s float64, sp string) (*graph.Node, error)
}

// Presets are the view presets of img.view(name, { size }).
var Presets = []Preset{
	{"scene", "the image stretched 1–99 %: judging what belongs to the scene", func(x *graph.Expander, a *graph.Node, _ float64, sp string) (*graph.Node, error) {
		return x.Call("levels", a, map[string]any{"auto": true, graph.SpaceArg: sp})
	}},
	{"speck", "small bright things that differ from their surroundings (grey DoG σ size/3 vs 4·size/3, white = 5 × p99): dust on density tiles, stars, sensor defects",
		func(x *graph.Expander, a *graph.Node, s float64, sp string) (*graph.Node, error) {
			return speckView(x, a, s, sp, "bright", true)
		}},
	{"speckDark", "small dark things, shown bright: dust on positives and prints", func(x *graph.Expander, a *graph.Node, s float64, sp string) (*graph.Node, error) {
		return speckView(x, a, s, sp, "dark", true)
	}},
	{"speckColor", "the speck view on all channels: neutral specks stay white, colour grain is coloured", func(x *graph.Expander, a *graph.Node, s float64, sp string) (*graph.Node, error) {
		return speckView(x, a, s, sp, "bright", false)
	}},
	{"signed", "brighter (red) and darker (blue) than the surroundings at size s: edges, scratches, both polarities", func(x *graph.Expander, a *graph.Node, s float64, sp string) (*graph.Node, error) {
		o := map[string]any{graph.SpaceArg: sp}
		g, err := x.Call("toGray", a, "average", o)
		if err != nil {
			return nil, err
		}
		h, err := x.Call("highpass", g, s, o)
		if err != nil {
			return nil, err
		}
		return x.Call("colormap", h, "diverging", o)
	}},
	{"detail", "local contrast (σ 4·size) in a diverging colormap of ±4: detail equally visible in shadows and highlights, for sweeping a whole tile",
		func(x *graph.Expander, a *graph.Node, s float64, sp string) (*graph.Node, error) {
			o := map[string]any{graph.SpaceArg: sp}
			g, err := x.Call("toGray", a, "average", o)
			if err != nil {
				return nil, err
			}
			l, err := x.Call("localContrast", g, 4*s, o)
			if err != nil {
				return nil, err
			}
			return x.Call("colormap", l, "diverging", map[string]any{"range": []any{-4.0, 4.0}, graph.SpaceArg: sp})
		}},
	{"neutral", "where the channels disagree (relative chroma in viridis, 0–1): grey dust versus coloured detail", func(x *graph.Expander, a *graph.Node, _ float64, sp string) (*graph.Node, error) {
		c, err := x.Call("chroma", a, map[string]any{"relative": true, graph.SpaceArg: sp})
		if err != nil {
			return nil, err
		}
		return x.Call("colormap", c, "viridis", map[string]any{"range": []any{0.0, 1.0}, graph.SpaceArg: sp})
	}},
}

// speckView is a bright (or dark) DoG of σ s/3 vs 4s/3, grey or on all
// channels, stretched to white = 5 × its p99.
func speckView(x *graph.Expander, a *graph.Node, s float64, sp, pol string, gray bool) (*graph.Node, error) {
	o := map[string]any{graph.SpaceArg: sp}
	src := a
	if gray {
		var err error
		if src, err = x.Call("toGray", a, "average", o); err != nil {
			return nil, err
		}
	}
	d, err := x.Call("dog", src, s/3, 4*s/3, map[string]any{"polarity": pol, graph.SpaceArg: sp})
	if err != nil {
		return nil, err
	}
	st, err := x.Call("stats", d, o)
	if err != nil {
		return nil, err
	}
	p99, err := x.Call("field", st, "p99")
	if err != nil {
		return nil, err
	}
	w, err := x.Call("mul", p99, 5.0)
	if err != nil {
		return nil, err
	}
	if w, err = x.Call("max", w, 1e-4); err != nil { // a flat window has p99 0
		return nil, err
	}
	return x.Call("levels", d, map[string]any{"black": 0.0, "white": graph.NodeArg{ID: w.ID}, graph.SpaceArg: sp})
}

func presetNames() []string {
	out := make([]string, len(Presets))
	for i, p := range Presets {
		out[i] = p.Name
	}
	return out
}

func presetDocs() map[string]string {
	out := map[string]string{}
	for _, p := range Presets {
		out[p.Name] = p.Doc
	}
	return out
}

func viewOp() graph.OpDef {
	return graph.OpDef{
		Name: "view", On: graph.KindImage, Returns: graph.KindImage, Space: graph.SpaceRule{Default: buffer.SpaceEncoded},
		Args: []graph.ArgSpec{
			{Name: "name", Type: "enum:" + strings.Join(presetNames(), ","), Positional: true, ValueDocs: presetDocs(),
				ValueSynonyms: map[string]string{"dust": "speck", "specks": "speck", "stars": "speck", "dog": "speck", "highpass": "signed",
					"edges": "signed", "scratches": "signed", "contrast": "detail", "clahe": "detail", "local": "detail",
					"color": "neutral", "colour": "neutral", "chroma": "neutral", "stretch": "scene", "auto": "scene", "plain": "scene",
					"darkSpeck": "speckDark", "speckColour": "speckColor"},
				Doc: "the preset: " + strings.Join(presetNames(), ", ") + " (cv.docs.read(\"views\") describes each)"},
			{Name: "size", Type: "number", Unit: graph.DocPx, Default: 3.0, Min: graph.F(0.5), Max: graph.F(200), Usual: [2]float64{1, 30}, Static: true,
				Doc: "typical size of the target in doc px; the presets derive their σ from it"},
		},
		Halo: func(a graph.Args) float64 { return 4 * a.DocLength("size") },
		Expand: func(x *graph.Expander, a *graph.Node, args graph.Args) ([]*graph.Node, error) {
			name := args.String("name")
			for _, p := range Presets {
				if p.Name == name {
					n, err := p.Expand(x, a, args.DocLength("size"), args.String(graph.SpaceArg))
					if err != nil {
						return nil, err
					}
					return []*graph.Node{n}, nil
				}
			}
			return nil, graph.Errorf(graph.EInternal, "no view preset %q", name)
		},
		Doc: "A named inspection view: img.view(\"speck\") (small bright things), \"speckDark\", \"speckColor\", \"signed\", \"detail\", \"neutral\" or \"scene\". size (doc px, default 3) is the typical size of the target. The caption names the view; explain() shows its expansion.",
		See: []string{"dog", "highpass", "localContrast", "levels"},
	}
}
