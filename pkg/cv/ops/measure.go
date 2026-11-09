package ops

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

// PercentileKey is the key of percentile p in stats().percentiles.
func PercentileKey(p float64) string { return strconv.FormatFloat(p, 'g', -1, 64) }

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

// runStats measures the whole image, or only the pixels of a mask, regions
// and/or a box (weighted by the mask).
func runStats(ctx context.Context, ec *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
	ps := a.Numbers("percentiles")
	src := image(in[0])
	var w []float32
	if v := a.Node("mask"); v != nil {
		m, err := selectionWeights(ctx, v, src)
		if err != nil {
			return nil, err
		}
		w = m
	}
	if a.Has("box") {
		b := a.XY("box", src)
		bm := kernels.BoxMask(src.Buf.W, src.Buf.H, src.Buf.Scale, b[0], b[1], b[2], b[3]).Plane(0)
		if w == nil {
			w = bm
		} else {
			for i := range w {
				w[i] *= bm[i]
			}
		}
	}
	if w == nil {
		s, err := kernels.Stats(ctx, src.Buf, ps)
		if err != nil {
			return nil, err
		}
		return data(statsData(s, ps, ec.Scale)), nil
	}
	ws, err := kernels.StatsWeighted(ctx, src.Buf, w, ps)
	if err == kernels.ErrEmptySelection {
		f := src.Frame
		return nil, graph.Errorf(graph.EArgRange, "stats: the mask/box selects no pixels of this image (frame %s).", f).
			WithHint("Boxes are [x, y, w, h] in doc px, absolute in the image's frame; check a mask with await mask.coverage().").
			WithFix(fmt.Sprintf("img.stats({ box: [%d, %d, %d, %d] })", f.X, f.Y, max(1, f.W/2), max(1, f.H/2)))
	}
	if err != nil {
		return nil, err
	}
	d := statsData(ws.StatsResult, ps, ec.Scale)
	d["area"] = ws.Weight / (src.Buf.Scale * src.Buf.Scale) // doc px²
	d["coverage"] = ws.Weight / float64(src.Buf.N())
	return data(d), nil
}

// selectionWeights is a mask (soft weights) or rasterized regions at the
// image's evaluation size.
func selectionWeights(ctx context.Context, v graph.Value, src *graph.ImageValue) ([]float32, error) {
	var m *buffer.Buffer
	switch x := v.(type) {
	case *graph.RegionsValue:
		if err := graph.SameFrame("stats", x.Frame, src.Frame); err != nil {
			return nil, err
		}
		var err error
		if m, err = x.Rasterize(ctx, graph.RasterOf(src)); err != nil {
			return nil, err
		}
	case *graph.ImageValue:
		if err := sameFrame("stats", src, x); err != nil {
			return nil, err
		}
		m = x.Buf
	default:
		return nil, graph.Errorf(graph.EWrongKind, "stats.mask needs a Mask or Regions.")
	}
	out := make([]float32, m.N())
	copy(out, m.Plane(0))
	return out, nil
}

func measureOps() []graph.OpDef {
	return []graph.OpDef{
		{
			Name: "stats", On: graph.KindImage, Returns: graph.KindValue, Space: graph.SpaceRule{Default: buffer.SpaceLinear}, Global: true,
			Args: []graph.ArgSpec{
				{Name: "percentiles", Type: "number[]", Default: []any{}, Min: graph.F(0), Max: graph.F(1),
					Doc: "extra luminance/channel percentiles as fractions, e.g. [0.005, 0.995]"},
				{Name: "mask", Type: "Regions|Mask", Optional: true, Doc: "measure only inside this mask (weighted by it) or these regions"},
				{Name: "box", Type: "box", Unit: graph.DocPx, Optional: true, InFrame: true, Doc: "measure only inside [x, y, w, h] (doc px, absolute in the image's frame); combines with mask"},
			},
			Fields: statsFields,
			Run:    runStats,
			Doc:    "Luminance and per-channel statistics in the working space (linear by default): meanL, medianL, p01, p99, clipLow, clipHigh (fractions of pixels with any channel clipped), percentiles, perChannel. With { mask } or { box: [x, y, w, h] } only those pixels count (adds area in doc px² and coverage). In encoded space meanL is the Rec. 709 weighted sum of encoded channels.",
			See:    []string{"histogram", "levels", "percentile"},
		},
		{
			Name: "histogram", On: graph.KindImage, Returns: graph.KindValue, Space: graph.SpaceRule{Default: buffer.SpaceEncoded}, Global: true, Chart: graph.ChartBars,
			Args: []graph.ArgSpec{
				{Name: "bins", Type: "number", Default: 256.0, Min: graph.F(2), Max: graph.F(4096)},
				{Name: "channel", Type: "enum:L,r,g,b", Default: "L"},
			},
			Fields: fixedFields(histogramFields),
			Gray: func(a graph.Args) []int {
				if a.String("channel") == "L" {
					return []int{0}
				}
				return nil
			},
			Run: func(ctx context.Context, ec *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				h, err := kernels.Histogram(ctx, image(in[0]).Buf, a.Int("bins"), a.String("channel"))
				if err != nil {
					return nil, graph.Errorf(graph.EArgRange, "histogram: %v", err)
				}
				counts := make([]any, len(h.Counts))
				for i, c := range h.Counts {
					counts[i] = float64(c)
				}
				return data(map[string]any{
					"bins": float64(len(h.Counts)), "channel": a.String("channel"), "space": ec.Node.Space.String(),
					"counts": counts, "under": float64(h.Under), "over": float64(h.Over), "total": float64(h.Total),
				}), nil
			},
			Doc: "Histogram of luminance (Rec. 709 of linear light) or one channel over equal bins of [0, 1] in the working space (encoded by default: the scale looks show); under/over count values outside it.",
			See: []string{"stats"},
		},
	}
}
