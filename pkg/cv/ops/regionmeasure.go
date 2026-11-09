package ops

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
	"github.com/go-go-golems/goja-cv/pkg/cv/parallel"
)

// regions.measure(image, opts) (GCV-003 §7.4): the regions with fields
// measured on another image, by statistics over each region's pixels and/or
// the core/ring measure at its centroid. The items are copies (items are
// shared between values); the node's schema lists the new fields.

// EFieldExists is the error of measuring a field the regions already have.
const EFieldExists = "E_FIELD_EXISTS"

var regionStats = []string{"mean", "max", "min", "std", "median", "p10", "p90"}

var statDocs = map[string]string{
	"mean": "mean", "max": "maximum", "min": "minimum", "std": "standard deviation", "median": "median",
	"p10": "10th percentile", "p90": "90th percentile",
}

// statSynonyms are words agents use for the stats names.
var statSynonyms = map[string]string{"maximum": "max", "peak": "max", "minimum": "min", "average": "mean", "avg": "mean",
	"stddev": "std", "sd": "std", "sigma": "std", "p50": "median", "q10": "p10", "q90": "p90"}

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// statNames are the stats the call requests, in the option's order.
func statNames(a graph.Args) []string {
	if !a.Has("stats") {
		return nil
	}
	return a.Strings("stats")
}

// ringMode is "" (no ring), or "on" with optional fixed radii.
func ringOf(a graph.Args) (bool, []float64) {
	switch x := a.Raw("ring").(type) {
	case bool:
		return x, nil
	case []any:
		out := make([]float64, 0, 2)
		for _, e := range x {
			f, _ := e.(float64)
			out = append(out, f)
		}
		return true, out
	}
	return false, nil
}

// statChannels are the suffixes of the stats fields: L, and R, G, B with
// channels: true.
func statChannels(a graph.Args) []string {
	if b, _ := a.Raw("channels").(bool); b {
		return []string{"L", "R", "G", "B"}
	}
	return []string{"L"}
}

// ringChannels is the channel mode of the ring fields: channel steps on
// colour images unless channels: false.
func ringChannels(a graph.Args) string {
	if b, ok := a.Raw("channels").(bool); ok && !b {
		return "off"
	}
	return "auto"
}

// newRegionFields are the fields a regions.measure call adds, in order.
func newRegionFields(a graph.Args) []graph.FieldSpec {
	prefix, _ := a.Raw("prefix").(string)
	var fs []graph.FieldSpec
	for _, st := range statNames(a) {
		for _, c := range statChannels(a) {
			what := "the gray plane"
			if c != "L" {
				what = "channel " + c
			}
			f := numField(prefixed(prefix, st+c), graph.Level, fmt.Sprintf("%s of %s over the region's pixels", statDocs[st], what))
			if c != "L" {
				f.Optional = true // colour images only
			}
			fs = append(fs, f)
		}
	}
	if on, _ := ringOf(a); on {
		for _, f := range coreRingFields(ringChannels(a)) {
			f.Name = prefixed(prefix, f.Name)
			fs = append(fs, f)
		}
	}
	return fs
}

func regionMeasureFields(a graph.Args) []graph.FieldSpec {
	return graph.WithFields(receiverFields(a), newRegionFields(a)...)
}

// regionMeasureProduces maps the unprefixed fields regions.measure adds to
// a call that adds them.
func regionMeasureProduces() map[string]string {
	out := map[string]string{}
	for _, st := range regionStats {
		out[st+"L"] = fmt.Sprintf(`regions.measure(img, { stats: [%q] })`, st)
		for _, c := range []string{"R", "G", "B"} {
			out[st+c] = fmt.Sprintf(`regions.measure(img, { stats: [%q], channels: true })`, st)
		}
	}
	for _, f := range coreRingFields("on") {
		out[f.Name] = "regions.measure(img, { ring: true })"
	}
	return out
}

