package ops

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

// Measurement ops (GCV-003 §7.2-§7.4): sample, profile and measure read a
// small window around each point through the probe kernel. They take
// points only (Regions are measured by regions.measure, which keeps their
// ids), work in the receiver's space and read the image at analysis scale
// 1, so their numbers do not depend on a look's scale.

// WOutside is the evaluation-time warning of points outside the frame.
const WOutside = "OUTSIDE"

// MaxProfileSamples bounds a profile.
const MaxProfileSamples = 4096

var grayArg = graph.ArgSpec{Name: "gray", Type: "enum:rec709,average", Default: "rec709",
	Doc: "the gray plane: rec709 (the luminance: Rec. 709 of linear light, in the image's space) or average (the mean of the stored channels)",
	ValueDocs: map[string]string{"rec709": "the luminance: Rec. 709 weights 0.2126, 0.7152, 0.0722 on linear light, expressed in the image's space",
		"average": "(r + g + b) / 3 of the values in the image's space"}}

// pointsArg takes points; a Regions node is accepted by the type only to be
// refused with the fix (validatePoints).
func pointsArg(doc string) graph.ArgSpec {
	return graph.ArgSpec{Name: "points", Type: "Regions|point[]", Unit: graph.DocPx, Positional: true, Doc: doc}
}

// validatePoints refuses Regions given as points, with the Regions op as
// the fix.
func validatePoints(op, fix string) func(graph.Args) error {
	return func(a graph.Args) error {
		if _, ok := a.Raw("points").(graph.InputRef); !ok || a.Input("points").Kind != graph.KindRegions {
			return nil
		}
		return graph.Errorf(graph.EWrongKind, "%s takes points; regions are measured by regions.measure, which keeps their ids.", op).
			WithHint("Pass a list of points ([x, y] or objects with cx, cy, such as (await regions).items), or measure the regions.").
			WithFix(fix)
	}
}

// docPoints reads a resolved point list (doc px).
func docPoints(a graph.Args) ([][2]float64, error) {
	list, ok := a.Raw("points").([]any)
	if !ok {
		return nil, graph.Errorf(graph.EArgType, "points must be a list of points [x, y] (doc px).").WithFix("img.measure([[120.5, 80.5], [33.5, 41.5]])")
	}
	out := make([][2]float64, len(list))
	for i, e := range list {
		p, ok := e.([]any)
		if !ok || len(p) != 2 {
			return nil, graph.Errorf(graph.EArgType, "points[%d] is not a point [x, y].", i)
		}
		x, ok1 := p[0].(float64)
		y, ok2 := p[1].(float64)
		if !ok1 || !ok2 {
			return nil, graph.Errorf(graph.EArgType, "points[%d] is not a point of numbers.", i)
		}
		out[i] = [2]float64{x, y}
	}
	return out, nil
}

// toBuffer converts doc-px points to buffer px of image value v.
func toBuffer(pts [][2]float64, v *graph.ImageValue) [][2]float64 {
	sx, sy := graph.AxisScales(v)
	ox, oy := v.BufferOrigin()
	out := make([][2]float64, len(pts))
	for i, p := range pts {
		out[i] = [2]float64{(p[0] - ox) * sx, (p[1] - oy) * sy}
	}
	return out
}

// warnOutside raises OUTSIDE for points outside the frame.
func warnOutside(ec *graph.EvalCtx, op string, outside []int, n int, f graph.Frame) {
	if len(outside) == 0 {
		return
	}
	idx := make([]string, 0, min(len(outside), 8))
	for _, i := range outside[:min(len(outside), 8)] {
		idx = append(idx, fmt.Sprint(i))
	}
	more := ""
	if len(outside) > 8 {
		more = ", …"
	}
	ec.Warn(WOutside, fmt.Sprintf("%s: %d of %d points lie outside the frame %s (rows %s%s have outside: true and null values).",
		op, len(outside), n, f, strings.Join(idx, ", "), more), "")
}

func channelNames(c int) []string {
	if c == 1 {
		return nil
	}
	return []string{"R", "G", "B"}[:c]
}

// coreDefault is measure's default core radius.
const coreDefault = 3.0

