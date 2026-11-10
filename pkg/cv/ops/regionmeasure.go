package ops

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
	"github.com/go-go-golems/goja-cv/pkg/cv/parallel"
)

// img.measure(items, opts) (GCV-004 §7.6, D7, D17): the one measuring op.
// Its result is a Regions node: the input items with fields measured on the
// receiver, by the core/ring step at each item's centre (unless stats is
// given; ring: true adds it to stats) and/or statistics over each item's
// area. The core radius comes from each item (coreRadius, else its r, else
// √(area / π) of a region); the ring is [1.5 r, 2.5 r] unless ringRadii.
// The items are copies (items are shared between values); the node's schema
// lists the new fields.

// EFieldExists is the error of measuring a field the items already have.
const EFieldExists = "E_FIELD_EXISTS"

var regionStats = []string{"mean", "max", "min", "std", "median", "p10", "p90"}

var statDocs = map[string]string{
	"mean": "mean", "max": "maximum", "min": "minimum", "std": "standard deviation", "median": "median",
	"p10": "10th percentile", "p90": "90th percentile",
}

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// statNames are the stats the call requests, in the option's order.
func statNames(a graph.Args) []string {
	if !a.Has("stats") {
		return nil
	}
	return a.Strings("stats")
}

// ringOn reports whether the call measures the core/ring step: ring, else
// true unless stats is given.
func ringOn(a graph.Args) bool {
	if a.Has("ring") {
		return a.Bool("ring")
	}
	return len(statNames(a)) == 0
}

// receiverChannels is the receiver's channel count (0 in help, where there
// is no call).
func receiverChannels(a graph.Args) int {
	if !a.HasInputs() {
		return 0
	}
	return a.Receiver().Channels
}

// statChannels are the suffixes of the stats fields: L, and R, G, B with
// channels: true on a colour image.
func statChannels(a graph.Args) []string {
	if b, _ := a.Raw("channels").(bool); b && receiverChannels(a) != 1 {
		return []string{"L", "R", "G", "B"}
	}
	return []string{"L"}
}

// ringChannels is the channel mode of the ring fields: the channel steps on
// a colour image unless channels: false; "auto" when the receiver is not
// known (help).
func ringChannels(a graph.Args) string {
	if b, ok := a.Raw("channels").(bool); ok && !b {
		return "off"
	}
	switch receiverChannels(a) {
	case 0:
		return "auto"
	case 1:
		return "off"
	}
	return "on"
}

// itemsFieldsOf is the schema of the items argument (geometry in help).
func itemsFieldsOf(a graph.Args) []graph.FieldSpec {
	if !a.HasInputs() {
		return graph.GeometryFields
	}
	return a.Input("items").Fields
}

// newMeasureFields are the fields a measure call adds, in order.
func newMeasureFields(a graph.Args) []graph.FieldSpec {
	prefix, _ := a.Raw("prefix").(string)
	var fs []graph.FieldSpec
	for _, st := range statNames(a) {
		for _, c := range statChannels(a) {
			what := "the gray plane"
			if c != "L" {
				what = "channel " + c
			}
			fs = append(fs, numField(prefixed(prefix, st+c), graph.Level, fmt.Sprintf("%s of %s over the item's area", statDocs[st], what)))
		}
	}
	if ringOn(a) {
		for _, f := range coreRingFields(ringChannels(a)) {
			f.Name = prefixed(prefix, f.Name)
			fs = append(fs, f)
		}
	}
	return fs
}

func measureFields(a graph.Args) []graph.FieldSpec {
	return graph.WithFields(itemsFieldsOf(a), newMeasureFields(a)...)
}