func validateRegionMeasure(a graph.Args) error {
	for _, st := range statNames(a) {
		if !contains(regionStats, st) {
			e := graph.Errorf(graph.EArgType, "regions.measure.stats: %q is not one of %s.", st, strings.Join(regionStats, ", ")).WithValid(regionStats)
			sug, ok := statSynonyms[st]
			if !ok {
				sug = graph.Closest(st, regionStats)
			}
			if sug != "" {
				e.Hint, e.Fix = fmt.Sprintf("Did you mean %q?", sug), fmt.Sprintf("{ stats: [%q] }", sug)
			}
			return e
		}
	}
	on, radii := ringOf(a)
	if len(statNames(a)) == 0 && !on {
		return graph.Errorf(graph.EArgType, "regions.measure needs stats, ring or both.").
			WithHint("stats measures each region's own pixels (mean, max, …); ring compares a disk at its centroid with a ring around it (step, spread, snr, stepChroma).").
			WithFix(`regions.measure(img, { ring: true })   // or { stats: ["mean", "max"] }`)
	}
	if p, ok := a.Raw("prefix").(string); ok && !identifier.MatchString(p) {
		return graph.Errorf(graph.EArgType, "regions.measure.prefix %q is not an identifier.", p).WithFix(`{ prefix: "dog" }`)
	}
	if on && radii != nil {
		if len(radii) != 2 || !(radii[1] > radii[0] && radii[0] > 0) {
			return graph.Errorf(graph.EArgRange, "regions.measure.ring = %v must be true or [r1, r2] with 0 < r1 < r2 (doc px).", radii).
				WithFix("{ ring: true }   // [coreRadius + 3, coreRadius + 9]")
		}
		if c, ok := a.Raw("coreRadius").(float64); ok {
			if err := checkRing("regions.measure", c, radii[0], radii[1]); err != nil {
				return err
			}
		}
	}
	if err := graph.SameFrame("regions.measure", a.Receiver().Frame, a.Input("image").Frame); err != nil {
		return err
	}
	have := receiverFields(a)
	for _, f := range newRegionFields(a) {
		if _, ok := graph.FindField(have, f.Name); ok {
			prefix := "img"
			if a.Input("image").Kind == graph.KindMask {
				prefix = "mask"
			}
			return graph.Errorf(EFieldExists, "regions.measure would add %s, which these regions already have.", f.Name).
				WithHint("Name the new fields with a prefix: prefix + Stat + Channel, e.g. dogMaxL.").
				WithFix(fmt.Sprintf("regions.measure(%s, { …, prefix: %q })", prefix, prefix))
		}
	}
	return nil
}

// defaultCore is the core radius of a region: clamp(round(√(area / π)), 2,
// 8) + 0.5 (the helper's mem.hot rule, as a disk around the centroid).
func defaultCore(area float64) float64 {
	return math.Max(2, math.Min(8, math.Round(math.Sqrt(area/math.Pi)))) + 0.5
}

func runRegionMeasure(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
	rv := regions(in[0])
	src := image(a.Value("image"))
	if err := graph.SameFrame("regions.measure", rv.Frame, src.Frame); err != nil {
		return nil, err
	}
	prefix, _ := a.Raw("prefix").(string)
	gray := grayOf(a)
	b := src.Buf
	sx, sy := graph.AxisScales(src)
	ox, oy := src.BufferOrigin()
	stats := statNames(a)
	chans := statChannels(a)
	var planes [][]float32 // per stats channel: gray, then r, g, b
	if len(stats) > 0 {
		planes = append(planes, kernels.GrayPlane(b, gray))
		if len(chans) > 1 && b.C > 1 {
			for c := 0; c < b.C; c++ {
				planes = append(planes, b.Plane(c))
			}
		}
	}
	ring, radii := ringOf(a)
	rmode := ringChannels(a)
	raster := graph.RasterOf(src)
	sameRaster := rv.W == b.W && rv.H == b.H && raster.OX == float64(rv.Frame.X) && raster.OY == float64(rv.Frame.Y)
	items := make([]*graph.Region, len(rv.Items))
	errs := make([]error, len(rv.Items))
	err := parallel.Rows(ctx, len(rv.Items), func(k0, k1 int) {
		for k := k0; k < k1; k++ {
			r := rv.Items[k]
			c := *r
			c.Measures = make(map[string]float64, len(r.Measures)+8)
			for key, v := range r.Measures {
				c.Measures[key] = v
			}
			if len(stats) > 0 {
				idx, w := regionWeights(rv, r, raster, sameRaster)
				for pi, plane := range planes {
					suffix := "L"
					if pi > 0 {
						suffix = []string{"R", "G", "B"}[pi-1]
					}
					s, err := kernels.StatsOf(ctx, plane, idx, w, b.Scale)
					if err != nil {
						errs[k] = graph.Errorf(graph.EArgRange, "regions.measure: region %d has no pixels in the image.", r.ID)
						break
					}
					vals := map[string]float64{"mean": s.Mean, "max": s.Max, "min": s.Min, "std": s.Std, "median": s.Median, "p10": s.P10, "p90": s.P90}
					for _, st := range stats {
						c.Measures[prefixed(prefix, st+suffix)] = vals[st]
					}
				}
			}
			if ring {
				core := defaultCore(r.Area)
				if v, ok := a.Raw("coreRadius").(float64); ok {
					core = v
				}
				r1, r2 := core+3, core+9
				if radii != nil {
					r1, r2 = radii[0], radii[1]
				}
				if err := checkRing("regions.measure", core, r1, r2); err != nil {
					errs[k] = err
					continue
				}
				x, y := (r.Centroid[0]-ox)*sx, (r.Centroid[1]-oy)*sy
				p := kernels.ProbePoint(ctx, b, x, y, kernels.CoreRingOptions{Core: core * sx, R1: r1 * sx, R2: r2 * sx, Gray: gray})
				row := map[string]any{}
				coreRingRow(row, prefix, p, rmode)
				for key, v := range row {
					c.Measures[key] = v.(float64)
				}
			}
			items[k] = &c
		}
	})
	if err != nil {
		return nil, err
	}
	for _, e := range errs {
		if e != nil {
			return nil, e
		}
	}
	return rv.With(items), nil
}