// ringRadii are the ring radii of a measure call (doc px): the option, or
// [core + 3, core + 9].
func ringRadii(a graph.Args, core float64) (float64, float64) {
	if l, ok := a.Raw("ringRadii").([]any); ok && len(l) == 2 {
		r1, _ := l[0].(float64)
		r2, _ := l[1].(float64)
		return r1, r2
	}
	return core + 3, core + 9
}

func checkRing(where string, core, r1, r2 float64) error {
	if r1 > core && r2 > r1 {
		return nil
	}
	return graph.Errorf(graph.EArgRange, "%s: ringRadii [%s, %s] must satisfy coreRadius (%s) < r1 < r2.", where, fmtG(r1), fmtG(r2), fmtG(core)).
		WithHint("The ring is the annulus between r1 and r2 around the point; it must lie outside the core.").
		WithFix(fmt.Sprintf("{ coreRadius: %s, ringRadii: [%s, %s] }", fmtG(core), fmtG(core+3), fmtG(core+9)))
}

func fmtG(f float64) string { return fmt.Sprintf("%g", f) }

// measureMeansFields are the core/ring fields in order (without prefix).
func coreRingFields(channels string) []graph.FieldSpec {
	fs := []graph.FieldSpec{
		numField("coreMean", graph.Level, "coverage-weighted mean of the gray plane in the core disk"),
		numField("ringMean", graph.Level, "the same in the ring"),
		numField("step", graph.Level, "coreMean − ringMean: positive when the spot is brighter than its surroundings"),
		numField("spread", graph.Level, "p90 − p10 of the gray plane in the ring: how grainy the surroundings are"),
		numField("snr", graph.Unitless, "step / max(spread, 1e-3): the step in units of local graininess"),
	}
	if channels != "off" {
		opt := channels == "auto"
		for _, c := range []string{"R", "G", "B"} {
			f := numField("step"+c, graph.Level, "the step of channel "+c)
			f.Optional = opt
			fs = append(fs, f)
		}
		f := numField("stepChroma", graph.Unitless, "(max − min of the channel steps) / |step|: 0 for a neutral spot (dust), larger for coloured ones; null when |snr| < 1 (the step is within the ring's spread, so its colour cannot be attributed)")
		f.Optional = opt
		fs = append(fs, f)
	}
	return append(fs, numField("ringCoverage", graph.Fraction, "fraction of the ring's weight inside the frame (below about 0.7 the ring is cut by the edge)"))
}

// channelsMode is "on", "off" or "auto" (colour images only) from the
// channels option.
func channelsMode(a graph.Args) string {
	b, ok := a.Raw("channels").(bool)
	switch {
	case !ok:
		return "auto"
	case b:
		return "on"
	}
	return "off"
}

var pointFields = []graph.FieldSpec{numField("cx", graph.DocPx, "point x"), numField("cy", graph.DocPx, "point y")}

var outsideField = graph.FieldSpec{Name: "outside", Type: "bool", Optional: true, Doc: "true for a point outside the frame (its values are null)"}

func sampleFields(graph.Args) []graph.FieldSpec {
	fs := graph.WithFields(pointFields, numField("meanL", graph.Level, "mean of the gray plane"))
	for _, c := range []string{"R", "G", "B"} {
		f := numField("mean"+c, graph.Level, "mean of channel "+c+" (colour images)")
		f.Optional = true
		fs = append(fs, f)
	}
	return append(fs, numField("area", graph.DocPx2, "covered weight: the pixel's area for radius 0"), outsideField)
}

func measureFields(a graph.Args) []graph.FieldSpec {
	return append(graph.WithFields(pointFields, coreRingFields(channelsMode(a))...), outsideField)
}

func profileFields(graph.Args) []graph.FieldSpec {
	list := func(n, doc string) graph.FieldSpec { return graph.FieldSpec{Name: n, Type: "number[]", Doc: doc} }
	fs := []graph.FieldSpec{
		{Name: "from", Type: "point", Unit: graph.DocPx, Doc: "start point [x, y] (doc px)"}, {Name: "to", Type: "point", Unit: graph.DocPx, Doc: "end point [x, y] (doc px)"},
		numField("step", graph.DocPx, "distance between samples"), numField("width", graph.DocPx, "samples averaged across the line"),
		list("t", "distance of each sample from the start (doc px)"), list("cx", "sample x"), list("cy", "sample y"),
		list("meanL", "the gray plane at each sample (null outside the frame)"),
	}
	for _, c := range []string{"R", "G", "B"} {
		f := list("mean"+c, "channel "+c+" at each sample (colour images)")
		f.Optional = true
		fs = append(fs, f)
	}
	return fs
}