func validateMeasure(a graph.Args) error {
	for _, st := range statNames(a) {
		if !contains(regionStats, st) {
			e := graph.Errorf(graph.EArgType, "measure.stats: %q is not one of %s.", st, strings.Join(regionStats, ", ")).WithValid(regionStats)
			if sug := graph.Closest(st, regionStats); sug != "" {
				e.Hint, e.Fix = fmt.Sprintf("Did you mean %q?", sug), fmt.Sprintf("{ stats: [%q] }", sug)
			}
			return e
		}
	}
	if len(statNames(a)) == 0 && !ringOn(a) {
		return graph.Errorf(graph.EArgType, "measure with ring: false and no stats measures nothing.").
			WithHint("The core/ring step is measured unless stats is given; stats measures each item's own area (mean, max, …).").
			WithFix(`img.measure(items)   // or img.measure(items, { stats: ["mean", "max"] })`)
	}
	if p, ok := a.Raw("prefix").(string); ok && !identifier.MatchString(p) {
		return graph.Errorf(graph.EArgType, "measure.prefix %q is not an identifier.", p).WithFix(`{ prefix: "dog" }`)
	}
	if ringOn(a) && a.Has("coreRadius") && a.Has("ringRadii") {
		l := a.Raw("ringRadii").([]any)
		if err := checkRing("measure", a.Raw("coreRadius").(float64), l[0].(float64), l[1].(float64)); err != nil {
			return err
		}
	}
	items := a.Input("items")
	if ringOn(a) && !a.Has("coreRadius") && graph.HasPointItems(items) {
		return graph.Errorf(graph.EArgType, "measure: a point [x, y] or { cx, cy } has no radius to size its core and ring.").
			WithHint("Regions size the core from their area and circles { cx, cy, r } from r; a bare point needs coreRadius (the ring is then [1.5, 2.5] × coreRadius unless ringRadii).").
			WithFix("img.measure(points, { coreRadius: 3 })")
	}
	have := items.Fields
	for _, f := range newMeasureFields(a) {
		if _, ok := graph.FindField(have, f.Name); ok {
			return graph.Errorf(EFieldExists, "measure would add %s, which these items already have.", f.Name).
				WithHint("Name the new fields with a prefix: prefix + Stat + Channel, e.g. dogMaxL, dogSnr.").
				WithFix(`img.measure(items, { …, prefix: "dog" })`)
		}
	}
	return nil
}

// coreRadius is the core radius of item r (doc px): the option, its r (a
// circle), else √(area / π) of a region, at least 1.
func coreRadius(a graph.Args, r *graph.Region) float64 {
	if a.Has("coreRadius") {
		return a.Raw("coreRadius").(float64)
	}
	if r.Shape == graph.Disk {
		return r.R
	}
	return math.Max(1, math.Sqrt(r.Area/math.Pi))
}

// ringOf is the ring of a core radius: ringRadii, else [1.5 r, 2.5 r].
func ringOf(a graph.Args, core float64) (float64, float64) {
	if l, ok := a.Raw("ringRadii").([]any); ok && len(l) == 2 {
		r1, _ := l[0].(float64)
		r2, _ := l[1].(float64)
		return r1, r2
	}
	return 1.5 * core, 2.5 * core
}

