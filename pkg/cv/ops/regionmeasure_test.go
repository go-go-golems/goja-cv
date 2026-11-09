package ops

import (
	"math"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

func regionsAtScale1(t *testing.T, e *graph.Engine, n *graph.Node) *graph.RegionsValue {
	t.Helper()
	v, err := e.Evaluate(t.Context(), n.ID, graph.EvalOptions{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	return v.(*graph.RegionsValue)
}

func nearly(t *testing.T, what string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.12g, want %.12g", what, got, want)
	}
}

// TestMeasureStatsEqualStatsPerItem: stats fields are stats({ mask:
// img.mask.regions([item]) }) of each item, also for regions found at
// another raster size (Region.Coverage).
func TestMeasureStatsEqualStatsPerItem(t *testing.T) {
	e, img, _ := plantedEngine(t)
	m := call(t, e, "mask.luminance", img.ID, map[string]any{"above": 0.75})
	for _, as := range []float64{1, 0.5} {
		comps := call(t, e, "components", m.ID, map[string]any{"minArea": 4, "analysisScale": as})
		n := min(6, len(regionsAtScale1(t, e, comps).Items))
		idx := make([]int, n)
		for i := range idx {
			idx[i] = i
		}
		items := linked(t, e, comps, idx...)
		meas := call(t, e, "measure", img.ID, items, map[string]any{"stats": []any{"mean", "median", "p10", "p90", "min", "max"}, "channels": true})
		rv := regionsAtScale1(t, e, meas)
		if len(rv.Items) == 0 {
			t.Fatal("no regions")
		}
		for k, r := range rv.Items {
			mask := call(t, e, "mask.regions", img.ID, items[k:k+1])
			s := evalData(t, e, call(t, e, "stats", img.ID, map[string]any{"mask": nodeArg(mask), "percentiles": []any{0, 0.1, 0.9, 1}}), 1).(map[string]any)
			q := s["percentiles"].(map[string]any)
			tag := func(f string) string {
				return "analysisScale " + PercentileKey(as) + " region " + PercentileKey(float64(r.ID)) + " " + f
			}
			nearly(t, tag("meanL"), r.Measures["meanL"], s["meanL"].(float64), 1e-9)
			nearly(t, tag("medianL"), r.Measures["medianL"], s["medianL"].(float64), 1e-9)
			nearly(t, tag("p10L"), r.Measures["p10L"], q["0.1"].(float64), 1e-9)
			nearly(t, tag("p90L"), r.Measures["p90L"], q["0.9"].(float64), 1e-9)
			nearly(t, tag("minL"), r.Measures["minL"], q["0"].(float64), 1e-9)
			nearly(t, tag("maxL"), r.Measures["maxL"], q["1"].(float64), 1e-9)
			per := s["perChannel"].(map[string]any)
			nearly(t, tag("meanG"), r.Measures["meanG"], per["g"].(map[string]any)["mean"].(float64), 1e-9)
		}
	}
}

// TestMeasureRadiiFromItems: the ring is measured unless stats is given;
// the core radius comes from coreRadius, a circle's r or a region's
// √(area / π), the ring is [1.5 r, 2.5 r] unless ringRadii; a region, a
// circle and a point with the same radii give the same numbers; prefixes
// name the fields; ids and order are kept.
func TestMeasureRadiiFromItems(t *testing.T) {
	e, img, _ := plantedEngine(t)
	specks := call(t, e, "detect.specks", img.ID)
	n := min(8, len(regionsAtScale1(t, e, specks).Items))
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	items := linked(t, e, specks, idx...)
	meas := call(t, e, "measure", img.ID, items, map[string]any{"prefix": "lin"})
	want := []string{"linCoreMean", "linRingMean", "linStep", "linSpread", "linSnr", "linStepR", "linStepG", "linStepB", "linStepChroma", "linRingCoverage"}
	names := graph.FieldNames(meas.Fields)
	if got := strings.Join(names[len(names)-len(want):], ","); got != strings.Join(want, ",") {
		t.Fatalf("new fields %s", got)
	}
	before, after := regionsAtScale1(t, e, specks), regionsAtScale1(t, e, meas)
	same := func(what string, a, b *graph.Region, pa, pb string) {
		for _, f := range []string{"step", "spread", "snr", "ringCoverage", "stepChroma"} {
			va, _ := a.Field(prefixed(pa, f))
			vb, _ := b.Field(prefixed(pb, f))
			if (va == nil) != (vb == nil) {
				t.Fatalf("%s %s: %v vs %v", what, f, va, vb)
			}
			if va != nil {
				nearly(t, what+" "+f, va.(float64), vb.(float64), 1e-12)
			}
		}
	}
	for i, r := range after.Items {
		if r.ID != before.Items[i].ID || r.Pix == nil {
			t.Fatalf("item %d is region %d, was %d", i, r.ID, before.Items[i].ID)
		}
		if _, ok := before.Items[i].Measures["linStep"]; ok {
			t.Fatal("the detector's items were modified")
		}
		core := math.Max(1, math.Sqrt(r.Area/math.Pi))
		cx, cy := r.Centroid[0], r.Centroid[1]
		explicit := regionsAtScale1(t, e, call(t, e, "measure", img.ID, items[i:i+1], map[string]any{"coreRadius": core, "ringRadii": []any{1.5 * core, 2.5 * core}})).Items[0]
		same("explicit radii", r, explicit, "lin", "")
		disk := regionsAtScale1(t, e, call(t, e, "measure", img.ID, []any{map[string]any{"cx": cx, "cy": cy, "r": core}})).Items[0]
		same("circle", r, disk, "lin", "")
		point := regionsAtScale1(t, e, call(t, e, "measure", img.ID, []any{[]any{cx, cy}}, map[string]any{"coreRadius": core})).Items[0]
		same("point", r, point, "lin", "")
		// the GCV-003 helper radii, given explicitly
		old := regionsAtScale1(t, e, call(t, e, "measure", img.ID, items[i:i+1], map[string]any{"coreRadius": 3.5, "ringRadii": []any{6.5, 12.5}})).Items[0]
		oldPt := regionsAtScale1(t, e, call(t, e, "measure", img.ID, []any{map[string]any{"cx": cx, "cy": cy}}, map[string]any{"coreRadius": 3.5, "ringRadii": []any{6.5, 12.5}})).Items[0]
		same("old radii", old, oldPt, "", "")
	}
	// stats on a measured set adds stats and leaves the ring fields alone
	both := call(t, e, "measure", img.ID, nodeArg(meas), map[string]any{"stats": []any{"mean"}, "prefix": "s"})
	if f := graph.FieldNames(both.Fields); f[len(f)-1] != "sMeanL" || !strings.Contains(strings.Join(f, ","), "linSnr") {
		t.Fatalf("stats on a measured set: %v", f)
	}
	// a point without a radius
	if ce := callErr(t, e, "measure", img.ID, []any{[]any{40.0, 52.0}}); ce.Code != graph.EArgType || ce.Fix != "img.measure(points, { coreRadius: 3 })" {
		t.Fatalf("point without radius: %+v", ce)
	}
}

func TestMeasureErrors(t *testing.T) {
	e, img, _ := plantedEngine(t)
	specks := call(t, e, "detect.specks", img.ID)
	ce := callErr(t, e, "measure", img.ID, nodeArg(specks), map[string]any{"stats": []any{"mean"}})
	if ce.Code != EFieldExists || !strings.Contains(ce.Message, "meanL") || !strings.Contains(ce.Fix, `prefix: "dog"`) {
		t.Fatalf("meanL twice: %+v", ce)
	}
	if ce := callErr(t, e, "measure", img.ID, nodeArg(specks), map[string]any{"ring": false}); ce.Code != graph.EArgType || !strings.Contains(ce.Message, "measures nothing") {
		t.Fatalf("no stats, no ring: %+v", ce)
	}
	if ce := callErr(t, e, "measure", img.ID, nodeArg(specks), map[string]any{"stats": []any{"maxx"}, "prefix": "p"}); ce.Code != graph.EArgType || ce.Hint != `Did you mean "max"?` {
		t.Fatalf("stats maxx: %+v", ce)
	}
	if ce := callErr(t, e, "measure", img.ID, nodeArg(specks), map[string]any{"coreRadius": 4.0, "ringRadii": []any{3.0, 10.0}}); ce.Code != graph.EArgRange {
		t.Fatalf("ring inside the core: %+v", ce)
	}
	// regions found on a crop do not cover the whole image
	crop := call(t, e, "crop", img.ID, []any{10.0, 10.0, 100.0, 100.0})
	onCrop := call(t, e, "detect.specks", crop.ID)
	if _, err := e.Evaluate(t.Context(), call(t, e, "measure", img.ID, nodeArg(onCrop)).ID, graph.EvalOptions{Scale: 1}); err == nil {
		t.Fatal("regions of a crop measured on the whole image")
	} else if ce, _ := graph.AsError(err); ce.Code != graph.EFrame {
		t.Fatalf("another frame: %v", err)
	}
	// a mask measured as an image: the mean of the mask over its components
	m := call(t, e, "mask.luminance", img.ID, map[string]any{"above": 0.75})
	mm := regionsAtScale1(t, e, call(t, e, "measure", m.ID, nodeArg(call(t, e, "components", m.ID)), map[string]any{"stats": []any{"mean"}}))
	if len(mm.Items) == 0 || !(mm.Items[0].Measures["meanL"] > 0.5) {
		t.Fatalf("mean of the mask over its components: %+v", mm.Items)
	}
}

// TestStepChromaIsNullBelowSNR1: on a flat patch the step is 0, so
// stepChroma is null (the field present, its value null); on the planted
// specks it is null exactly where |snr| < 1 and a number elsewhere.
func TestStepChromaIsNullBelowSNR1(t *testing.T) {
	e := testEngine(t)
	flat := testSource(t, e, "srgb", graph.Frame{W: 64, H: 48}, 0.3, 0.3, 0.3)
	meas := call(t, e, "measure", flat.ID, []any{map[string]any{"x": 20.0, "y": 14.0, "w": 8.0, "h": 8.0}})
	rv := regionsAtScale1(t, e, meas)
	if v, ok := rv.Items[0].Field("stepChroma"); !ok || v != nil {
		t.Fatalf("flat patch: stepChroma %v (present %v), want null", v, ok)
	}
	rec := rv.Items[0].Data(meas.Fields)
	if v, ok := rec.Vals["stepChroma"]; !ok || v != nil {
		t.Fatalf("item: stepChroma %v (present %v), want null", v, ok)
	}
	if _, ok := rec.Vals["snr"].(float64); !ok {
		t.Fatalf("snr %v", rec.Vals["snr"])
	}

	pe, img, _ := plantedEngine(t)
	specks := call(t, pe, "measure", img.ID, nodeArg(call(t, pe, "detect.specks", img.ID)))
	nulls, numbers := 0, 0
	for _, r := range regionsAtScale1(t, pe, specks).Items {
		v, _ := r.Field("stepChroma")
		if (v == nil) != (math.Abs(r.Measures["snr"]) < 1) {
			t.Errorf("region %d: snr %.3f, stepChroma %v", r.ID, r.Measures["snr"], v)
		}
		if v == nil {
			nulls++
		} else {
			numbers++
		}
	}
	if numbers == 0 {
		t.Fatal("no stepChroma above |snr| 1")
	}
	t.Logf("planted specks: %d null, %d numbers", nulls, numbers)
}