// nullRow is a row of a point outside the frame: its point, outside and
// null values.
func nullRow(fields []graph.FieldSpec, x, y float64) map[string]any {
	row := map[string]any{}
	for _, f := range fields {
		if !f.Optional {
			row[f.Name] = nil
		}
	}
	row["cx"], row["cy"], row["outside"] = x, y, true
	return row
}

func runSample(ctx context.Context, ec *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
	src := image(in[0])
	pts, err := docPoints(a)
	if err != nil {
		return nil, err
	}
	sx, sy := graph.AxisScales(src)
	res, err := kernels.SamplePoints(ctx, src.Buf, a.Luma().Buf.Plane(0), toBuffer(pts, src), a.Length("radius", sx))
	if err != nil {
		return nil, err
	}
	rows := make([]any, len(res))
	var outside []int
	for i, s := range res {
		if s.Outside {
			outside = append(outside, i)
			rows[i] = nullRow(sampleFields(a), pts[i][0], pts[i][1])
			continue
		}
		row := map[string]any{"cx": pts[i][0], "cy": pts[i][1], "meanL": s.Gray, "area": s.Weight / (sx * sy)}
		for c, name := range channelNames(len(s.Chans)) {
			row["mean"+name] = s.Chans[c]
		}
		rows[i] = row
	}
	warnOutside(ec, "sample", outside, len(pts), src.Frame)
	return data(rows), nil
}

func runMeasure(ctx context.Context, ec *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
	src := image(in[0])
	pts, err := docPoints(a)
	if err != nil {
		return nil, err
	}
	core := a.Length("coreRadius", 1)
	r1, r2 := ringRadii(a, core)
	if err := checkRing("measure", core, r1, r2); err != nil {
		return nil, err
	}
	sx, _ := graph.AxisScales(src)
	res, err := kernels.ProbeCoreRing(ctx, src.Buf, toBuffer(pts, src), kernels.CoreRingOptions{Core: core * sx, R1: r1 * sx, R2: r2 * sx, Gray: a.Luma().Buf.Plane(0)})
	if err != nil {
		return nil, graph.Errorf(graph.EArgRange, "measure: %v", err)
	}
	mode := channelsMode(a)
	fields := measureFields(a)
	rows := make([]any, len(res))
	var outside []int
	for i, r := range res {
		if r.Outside {
			outside = append(outside, i)
			rows[i] = nullRow(fields, pts[i][0], pts[i][1])
			continue
		}
		row := map[string]any{"cx": pts[i][0], "cy": pts[i][1]}
		coreRingRow(row, "", r, mode)
		rows[i] = row
	}
	warnOutside(ec, "measure", outside, len(pts), src.Frame)
	return data(rows), nil
}

// coreRingRow writes the core/ring fields of r into row, each name
// prefixed (regions.measure's prefix: dog → dogStep).
func coreRingRow(row map[string]any, prefix string, r kernels.CoreRing, channels string) {
	name := func(s string) string { return prefixed(prefix, s) }
	row[name("coreMean")], row[name("ringMean")] = r.CoreMean, r.RingMean
	row[name("step")], row[name("spread")], row[name("snr")] = r.Step, r.Spread, r.SNR
	if channels != "off" && r.ChanSteps != nil {
		for c, n := range channelNames(len(r.ChanSteps)) {
			row[name("step"+n)] = r.ChanSteps[c]
		}
		row[name("stepChroma")] = nullable(r.StepChroma)
	}
	row[name("ringCoverage")] = r.RingCoverage
}

// nullable is x, or nil (JavaScript null) for NaN.
func nullable(x float64) any {
	if math.IsNaN(x) {
		return nil
	}
	return x
}

// prefixed is prefix + Name in camelCase ("dog", "maxL" → "dogMaxL").
func prefixed(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + strings.ToUpper(name[:1]) + name[1:]
}