// regionWeights are the pixel indices of the image buffer and weights of
// region r: its own pixels with weight 1 when the regions were found on the
// image's raster, else its coverage on that raster.
func regionWeights(rv *graph.RegionsValue, r *graph.Region, t graph.Raster, same bool) ([]int, []float32) {
	if same {
		idx := make([]int, len(r.Pix))
		w := make([]float32, len(r.Pix))
		for i, p := range r.Pix {
			idx[i], w[i] = int(p), 1
		}
		return idx, w
	}
	rect, cov := rv.Coverage(r, t)
	var idx []int
	var w []float32
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if a := cov[(y-rect.Min.Y)*rect.Dx()+(x-rect.Min.X)]; a > 0 {
				idx = append(idx, y*t.W+x)
				w = append(w, a)
			}
		}
	}
	return idx, w
}

func regionMeasureOp() graph.OpDef {
	return graph.OpDef{
		Name: "measure", On: graph.KindRegions, Returns: graph.KindRegions,
		Space: graph.SpaceRule{Default: buffer.SpaceLinear},
		Args: []graph.ArgSpec{
			{Name: "image", Type: "Image|Mask", Positional: true, Doc: "the image (or mask) to measure, in the regions' frame"},
			{Name: "stats", Type: "string[]", Optional: true,
				Doc: "statistics over each region's own pixels, on the gray plane (fields meanL, maxL, …): " + strings.Join(regionStats, ", ")},
			{Name: "channels", Type: "bool", Optional: true,
				Doc: "true: the stats per channel too (meanR, maxG, …); false: no channel steps in the ring fields"},
			{Name: "ring", Type: "bool|number[]", Unit: graph.DocPx, Optional: true,
				Doc: "true or [r1, r2] (doc px): the core/ring measure at each centroid (coreMean, ringMean, step, spread, snr, stepR/G/B, stepChroma, ringCoverage)",
				Redirects: map[string]graph.Redirect{
					"ringRadii": {Hint: "regions.measure takes the ring radii as ring: [r1, r2] (img.measure calls them ringRadii).", Fix: "regions.measure(img, { ring: [6, 12], coreRadius: 3 })"},
					"radius":    {Hint: "The core radius is coreRadius; the ring is ring: [r1, r2].", Fix: "regions.measure(img, { ring: true, coreRadius: 3 })"},
				}},
			{Name: "coreRadius", Type: "number", Unit: graph.DocPx, Optional: true, Min: graph.F(0.5), Max: graph.F(200),
				Doc: "core radius for ring (doc px); default per region clamp(round(√(area / π)), 2, 8) + 0.5"},
			{Name: "prefix", Type: "string", Optional: true, Doc: "prefix of the new field names: dog → dogMaxL, dogStep"},
			grayArg, analysisScaleArg,
		},
		AnalysisScale: analysisScale,
		Validate:      validateRegionMeasure,
		Fields:        regionMeasureFields,
		Produces:      regionMeasureProduces(),
		Run:           runRegionMeasure,
		Doc: "The regions with fields measured on an image (or mask) of the same frame: { stats: [\"mean\", \"max\"] } over each region's pixels (meanL, maxL; with channels: true also R, G, B), " +
			"{ ring: true } the core/ring measure at each centroid (step, spread, snr, stepChroma, …), names prefixed with prefix. Ids and order are kept: rank with sortBy(\"snr\").",
		See: []string{"Image.measure", "sortBy", "components"},
	}
}
