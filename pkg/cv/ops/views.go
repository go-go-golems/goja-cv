package ops

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

// High-pass views and presets (GCV-002 §7.5, §7.11). highpass, dog and view
// are macros: their expansion's nodes are the same (hash-consed) nodes as
// the hand-written chain, and their macro node names the view in captions.
// localContrast is a kernel (it needs a square and a square root).

var polarityArg = graph.ArgSpec{Name: "polarity", Type: "enum:both,bright,dark", Default: "both",
	Doc: "both: the signed residual; bright: only what is brighter than its surroundings (d.max(0)); dark: only what is darker, as positive values (d.mul(-1).max(0))"}

// polarity applies the polarity option to a signed residual d.
func polarity(x *graph.Expander, d *graph.Node, p string) ([]*graph.Node, error) {
	switch p {
	case "bright":
		n, err := x.Call("max", d, 0.0)
		return []*graph.Node{n}, err
	case "dark":
		neg, err := x.Call("mul", d, -1.0)
		if err != nil {
			return nil, err
		}
		n, err := x.Call("max", neg, 0.0)
		return []*graph.Node{n}, err
	}
	return []*graph.Node{d}, nil
}

func expandHighpass(x *graph.Expander, a *graph.Node, args graph.Args) ([]*graph.Node, error) {
	g, err := x.Call("gaussian", a, args.DocLength("sigma"))
	if err != nil {
		return nil, err
	}
	d, err := x.Call("sub", a, graph.NodeArg{ID: g.ID})
	if err != nil {
		return nil, err
	}
	return polarity(x, d, args.String("polarity"))
}

func expandDoG(x *graph.Expander, a *graph.Node, args graph.Args) ([]*graph.Node, error) {
	g1, err := x.Call("gaussian", a, args.DocLength("sigma1"))
	if err != nil {
		return nil, err
	}
	g2, err := x.Call("gaussian", a, args.DocLength("sigma2"))
	if err != nil {
		return nil, err
	}
	d, err := x.Call("sub", g1, graph.NodeArg{ID: g2.ID})
	if err != nil {
		return nil, err
	}
	return polarity(x, d, args.String("polarity"))
}

