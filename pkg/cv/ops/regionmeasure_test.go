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

// TestRegionMeasureStatsEqualStatsPerRegion: stats fields are
// stats({ mask: regions.only([id]).toMask() }) of each region, also for
// regions found at another raster size (Region.Coverage).
func TestRegionMeasureStatsEqualStatsPerRegion(t *testing.T) {
	e, img, _ := plantedEngine(t)
	m := call(t, e, "mask.luminance", img.ID, map[string]any{"above": 0.75})
	for _, as := range []float64{1, 0.5} {
		comps := call(t, e, "top", call(t, e, "components", m.ID, map[string]any{"minArea": 4, "analysisScale": as}).ID, 6)
		meas := call(t, e, "measure", comps.ID, graph.NodeArg{ID: img.ID}, map[string]any{"stats": []any{"mean", "median", "p10", "p90", "min", "max"}, "channels": true})
		rv := regionsAtScale1(t, e, meas)
		if len(rv.Items) == 0 {
			t.Fatal("no regions")
		}
		for _, r := range rv.Items {
			mask := call(t, e, "toMask", call(t, e, "only", comps.ID, float64(r.ID)).ID)
			s := evalData(t, e, call(t, e, "stats", img.ID, map[string]any{"mask": graph.NodeArg{ID: mask.ID}, "percentiles": []any{0, 0.1, 0.9, 1}}), 1).(map[string]any)
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

// TestRegionMeasureRingEqualsMeasureAtCentroid: ring fields are
// img.measure at each centroid with the default core radius; prefixes name
// the fields; ids and order are kept.
func TestRegionMeasureRingEqualsMeasureAtCentroid(t *testing.T) {
	e, img, _ := plantedEngine(t)
	specks := call(t, e, "top", call(t, e, "sortBy", call(t, e, "detect.specks", img.ID).ID, "area").ID, 8)
	meas := call(t, e, "measure", specks.ID, graph.NodeArg{ID: img.ID}, map[string]any{"ring": true, "prefix": "lin"})
	want := []string{"linCoreMean", "linRingMean", "linStep", "linSpread", "linSnr", "linStepR", "linStepG", "linStepB", "linStepChroma", "linRingCoverage"}
	names := graph.FieldNames(meas.Fields)
	if got := strings.Join(names[len(names)-len(want):], ","); got != strings.Join(want, ",") {
		t.Fatalf("new fields %s", got)
	}
	before, after := regionsAtScale1(t, e, specks), regionsAtScale1(t, e, meas)
	for i, r := range after.Items {
		if r.ID != before.Items[i].ID || r.Pix == nil {
			t.Fatalf("item %d is region %d, was %d", i, r.ID, before.Items[i].ID)
		}
		if _, ok := before.Items[i].Measures["linStep"]; ok {
			t.Fatal("the receiver's items were modified")
		}
		core := defaultCore(r.Area)
		rows := evalData(t, e, call(t, e, "measure", img.ID, []any{[]any{r.Centroid[0], r.Centroid[1]}}, map[string]any{"coreRadius": core}), 1).([]any)
		row := rows[0].(map[string]any)
		for _, f := range []string{"step", "spread", "snr", "stepChroma", "ringCoverage"} {
			nearly(t, f, r.Measures[prefixed("lin", f)], row[f].(float64), 1e-12)
		}
	}
	// ranking by a measured field
	ranked := call(t, e, "sortBy", meas.ID, "linSnr")
	rv := regionsAtScale1(t, e, ranked)
	for i := 1; i < len(rv.Items); i++ {
		if rv.Items[i].Measures["linSnr"] > rv.Items[i-1].Measures["linSnr"] {
			t.Fatal("sortBy(linSnr) is not largest first")
		}
	}
}

func TestRegionMeasureErrors(t *testing.T) {
	e, img, _ := plantedEngine(t)
	specks := call(t, e, "detect.specks", img.ID)
	ce := callErr(t, e, "measure", specks.ID, graph.NodeArg{ID: img.ID}, map[string]any{"stats": []any{"mean"}})
	if ce.Code != EFieldExists || !strings.Contains(ce.Message, "meanL") || !strings.Contains(ce.Fix, `prefix: "img"`) {
		t.Fatalf("meanL twice: %+v", ce)
	}
	if ce := callErr(t, e, "measure", specks.ID, graph.NodeArg{ID: img.ID}); ce.Code != graph.EArgType || !strings.Contains(ce.Fix, "ring: true") {
		t.Fatalf("no stats, no ring: %+v", ce)
	}
	if ce := callErr(t, e, "measure", specks.ID, graph.NodeArg{ID: img.ID}, map[string]any{"stats": []any{"maximum"}}); ce.Code != graph.EArgType || ce.Hint != `Did you mean "max"?` {
		t.Fatalf("stats maximum: %+v", ce)
	}
	crop := call(t, e, "crop", img.ID, []any{10.0, 10.0, 100.0, 100.0})
	if ce := callErr(t, e, "measure", specks.ID, graph.NodeArg{ID: crop.ID}, map[string]any{"ring": true}); ce.Code != graph.EFrame {
		t.Fatalf("another frame: %+v", ce)
	}
	if ce := callErr(t, e, "where", specks.ID, map[string]any{"snr": map[string]any{"gte": 2}}); !strings.Contains(ce.Hint, "snr comes from regions.measure(img, { ring: true })") {
		t.Fatalf("snr on specks: %+v", ce)
	}
	// a mask measured as an image: components().measure(mask, { stats: ["mean"] }) replaces meanMask
	m := call(t, e, "mask.luminance", img.ID, map[string]any{"above": 0.75})
	mm := regionsAtScale1(t, e, call(t, e, "measure", call(t, e, "components", m.ID).ID, graph.NodeArg{ID: m.ID}, map[string]any{"stats": []any{"mean"}}))
	if len(mm.Items) == 0 || !(mm.Items[0].Measures["meanL"] > 0.5) {
		t.Fatalf("mean of the mask over its components: %+v", mm.Items)
	}
}