func runProfile(ctx context.Context, ec *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
	src := image(in[0])
	from, to := a.Lengths("from", 1), a.Lengths("to", 1)
	step := a.Length("step", 1)
	width := max(1, int(math.Round(a.Length("width", 1))))
	n, err := profileSamples("profile", from, to, step)
	if err != nil {
		return nil, err
	}
	dx, dy := to[0]-from[0], to[1]-from[1]
	length := math.Hypot(dx, dy)
	ux, uy := 1.0, 0.0
	if length > 0 {
		ux, uy = dx/length, dy/length
	}
	nx, ny := -uy, ux // across the line
	sx, sy := graph.AxisScales(src)
	ox, oy := src.BufferOrigin()
	b := src.Buf
	planes := []([]float32){}
	for c := 0; c < b.C; c++ {
		planes = append(planes, b.Plane(c))
	}
	luma := a.Luma().Buf.Plane(0)
	t, xs, ys, vl := make([]any, n), make([]any, n), make([]any, n), make([]any, n)
	vc := make([][]any, b.C)
	for c := range vc {
		vc[c] = make([]any, n)
	}
	var outside []int
	for k := 0; k < n; k++ {
		if k%256 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		d := float64(k) * step
		x, y := from[0]+ux*d, from[1]+uy*d
		t[k], xs[k], ys[k] = d, x, y
		sums := make([]float64, b.C)
		var gsum float64
		cnt := 0
		for j := 0; j < width; j++ {
			o := float64(j) - float64(width-1)/2
			bx, by := (x+nx*o-ox)*sx, (y+ny*o-oy)*sy
			g, ok := kernels.Bilinear(luma, b.W, b.H, bx, by)
			gsum += g
			for c := 0; c < b.C && ok; c++ {
				var v float64
				v, ok = kernels.Bilinear(planes[c], b.W, b.H, bx, by)
				sums[c] += v
			}
			if ok {
				cnt++
			} else {
				for c := range sums {
					sums[c] = 0
				}
				cnt = 0
				break
			}
		}
		if cnt == 0 {
			outside = append(outside, k)
			vl[k] = nil
			for c := range vc {
				vc[c][k] = nil
			}
			continue
		}
		for c := range sums {
			sums[c] /= float64(cnt)
			vc[c][k] = sums[c]
		}
		vl[k] = gsum / float64(cnt)
	}
	if len(outside) > 0 {
		ec.Warn(WOutside, fmt.Sprintf("profile: %d of %d samples lie outside the frame %s (null).", len(outside), n, src.Frame), "")
	}
	out := map[string]any{
		"from": []any{from[0], from[1]}, "to": []any{to[0], to[1]}, "step": step, "width": float64(width),
		"t": t, "cx": xs, "cy": ys, "meanL": vl,
	}
	for c, name := range channelNames(b.C) {
		out["mean"+name] = vc[c]
	}
	return data(out), nil
}

// profileSamples is the number of samples of a profile: t = 0, step, … ≤
// length, at most MaxProfileSamples.
func profileSamples(where string, from, to []float64, step float64) (int, error) {
	length := math.Hypot(to[0]-from[0], to[1]-from[1])
	n := int(math.Floor(length/step+1e-9)) + 1
	if n > MaxProfileSamples {
		return 0, graph.Errorf(graph.EArgRange, "%s: %d samples along %.1f doc px at step %g; at most %d.", where, n, length, step, MaxProfileSamples).
			WithFix(fmt.Sprintf("{ step: %g }", math.Ceil(length/float64(MaxProfileSamples-1)*100)/100))
	}
	return n, nil
}

func literalPoint(v any) ([]float64, bool) {
	l, ok := v.([]any)
	if !ok || len(l) != 2 {
		return nil, false
	}
	x, ok1 := l[0].(float64)
	y, ok2 := l[1].(float64)
	return []float64{x, y}, ok1 && ok2
}

