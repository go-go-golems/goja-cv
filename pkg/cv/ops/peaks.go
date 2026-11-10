package ops

import (
	"context"
	"math"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

// x.peaks({ radius, snr, polarity }) (GCV-005 §6.8, §7.8, D12–D16): the
// local maxima of measure's core/ring step as a Regions node of disks with
// the geometry every region has, the radius, and measure's core/ring fields
// at core r, ring [1.5 r, 2.5 r]. Searched on the receiver's luminance at
// analysis scale 1.

// peakFields is the schema of peaks: geometry, r, then the core/ring
// fields without channel steps.
var peakFields = graph.WithFields(graph.GeometryFields,
	append([]graph.FieldSpec{numField("r", graph.DocPx, "the radius the peak was found at: its core radius (the ring is [1.5 r, 2.5 r])")},
		coreRingFields("off")...)...)

// peakRadii are the radii of a call in doc px (a number or a list).
func peakRadii(a graph.Args) []float64 {
	if l := a.Lengths("radius", 1); l != nil {
		return l
	}
	return []float64{a.Length("radius", 1)}
}

func runPeaks(ctx context.Context, _ *graph.EvalCtx, _ []graph.Value, a graph.Args) (graph.Value, error) {
	L := a.Luma()
	sx, sy := graph.AxisScales(L)
	ox, oy := L.BufferOrigin()
	radii := peakRadii(a)
	for i := range radii {
		radii[i] *= sx
	}
	ps, err := kernels.Peaks(ctx, L.Buf, kernels.PeakOptions{Radii: radii, MinSNR: a.Number("snr"), Polarity: a.String("polarity")})
	if err != nil {
		return nil, err
	}
	rv := graph.NewRegions(L.Buf.W, L.Buf.H, L.Buf.Scale, L.Frame)
	rv.Items = make([]*graph.Region, len(ps))
	for i, p := range ps {
		r := p.R / sx
		c := [2]float64{ox + p.CX/sx, oy + p.CY/sy}
		reg := &graph.Region{
			ID: i + 1, Shape: graph.Disk, R: r, Centroid: c,
			Box: [4]float64{c[0] - r, c[1] - r, 2 * r, 2 * r}, Area: math.Pi * r * r,
			Measures: map[string]float64{"aspect": 1}, Tags: map[string]string{},
		}
		m := p.CoreRing
		reg.Measures["coreMean"], reg.Measures["ringMean"] = m.CoreMean, m.RingMean
		reg.Measures["step"], reg.Measures["spread"], reg.Measures["snr"] = m.Step, m.Spread, m.SNR
		reg.Measures["ringCoverage"] = m.RingCoverage
		rv.Items[i] = reg
	}
	return rv, nil
}

func peaksOp() graph.OpDef {
	return graph.OpDef{
		Name: "peaks", On: graph.KindImage, Returns: graph.KindRegions, Luma: true,
		Args: []graph.ArgSpec{
			{Name: "radius", Type: "number|number[]", Unit: graph.DocPx, Default: 3.0, Min: graph.F(1), Max: graph.F(64), Usual: [2]float64{1.5, 16},
				Doc: "core radius of the spots sought (doc px), or a list of radii to search several sizes ([2, 4, 8]); the ring is [1.5 r, 2.5 r]"},
			{Name: "snr", Type: "number", Default: 1.0, Min: graph.F(0), Usual: [2]float64{0, 5},
				Doc: "smallest |snr| kept: step / the ring's 10–90 % spread, measure's snr (1 keeps faint spots; sort or filter for the strong ones)"},
			{Name: "polarity", Type: "enum:bright,dark,both", Default: "bright", Doc: "maxima of the step (bright spots), minima (dark spots), or both"},
		},
		Validate: func(a graph.Args) error {
			if l, ok := a.Raw("radius").([]any); ok && len(l) == 0 {
				return graph.Errorf(graph.EArgType, "peaks.radius is an empty list.").WithFix("{ radius: [2, 4, 8] }")
			}
			return nil
		},
		AnalysisScale: func(graph.Args) float64 { return 1 },
		Fields:        fixedFields(peakFields),
		Run:           runPeaks,
		Doc: "Spots as data: the local maxima of the core/ring step (measure's step: the mean of a disk of radius r minus the mean of the ring [1.5 r, 2.5 r]), " +
			"with sub-pixel centres, as disks { id, cx, cy, r, step, spread, snr, ringCoverage, … } sorted by |snr| (id = rank). " +
			"snr is measured against each spot's own ring, so a faint speck on a quiet dark background stands out; a peak's numbers are what measure reports there. " +
			"With radius: [2, 4, 8] each spot is found at its own size and overlapping peaks keep the higher |snr| (a saturated blob is one large peak). " +
			"Searched on the luminance at analysis scale 1; on img.crop(b) only the crop is searched, and peaks near its border have cut rings (ringCoverage). " +
			"On a view that clips at zero (the speck view) rings are flat and snr is meaningless: search the image, then measure the view with a prefix. " +
			"About 0.1–0.2 s per megapixel with three radii (a 53-megapixel scan takes seconds): crop a large scan.",
		See: []string{"measure", "sheet", "overlay", "detect.specks", "erode"},
	}
}
