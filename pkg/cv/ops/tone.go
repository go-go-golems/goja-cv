package ops

import (
	"context"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

// exposureHint is exposure's doc and RANGE hint (GCV-004 D3): most calls
// that want a large exposure want a brighter picture.
const exposureHint = "For a brighter picture to look at, use levels({ auto: true }); for a gain on stored values, mul(2 ** ev)."

func toneOps() []graph.OpDef {
	return []graph.OpDef{
		{
			Name: "exposure", On: graph.KindImage, Returns: graph.KindImage, Space: graph.Physical,
			Args: []graph.ArgSpec{{Name: "ev", Type: "number", Positional: true, Min: graph.F(-10), Max: graph.F(10),
				Usual: [2]float64{-3, 3}, UsualHint: exposureHint, Doc: "stops of light: linear values are multiplied by 2^ev"}},
			Identity: func(a graph.Args) bool { return a.Number("ev") == 0 },
			Halo:     func(graph.Args) float64 { return 0 },
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
				out, err := kernels.Exposure(ctx, src.Buf, a.Number("ev"))
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out), nil
			},
			Doc: "Change exposure by ev stops of light: computed in linear light (values × 2^ev), returned in the image's space. " + exposureHint,
			See: []string{"levels", "contrast", "stats"},
		},
		{
			Name: "levels", On: graph.KindImage, Returns: graph.KindImage, Luma: true,
			LumaWhen: func(a graph.Args) bool { _, auto := autoPercentiles(a); return auto },
			Args: []graph.ArgSpec{
				{Name: "black", Type: "number", Unit: graph.Level, Optional: true, Min: graph.F(-1), Max: graph.F(16), Usual: [2]float64{0, 0.5},
					Doc: "input level mapped to 0, in the image's space (the scale of its stats percentiles); default 0"},
				{Name: "white", Type: "number", Unit: graph.Level, Optional: true, Min: graph.F(0), Max: graph.F(64), Usual: [2]float64{0, 4},
					Doc: "input level mapped to 1, in the image's space; default 1"},
				{Name: "gamma", Type: "number", Default: 1.0, Min: graph.F(0.01), Max: graph.F(10), Usual: [2]float64{0.2, 5},
					Doc: "midtone gamma applied to the remapped ramp (> 1 brightens)"},
				{Name: "auto", Type: "bool|number[]", Optional: true,
					Doc: "stretch by the image's own percentiles: true = [0.01, 0.99] (p01 → 0, p99 → 1), or [lo, hi] fractions; after a crop, the crop's",
					Redirects: map[string]graph.Redirect{
						"box": {Hint: "levels has no box: crop the window, then stretch it by its own pixels.", Fix: "img.crop([x, y, w, h]).levels({ auto: true })"},
					}},
			},
			Validate: func(a graph.Args) error {
				if a.Has("auto") && (a.Has("black") || a.Has("white")) {
					return graph.Errorf(graph.EArgType, "levels takes auto or black/white, not both: auto computes black and white.").
						WithFix("img.levels({ auto: true })   // or img.levels({ black: 0.02, white: 0.6 })")
				}
				if p, ok := autoPercentiles(a); ok && !(p[0] >= 0 && p[0] < p[1] && p[1] <= 1) {
					return graph.Errorf(graph.EArgRange, "levels auto [%g, %g] must be fractions with lo < hi in [0, 1].", p[0], p[1]).
						WithFix("img.levels({ auto: [0.01, 0.99] })")
				}
				return nil
			},
			Identity: func(a graph.Args) bool {
				if _, auto := autoPercentiles(a); auto {
					return false
				}
				return levelOr(a, "black", 0) == 0 && levelOr(a, "white", 1) == 1 && a.Number("gamma") == 1
			},
			Halo: func(graph.Args) float64 { return 0 },
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
				b, w := levelOr(a, "black", 0), levelOr(a, "white", 1)
				if p, auto := autoPercentiles(a); auto {
					lum := append([]float32(nil), a.Luma().Buf.Plane(0)...) // the one luminance; Percentile reorders
					b = kernels.Percentile(lum, p[0])
					w = kernels.Percentile(lum, p[1])
					if !(w > b) {
						w = b + 1e-6 // a flat window: everything maps to 0
					}
				}
				if !(w > b) {
					return nil, graph.Errorf(graph.EArgRange, "levels: white (%g) must be above black (%g).", w, b).WithFix("img.levels({ black: s.p01, white: s.p99 })   // with s = img.stats()")
				}
				out, err := kernels.Levels(ctx, src.Buf, b, w, a.Number("gamma"))
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out), nil
			},
			Doc: "Map the black and white input levels to 0 and 1 and bend the midtones with gamma, in the image's space (file units for an opened file). levels({ auto: true }) takes black and white from the image's own p01 and p99 (of its luminance), so after a crop it stretches the crop.",
			See: []string{"stats", "contrast", "crop", "view"},
		},
		{
			Name: "contrast", On: graph.KindImage, Returns: graph.KindImage,
			Args: []graph.ArgSpec{{Name: "amount", Type: "number", Positional: true, Min: graph.F(-1), Max: graph.F(1),
				Usual: [2]float64{-0.5, 0.5}, Doc: "scale the distance from mid-grey (0.5) by 1 + amount"}},
			Identity: func(a graph.Args) bool { return a.Number("amount") == 0 },
			Halo:     func(graph.Args) float64 { return 0 },
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
				out, err := kernels.Contrast(ctx, src.Buf, a.Number("amount"))
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out), nil
			},
			Doc: "Increase (amount > 0) or decrease contrast around mid-grey 0.5 of the image's space (the displayed mid-grey for an opened file).",
			See: []string{"levels", "exposure"},
		},
		{
			Name: "toGray", On: graph.KindImage, Returns: graph.KindImage,
			SpaceOf: func(a graph.Args) graph.SpaceRule { // the luminance is defined on linear light; the average follows
				if a.String("weights") == "rec709" {
					return graph.Physical
				}
				return graph.Follows
			},
			Args: []graph.ArgSpec{{Name: "weights", Type: "enum:rec709,average", Default: "rec709", Positional: true}},
			Halo: func(graph.Args) float64 { return 0 },
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
				if src.Buf.C == 1 {
					return src, nil
				}
				out, err := kernels.ToGray(ctx, src.Buf, kernels.GrayWeights[a.String("weights")])
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out), nil
			},
			Doc: "Convert to a single-channel image of weighted channels: rec709 (Rec. 709 luminance of linear light, returned in the image's space: the one luminance of stats, mask.luminance and the measurements) or average (the plain mean, in the image's space).",
			See: []string{"stats"},
		},
	}
}

// levelOr reads an optional level argument with its default.
func levelOr(a graph.Args, name string, def float64) float64 {
	if !a.Has(name) {
		return def
	}
	return a.Number(name)
}

// autoPercentiles reads levels' auto option: [0.01, 0.99] for true, the
// list as given, or ok = false when absent or false.
func autoPercentiles(a graph.Args) ([2]float64, bool) {
	switch v := a.Raw("auto").(type) {
	case bool:
		return [2]float64{0.01, 0.99}, v
	case []any:
		if len(v) == 2 {
			lo, _ := v[0].(float64)
			hi, _ := v[1].(float64)
			return [2]float64{lo, hi}, true
		}
	}
	return [2]float64{}, false
}