func measurementOps() []graph.OpDef {
	return []graph.OpDef{
		{
			Name: "sample", On: graph.KindImage, Returns: graph.KindValue, Luma: true,
			Args: []graph.ArgSpec{
				pointsArg("points in absolute doc px: [x, y] lists or objects with cx, cy (region items, measurement rows); the centre of pixel (i, j) is [i + 0.5, j + 0.5]"),
				{Name: "radius", Type: "number", Unit: graph.DocPx, Default: 0.0, Min: graph.F(0), Max: graph.F(500),
					Doc: "0: the pixel that contains the point; > 0: the coverage-weighted mean of a disk (4 × 4 supersampled, as mask.ellipse)"},
				grayArg, analysisScaleArg,
			},
			AnalysisScale: analysisScale,
			Validate:      validatePoints("sample", `regions.measure(img, { stats: ["mean"] })`),
			Fields:        sampleFields,
			Run:           runSample,
			Doc: "The value at points: one row per point { cx, cy, meanL, meanR, meanG, meanB, area } in the image's space (img.as(\"linear\") for light). " +
				"Points outside the frame give a row with outside: true and null values (OUTSIDE warning).",
			See: []string{"measure", "profile", "stats"},
		},
		{
			Name: "measure", On: graph.KindImage, Returns: graph.KindValue, Luma: true,
			Args: []graph.ArgSpec{
				pointsArg("points in absolute doc px: [x, y] lists or objects with cx, cy (region items, measurement rows)"),
				{Name: "coreRadius", Type: "number", Unit: graph.DocPx, Default: coreDefault, Min: graph.F(0.5), Max: graph.F(200),
					Doc: "radius of the core disk (doc px)"},
				{Name: "ringRadii", Type: "number[]", Unit: graph.DocPx, Len: 2, Optional: true, Min: graph.F(0), Max: graph.F(400),
					Doc: "[r1, r2]: the ring between r1 and r2 (doc px); default [coreRadius + 3, coreRadius + 9]"},
				{Name: "channels", Type: "bool", Optional: true, Doc: "add stepR, stepG, stepB and stepChroma (default: on colour images)"},
				grayArg, analysisScaleArg,
			},
			AnalysisScale: analysisScale,
			Validate: func(a graph.Args) error {
				if err := validatePoints("measure", "regions.measure(img, { ring: true })")(a); err != nil {
					return err
				}
				core := a.Raw("coreRadius").(float64)
				r1, r2 := ringRadii(a, core)
				return checkRing("measure", core, r1, r2)
			},
			Fields: measureFields,
			Run:    runMeasure,
			Doc: "How much each point stands out from its surroundings: one row per point { cx, cy, coreMean, ringMean, step, spread, snr, stepR, stepG, stepB, stepChroma, ringCoverage }, " +
				"comparing the core disk (coreRadius) with the ring (ringRadii) on the gray plane, linear by default. step > 0 is brighter; snr = step / ring spread (p90 − p10); " +
				"stepChroma ≈ 0 for a neutral (grey) spot such as dust, null when |snr| < 1 (a step within the ring's spread has no attributable colour). The helper mem.probe(x, y, r) is img.measure([[x + 0.5, y + 0.5]], { coreRadius: r + 0.5, ringRadii: [r + 3.5, r + 9.5], gray: \"average\" }).",
			See: []string{"sample", "profile", "Regions.measure"},
		},
		{
			Name: "profile", On: graph.KindImage, Returns: graph.KindValue, Luma: true, Chart: graph.ChartLines,
			Args: []graph.ArgSpec{
				{Name: "from", Type: "point", Unit: graph.DocPx, Positional: true, Doc: "start point (doc px; [x, y] or an object with cx, cy)"},
				{Name: "to", Type: "point", Unit: graph.DocPx, Positional: true, Doc: "end point"},
				{Name: "step", Type: "number", Unit: graph.DocPx, Default: 1.0, Min: graph.F(0.05), Max: graph.F(1000), Doc: "distance between samples (doc px)"},
				{Name: "width", Type: "number", Unit: graph.DocPx, Default: 1.0, Min: graph.F(1), Max: graph.F(64),
					Doc: "samples averaged across the line, 1 doc px apart (suppresses grain without blurring along the line)"},
				grayArg, analysisScaleArg,
			},
			AnalysisScale: analysisScale,
			Validate: func(a graph.Args) error {
				from, ok1 := literalPoint(a.Raw("from"))
				to, ok2 := literalPoint(a.Raw("to"))
				step, ok3 := a.Raw("step").(float64)
				if !ok1 || !ok2 || !ok3 {
					return nil // regions or items as points: checked when evaluated
				}
				_, err := profileSamples("profile", from, to, step)
				return err
			},
			Fields: profileFields,
			Run:    runProfile,
			Doc: "The values along the line from A to B: { from, to, step, width, t, cx, cy, meanL, meanR, meanG, meanB } with bilinear samples at t = 0, step, … (doc px), " +
				"each averaged over width positions across the line; null outside the frame. look() of a profile is a line chart.",
			See: []string{"sample", "measure"},
		},
	}
}
