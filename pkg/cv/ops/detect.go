package ops

import (
	"context"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

func speckArgs(maxArea, minContrast float64) []graph.ArgSpec {
	return []graph.ArgSpec{
		{Name: "maxArea", Type: "number", Unit: graph.DocPx2, Default: maxArea, Min: graph.F(1), Max: graph.F(1e6), Usual: [2]float64{4, 5000},
			Doc: "largest speck in doc px²"},
		{Name: "minContrast", Type: "number", Default: minContrast, Min: graph.F(0.001), Max: graph.F(1), Usual: [2]float64{0.01, 0.3},
			Doc: "smallest peak contrast against the local background, in levels of the working space (encoded by default)"},
		{Name: "polarity", Type: "enum:both,dark,bright", Default: "both", Doc: "dark specks (dust on a positive), bright ones, or both"},
		analysisScaleArg,
	}
}

func runSpecks(ctx context.Context, ec *graph.EvalCtx, src *graph.ImageValue, a graph.Args) (*graph.RegionsValue, *kernels.SpeckResult, error) {
	res, err := kernels.DetectSpecks(ctx, src.Buf, kernels.SpeckOptions{
		MaxArea: a.Area("maxArea", ec.Scale), MinContrast: a.Number("minContrast"), Polarity: a.String("polarity"),
	})
	if err != nil {
		return nil, nil, err
	}
	rv := graph.NewRegions(src.Buf.W, src.Buf.H, ec.Scale, src.Frame)
	for i, s := range res.Specks {
		r := rv.NewRegion(i+1, s.Pix)
		r.Measures["contrast"] = s.Contrast
		r.Measures["localNoise"] = s.LocalNoise
		r.Measures["sharpness"] = s.Sharpness
		r.Measures["score"] = s.Score
		r.Measures["meanL"] = s.MeanL
		if s.Dark {
			r.Tags["polarity"] = "dark"
		} else {
			r.Tags["polarity"] = "bright"
		}
		rv.Items = append(rv.Items, r)
	}
	sortByArea(rv.Items)
	return rv, res, nil
}

// DustScore is the score from which check.dust counts a speck as dust.
const DustScore = 0.5

func detectOps() []graph.OpDef {
	return []graph.OpDef{
		{
			Name: "detect.specks", On: graph.KindImage, Returns: graph.KindRegions, Global: true,
			Args:          speckArgs(200, 0.03),
			AnalysisScale: analysisScale,
			Run: func(ctx context.Context, ec *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				rv, _, err := runSpecks(ctx, ec, a.Luma(), a)
				if err != nil {
					return nil, err
				}
				return rv, nil
			},
			Luma: true,
			Doc: "Find small dark/bright specks (dust): the luminance in the image's space (file units for an opened file) minus a morphological background, " +
				"thresholded at max(minContrast, 4σ noise) with hysteresis, labelled, at most maxArea. " +
				"Each region has contrast, localNoise, sharpness (soft round blobs like stars are low) and score in [0, 1]; " +
				"keep real dust with .where({ score: { gte: 0.5 } }). Regions are analysed at analysisScale (default 1), so ids and boxes are the same in a look and a measurement.",
			Fields:   fixedFields(speckFields),
			Produces: speckProduces,
			See:      []string{"heal", "check.dust", "where", "overlay"},
		},
		{
			Name: "check.dust", On: graph.KindImage, Returns: graph.KindValue, Global: true,
			Args:          speckArgs(200, 0.03),
			AnalysisScale: analysisScale,
			Fields:        fixedFields(dustFields),
			Luma:          true,
			Run: func(ctx context.Context, ec *graph.EvalCtx, _ []graph.Value, a graph.Args) (graph.Value, error) {
				src := a.Luma()
				rv, res, err := runSpecks(ctx, ec, src, a)
				if err != nil {
					return nil, err
				}
				var count int
				var maxArea, area float64
				for _, r := range rv.Items {
					if r.Measures["score"] < DustScore {
						continue
					}
					count++
					maxArea = max(maxArea, r.Area)
					area += r.Area
				}
				verdict := "clean"
				switch {
				case count > 10:
					verdict = "dusty"
				case count > 0:
					verdict = "light"
				}
				out := map[string]any{
					"verdict": verdict, "count": float64(count), "candidates": float64(len(rv.Items)),
					"maxArea": maxArea, "coverage": area / float64(src.Frame.W*src.Frame.H), "noise": res.Noise,
				}
				if count > 0 {
					out["hint"] = "img.heal(img.detect.specks().where({ score: { gte: 0.5 } }), { radius: 3 })"
				}
				return data(out), nil
			},
			Doc: `Is there dust? Runs detect.specks and counts specks with score ≥ 0.5: { verdict: "clean" | "light" (1–10) | "dusty" (> 10), count, candidates, maxArea (doc px²), coverage, noise }.`,
			See: []string{"detect.specks", "heal"},
		},
	}
}
