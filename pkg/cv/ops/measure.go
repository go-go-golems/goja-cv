package ops

import (
	"context"
	"strconv"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

// PercentileKey is the key of percentile p in stats().percentiles.
func PercentileKey(p float64) string { return strconv.FormatFloat(p, 'g', -1, 64) }

var statsFields = []string{"meanL", "medianL", "p01", "p99", "clipLow", "clipHigh", "percentiles", "perChannel", "w", "h", "scale"}

func channelData(c kernels.ChannelStats, ps []float64) map[string]any {
	m := map[string]any{
		"mean": c.Mean, "median": c.Median, "p01": c.P01, "p99": c.P99,
		"clipLow": c.ClipLow, "clipHigh": c.ClipHigh,
	}
	if len(ps) > 0 {
		q := map[string]any{}
		for i, p := range ps {
			q[PercentileKey(p)] = c.Percentiles[i]
		}
		m["percentiles"] = q
	}
	return m
}

func statsData(s *kernels.StatsResult, ps []float64, scale float64) map[string]any {
	q := map[string]any{}
	for i, p := range ps {
		q[PercentileKey(p)] = s.L.Percentiles[i]
	}
	per := map[string]any{}
	if len(s.Channels) == 0 {
		per["gray"] = channelData(s.L, ps)
	}
	for i, c := range s.Channels {
		per[[]string{"r", "g", "b", "a"}[i]] = channelData(c, ps)
	}
	return map[string]any{
		"meanL": s.L.Mean, "medianL": s.L.Median, "p01": s.L.P01, "p99": s.L.P99,
		"clipLow": s.ClipLow, "clipHigh": s.ClipHigh,
		"percentiles": q, "perChannel": per,
		"w": float64(s.W), "h": float64(s.H), "scale": scale,
	}
}

func measureOps() []graph.OpDef {
	return []graph.OpDef{
		{
			Name: "stats", On: graph.KindImage, Returns: graph.KindValue, Space: "analysis", Global: true,
			Args: []graph.ArgSpec{{Name: "percentiles", Type: "number[]", Default: []any{}, Min: graph.F(0), Max: graph.F(1),
				Doc: "extra luminance/channel percentiles as fractions, e.g. [0.005, 0.995]"}},
			Fields: statsFields,
			Run: func(ctx context.Context, ec *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				ps := a.Numbers("percentiles")
				src := image(in[0])
				s, err := kernels.Stats(ctx, src.Buf, ps)
				if err != nil {
					return nil, err
				}
				return data(statsData(s, ps, ec.Scale)), nil
			},
			Doc: "Luminance and per-channel statistics in linear light: meanL, medianL, p01, p99, clipLow, clipHigh (fractions of pixels with any channel clipped), percentiles, perChannel.",
			See: []string{"histogram", "levels", "percentile"},
		},
		{
			Name: "histogram", On: graph.KindImage, Returns: graph.KindValue, Space: "analysis", Global: true,
			Args: []graph.ArgSpec{
				{Name: "bins", Type: "number", Default: 256.0, Min: graph.F(2), Max: graph.F(4096)},
				{Name: "channel", Type: "enum:L,r,g,b", Default: "L"},
			},
			Fields: []string{"bins", "channel", "space", "counts", "under", "over", "total"},
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				h, err := kernels.Histogram(ctx, image(in[0]).Buf, a.Int("bins"), a.String("channel"))
				if err != nil {
					return nil, graph.Errorf(graph.EArgRange, "histogram: %v", err)
				}
				counts := make([]any, len(h.Counts))
				for i, c := range h.Counts {
					counts[i] = float64(c)
				}
				return data(map[string]any{
					"bins": float64(len(h.Counts)), "channel": a.String("channel"), "space": "perceptual",
					"counts": counts, "under": float64(h.Under), "over": float64(h.Over), "total": float64(h.Total),
				}), nil
			},
			Doc: "Histogram of luminance or one channel over equal bins of the perceptual (sRGB) scale [0, 1]; under/over count values outside it.",
			See: []string{"stats"},
		},
	}
}