func runMeasure(ctx context.Context, ec *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
	src := image(in[0])
	rv := regions(a.Value("items"))
	if err := itemsCover("measure", rv, src.Frame); err != nil {
		return nil, err
	}
	prefix, _ := a.Raw("prefix").(string)
	b := src.Buf
	gray := a.Luma().Buf.Plane(0) // the image's luminance in its space (a grey image: itself)
	sx, sy := graph.AxisScales(src)
	ox, oy := src.BufferOrigin()
	stats := statNames(a)
	var planes [][]float32 // per stats channel: gray, then r, g, b
	if len(stats) > 0 {
		planes = append(planes, gray)
		if len(statChannels(a)) > 1 && b.C > 1 {
			for c := 0; c < b.C; c++ {
				planes = append(planes, b.Plane(c))
			}
		}
	}
	ring := ringOn(a)
	rmode := ringChannels(a)
	raster := graph.RasterOf(src)
	sameRaster := rv.W == b.W && rv.H == b.H && raster.OX == float64(rv.Frame.X) && raster.OY == float64(rv.Frame.Y)
	items := make([]*graph.Region, len(rv.Items))
	errs := make([]error, len(rv.Items))
	outside := make([]bool, len(rv.Items))
	err := parallel.Rows(ctx, len(rv.Items), func(k0, k1 int) {
		for k := k0; k < k1; k++ {
			r := rv.Items[k]
			c := *r
			c.Measures = make(map[string]float64, len(r.Measures)+8)
			for key, v := range r.Measures {
				c.Measures[key] = v
			}
			if len(stats) > 0 {
				idx, w := regionWeights(rv, r, raster, sameRaster && r.Shape == graph.Pixels)
				for pi, plane := range planes {
					suffix := "L"
					if pi > 0 && pi <= 3 { // planes: gray, then R, G, B
						suffix = string("RGB"[pi-1])
					}
					s, err := kernels.StatsOf(ctx, plane, idx, w, b.Scale)
					if err != nil {
						errs[k] = graph.Errorf(graph.EArgRange, "measure: item %d (id %d) has no area in the image; stats need an area.", k, r.ID).
							WithHint("Points have no area: measure them with the ring, or pass circles { cx, cy, r }.")
						break
					}
					vals := map[string]float64{"mean": s.Mean, "max": s.Max, "min": s.Min, "std": s.Std, "median": s.Median, "p10": s.P10, "p90": s.P90}
					for _, st := range stats {
						c.Measures[prefixed(prefix, st+suffix)] = vals[st]
					}
				}
			}
			if ring {
				core := coreRadius(a, r)
				r1, r2 := ringOf(a, core)
				if err := checkRing("measure", core, r1, r2); err != nil {
					errs[k] = err
					continue
				}
				x, y := (r.Centroid[0]-ox)*sx, (r.Centroid[1]-oy)*sy
				p := kernels.ProbePoint(ctx, b, x, y, kernels.CoreRingOptions{Core: core * sx, R1: r1 * sx, R2: r2 * sx, Gray: gray})
				row := map[string]any{}
				if p.Outside {
					outside[k] = true
					for _, f := range coreRingFields(rmode) {
						row[prefixed(prefix, f.Name)] = nil
					}
				} else {
					coreRingRow(row, prefix, p, rmode)
				}
				for key, v := range row {
					f, ok := v.(float64)
					if !ok { // null (stepChroma below |snr| 1, an item outside): kept as NaN, read back as null
						f = math.NaN()
					}
					c.Measures[key] = f
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
	var out []int
	for k, o := range outside {
		if o {
			out = append(out, k)
		}
	}
	if len(out) > 0 {
		ec.Warn(WOutside, fmt.Sprintf("measure: %d of %d items lie outside the frame %s (items %s: their ring fields are null).",
			len(out), len(rv.Items), src.Frame, graph.IndexRanges(out)), "")
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

func measureOp() graph.OpDef {
	return graph.OpDef{
		Name: "measure", On: graph.KindImage, Returns: graph.KindRegions, Luma: true,
		Args: []graph.ArgSpec{
			itemsArg("items", graph.ItemsType+"|point[]", "what to measure: a Regions node or an array of items (region items, boxes { x, y, w, h }, circles { cx, cy, r }, points [x, y] or { cx, cy }), absolute doc px"),
			{Name: "ring", Type: "bool", Optional: true,
				Doc: "the core/ring step at each item's centre (coreMean, ringMean, step, spread, snr, stepR/G/B, stepChroma (null when |snr| < 1), ringCoverage); default: on unless stats is given"},
			{Name: "stats", Type: "string[]", Optional: true,
				Doc: "statistics over each item's area, on the gray plane (fields meanL, maxL, …): " + strings.Join(regionStats, ", ")},
			{Name: "coreRadius", Type: "number", Unit: graph.DocPx, Optional: true, Min: graph.F(0.5), Max: graph.F(200),
				Doc: "core radius (doc px) for every item; default per item: its r (a circle), else √(area / π) of a region; points need it"},
			{Name: "ringRadii", Type: "number[]", Unit: graph.DocPx, Len: 2, Optional: true, Min: graph.F(0), Max: graph.F(400),
				Doc: "[r1, r2]: the ring between r1 and r2 (doc px); default [1.5, 2.5] × the core radius"},
			{Name: "channels", Type: "bool", Optional: true,
				Doc: "true: the stats per channel too (meanR, maxG, …); false: no channel steps in the ring fields (default: on colour images)"},
			{Name: "prefix", Type: "string", Optional: true, Doc: "prefix of the new field names: dog → dogMaxL, dogStep"},
			grayArg, analysisScaleArg,
		},
		AnalysisScale: analysisScale,
		DocFrame: func(a graph.Args) graph.Frame { // the raster of the items, else the image's
			if f := a.Input("items").Frame; !f.Empty() {
				return f
			}
			return a.Receiver().Frame
		},
		Validate: validateMeasure,
		Fields:   measureFields,
		Run:      runMeasure,
		Doc: "How much each item stands out from its surroundings, measured in the image's space (img.as(\"linear\") for light): the items with new fields, a Regions node (awaited: an array). " +
			"By default the core/ring step at each centre: core radius from the item (its r, or √(area / π) of a region; coreRadius for points), ring [1.5 r, 2.5 r]; " +
			"step > 0 is brighter; snr = step / ring spread (p90 − p10); stepChroma ≈ 0 for a neutral (grey) spot such as dust, null when |snr| < 1. " +
			"{ stats: [\"mean\", \"max\"] } measures each item's area instead (add ring: true for both); names prefixed with prefix. Ids, labels and order are kept.",
		See: []string{"sample", "profile", "detect.specks"},
	}
}