func filterViewOps() []graph.OpDef {
	sigma := func(name, doc string, def any) graph.ArgSpec {
		return graph.ArgSpec{Name: name, Type: "number", Unit: graph.DocPx, Positional: true, Default: def, Min: graph.F(0.1), Max: graph.F(500),
			Usual: [2]float64{0.3, 100}, Doc: doc}
	}
	return []graph.OpDef{
		{
			Name: "highpass", On: graph.KindImage, Returns: graph.KindImage,
			Args:   []graph.ArgSpec{sigma("sigma", "σ of the blur subtracted, doc px: keeps what differs from its surroundings at about this size", nil), polarityArg},
			Halo:   func(a graph.Args) float64 { return 3 * a.DocLength("sigma") },
			Expand: expandHighpass,
			Doc:    "What differs from its surroundings at size σ: a − G_sigma(a), signed (positive where brighter), in the image's space. A derived image: show it with .mul(4).add(0.5), a diverging colormap, or view(\"signed\").",
			See:    []string{"dog", "localContrast", "view", "sub"},
		},
		{
			Name: "dog", On: graph.KindImage, Returns: graph.KindImage,
			Args: []graph.ArgSpec{
				sigma("sigma1", "σ of the first blur, doc px: suppresses the finest grain", nil),
				sigma("sigma2", "σ of the second blur, doc px (> sigma1): the size of the surroundings", nil),
				polarityArg,
			},
			Validate: func(a graph.Args) error {
				s1, s2 := a.DocLength("sigma1"), a.DocLength("sigma2")
				if !(s2 > s1) {
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
			Doc:    "Difference of Gaussians G_sigma1(a) − G_sigma2(a) with sigma1 < sigma2, signed, in the image's space (img.as(\"linear\").dog(…) for light): the first blur suppresses grain, so small specks rank higher than with highpass. dog(1, 4) for 1–3 px specks.",
			See:    []string{"highpass", "view"},
		},
		{
			Name: "localContrast", On: graph.KindImage, Returns: graph.KindImage, Result: graph.DerivedEnc,
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
			Doc: "Local z-scores (a − G_sigma(a)) / (local spread + 0.001), in the image's space: 3 means three local standard deviations above the local mean, in a dark corner and a bright one alike. A derived image.",
			See: []string{"highpass", "view"},
		},
	}
}

// Preset is one named view: a description (help, the views topic) and its
// expansion on image a with target size s (doc px). A view follows its
// receiver's space, except speck and speckColor, which compute in linear light:
// on the colleague's density tiles (linear = density) a labelled speck
// ranks 18th (median) with 79.6 % in the top 50, against 20th and 70.4 % in
// file units (TestRealViewRanking, 142 points on 15 tiles; GCV-003).
type Preset struct {
	Name   string
	Doc    string
	Expand func(x *graph.Expander, a *graph.Node, s float64) (*graph.Node, error)
}

// Presets are the view presets of img.view(name, { size }).
var Presets = []Preset{
	{"scene", "the image stretched 1–99 %: judging what belongs to the scene", func(x *graph.Expander, a *graph.Node, _ float64) (*graph.Node, error) {
		return x.Call("levels", a, map[string]any{"auto": true})
	}},
	{"speck", "small bright things that differ from their surroundings (grey DoG σ size/3 vs 4·size/3 in linear light, white = 5 × p99): dust on density tiles, stars, sensor defects",
		func(x *graph.Expander, a *graph.Node, s float64) (*graph.Node, error) {
			return speckView(x, a, s, "bright", true, true)
		}},
	{"speckDark", "small dark things, shown bright: dust on positives and prints", func(x *graph.Expander, a *graph.Node, s float64) (*graph.Node, error) {
		return speckView(x, a, s, "dark", true, false)
	}},
	{"speckColor", "the speck view on all channels: neutral specks stay white, colour grain is coloured", func(x *graph.Expander, a *graph.Node, s float64) (*graph.Node, error) {
		return speckView(x, a, s, "bright", false, true)
	}},
	{"signed", "brighter (red) and darker (blue) than the surroundings at size s: edges, scratches, both polarities", func(x *graph.Expander, a *graph.Node, s float64) (*graph.Node, error) {
		g, err := x.Call("toGray", a, "average")
		if err != nil {
			return nil, err
		}
		h, err := x.Call("highpass", g, s)
		if err != nil {
			return nil, err
		}
		return x.Call("colormap", h, "diverging")
	}},
	{"detail", "local contrast (σ 4·size) in a diverging colormap of ±4: detail equally visible in shadows and highlights, for sweeping a whole tile",
		func(x *graph.Expander, a *graph.Node, s float64) (*graph.Node, error) {
			g, err := x.Call("toGray", a, "average")
			if err != nil {
				return nil, err
			}
			l, err := x.Call("localContrast", g, 4*s)
			if err != nil {
				return nil, err
			}
			return x.Call("colormap", l, "diverging", map[string]any{"range": []any{-4.0, 4.0}})
		}},
	{"neutral", "where the channels disagree (relative chroma in viridis, 0–1): grey dust versus coloured detail", func(x *graph.Expander, a *graph.Node, _ float64) (*graph.Node, error) {
		c, err := x.Call("chroma", a, map[string]any{"relative": true})
		if err != nil {
			return nil, err
		}
		return x.Call("colormap", c, "viridis", map[string]any{"range": []any{0.0, 1.0}})
	}},
}

// speckView is a bright (or dark) DoG of σ s/3 vs 4s/3, in linear light
// (linear) or in the receiver's space, grey or on all channels, stretched to
// white = 5 × its p99.
func speckView(x *graph.Expander, a *graph.Node, s float64, pol string, gray, linear bool) (*graph.Node, error) {
	src, err := a, error(nil)
	if linear {
		if src, err = x.Call(graph.AsOp, a, "linear"); err != nil {
			return nil, err
		}
	}
	if gray {
		if src, err = x.Call("toGray", src, "average"); err != nil {
			return nil, err
		}
	}
	d, err := x.Call("dog", src, s/3, 4*s/3, map[string]any{"polarity": pol})
	if err != nil {
		return nil, err
	}
	return x.Call("levelsAt", d) // white = max(5 × p99, 1e-4), computed when it runs
}

// levelsAtOp is levels with black 0 and white = max(times × the p-quantile
// of the receiver's luminance, floor), computed when it runs: the speck
// view's stretch. Internal: not callable from JavaScript.
func levelsAtOp() graph.OpDef {
	return graph.OpDef{
		Name: "levelsAt", On: graph.KindImage, Returns: graph.KindImage, Internal: true, Global: true, Luma: true,
		Args: []graph.ArgSpec{
			{Name: "p", Type: "number", Default: 0.99, Min: graph.F(0), Max: graph.F(1)},
			{Name: "times", Type: "number", Default: 5.0},
			{Name: "floor", Type: "number", Default: 1e-4},
		},
		Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
			src := image(in[0])
			lum := append([]float32(nil), a.Luma().Buf.Plane(0)...) // Percentile reorders; the quantile is the one stats reports
			w := max(a.Number("times")*kernels.Percentile(lum, a.Number("p")), a.Number("floor"))
			out, err := kernels.Levels(ctx, src.Buf, 0, w, 1)
			if err != nil {
				return nil, err
			}
			return graph.NewImage(out), nil
		},
		Doc: "levels({ black: 0, white: max(times × p-quantile, floor) }) with the quantile of the receiver's luminance (the speck view's stretch; internal).",
	}
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
		Name: "view", On: graph.KindImage, Returns: graph.KindImage,
		Args: []graph.ArgSpec{
			{Name: "name", Type: "enum:" + strings.Join(presetNames(), ","), Positional: true, ValueDocs: presetDocs(),
				Doc: "the preset: " + strings.Join(presetNames(), ", ") + " (cv.docs.read(\"views\") describes each)"},
			{Name: "size", Type: "number", Unit: graph.DocPx, Default: 3.0, Min: graph.F(0.5), Max: graph.F(200), Usual: [2]float64{1, 30},
				Doc: "typical size of the target in doc px; the presets derive their σ from it"},
		},
		Halo: func(a graph.Args) float64 { return 4 * a.DocLength("size") },
		Expand: func(x *graph.Expander, a *graph.Node, args graph.Args) ([]*graph.Node, error) {
			name := args.String("name")
			for _, p := range Presets {
				if p.Name == name {
					n, err := p.Expand(x, a, args.DocLength("size"))
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
