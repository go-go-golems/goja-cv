package ops

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

func evalRegions(t *testing.T, e *graph.Engine, n *graph.Node, scale float64) []*graph.Region {
	t.Helper()
	return evalValue(t, e, n, scale).(*graph.RegionsValue).Items
}

// TestPeaksOp (GCV-005 §6.8): peaks of the planted scan are disks with ids
// 1…n by |snr|, find the planted specks of their polarity, report what
// measure reports at the same centre and radius, equal the peaks of the
// receiver's toGray, do not depend on the evaluation scale, and on a crop
// lie inside it in doc px.
func TestPeaksOp(t *testing.T) {
	e, img, planted := plantedEngine(t)
	opts := map[string]any{"radius": []any{2.0, 4.0}, "snr": 1.0, "polarity": "both"}
	pk := call(t, e, "peaks", img.ID, opts)
	items := evalRegions(t, e, pk, 1)
	if len(items) == 0 {
		t.Fatal("no peaks")
	}
	for i, r := range items {
		if r.ID != i+1 || r.Shape != graph.Disk || (r.R != 2 && r.R != 4) || r.Measures["aspect"] != 1 {
			t.Fatalf("item %d: id %d shape %v r %g", i, r.ID, r.Shape, r.R)
		}
		if i > 0 && math.Abs(r.Measures["snr"]) > math.Abs(items[i-1].Measures["snr"]) {
			t.Fatalf("item %d: |snr| %g above the previous %g", i, r.Measures["snr"], items[i-1].Measures["snr"])
		}
		if math.Abs(r.Measures["snr"]) < 1 || r.Area != math.Pi*r.R*r.R || r.Box != [4]float64{r.Centroid[0] - r.R, r.Centroid[1] - r.R, 2 * r.R, 2 * r.R} {
			t.Fatalf("item %d: %+v", i, r)
		}
	}
	// every compact planted speck (not the elongated ones) has a peak of its polarity within 1.5 px
	found, compact := 0, 0
	for _, s := range planted.Specks {
		if s.RX != s.RY {
			continue
		}
		compact++
		for _, r := range items {
			if math.Hypot(r.Centroid[0]-s.X, r.Centroid[1]-s.Y) <= 1.5 && (r.Measures["step"] < 0) == s.Dark {
				found++
				break
			}
		}
	}
	t.Logf("%d peaks; %d of %d compact planted specks found within 1.5 px", len(items), found, compact)
	if found < compact-1 {
		t.Errorf("peaks found %d of %d compact planted specks", found, compact)
	}

	// a peak's numbers are measure's at the same centre and radius
	var circles []any
	for _, r := range items {
		circles = append(circles, map[string]any{"cx": r.Centroid[0], "cy": r.Centroid[1], "r": r.R})
	}
	measured := evalRegions(t, e, call(t, e, "measure", img.ID, circles), 1)
	for i, m := range measured {
		for _, f := range []string{"coreMean", "ringMean", "step", "spread", "snr", "ringCoverage"} {
			if d := math.Abs(m.Measures[f] - items[i].Measures[f]); d > 1e-9 {
				t.Fatalf("peak %d: %s %g, measure %g", i+1, f, items[i].Measures[f], m.Measures[f])
			}
		}
	}

	// the luminance of a colour image is its toGray; the scale does not matter
	grey := evalRegions(t, e, call(t, e, "peaks", call(t, e, "toGray", img.ID).ID, opts), 1)
	small := evalRegions(t, e, pk, 0.25)
	if fmt.Sprint(regionRows(grey)) != fmt.Sprint(regionRows(items)) || fmt.Sprint(regionRows(small)) != fmt.Sprint(regionRows(items)) {
		t.Error("peaks of toGray() or at scale 0.25 differ from peaks of the image at scale 1")
	}

	// on a crop: inside the crop, in doc px; a peak far from the crop's border is a peak of the whole image
	// (to rounding: the row prefix sums and the coverage weights start at the crop's corner)
	box := []any{60.0, 40.0, 120.0, 100.0}
	cropped := evalRegions(t, e, call(t, e, "peaks", call(t, e, "crop", img.ID, box).ID, opts), 1)
	if len(cropped) == 0 {
		t.Fatal("no peaks in the crop")
	}
	for _, r := range cropped {
		x, y := r.Centroid[0], r.Centroid[1]
		if x < 60 || y < 40 || x > 180 || y > 140 {
			t.Fatalf("crop peak at (%g, %g) outside [60, 40, 120, 100]", x, y)
		}
		if x > 80 && x < 160 && y > 60 && y < 120 {
			same := false
			for _, q := range items {
				if math.Hypot(q.Centroid[0]-x, q.Centroid[1]-y) < 1e-4 && math.Abs(q.Measures["snr"]-r.Measures["snr"]) < 1e-4 {
					same = true
				}
			}
			if !same {
				t.Errorf("crop peak at (%g, %g), far from the border, is not a peak of the image", x, y)
			}
		}
	}
}

// regionRows are the centres, radii and measures of regions, for comparison.
func regionRows(items []*graph.Region) []string {
	out := make([]string, len(items))
	for i, r := range items {
		out[i] = fmt.Sprintf("%d %.12g %.12g %g %.12g %.12g", r.ID, r.Centroid[0], r.Centroid[1], r.R, r.Measures["step"], r.Measures["snr"])
	}
	return out
}

// TestPeaksArgsAndConsumers: the radius and snr limits, measure without a
// prefix, and every consumer of Disk regions (overlay, sheet, mask.regions,
// measure with a prefix, heal, crop).
func TestPeaksArgsAndConsumers(t *testing.T) {
	e, img, _ := plantedEngine(t)
	for _, raw := range []map[string]any{{"radius": 0.5}, {"radius": 65.0}, {"radius": []any{2.0, 80.0}}, {"snr": -1.0}, {"radius": []any{}}, {"polarity": "up"}} {
		if _, err := e.Call("peaks", img.ID, []any{raw}); err == nil {
			t.Errorf("peaks(%v) was accepted", raw)
		}
	}
	cr, err := e.Call("peaks", img.ID, []any{map[string]any{"snr": 0.0, "radius": 3.0}})
	if err != nil || len(cr.Warnings) != 0 {
		t.Fatalf("peaks({ snr: 0 }): %v %+v", err, cr.Warnings)
	}
	pk := call(t, e, "peaks", img.ID, map[string]any{"radius": []any{2.0, 4.0}, "polarity": "both"})
	items := evalRegions(t, e, pk, 1)

	_, err = e.Call("measure", img.ID, []any{graph.NodeArg{ID: pk.ID}})
	if ce, ok := graph.AsError(err); !ok || ce.Code != EFieldExists || !strings.Contains(ce.Fix, "prefix") {
		t.Fatalf("measure(peaks) without a prefix: %+v", err)
	}
	sp := evalRegions(t, e, call(t, e, "measure", call(t, e, "view", img.ID, "speck").ID, graph.NodeArg{ID: pk.ID}, map[string]any{"prefix": "sp"}), 1)
	if len(sp) != len(items) {
		t.Fatalf("measure with prefix: %d items, want %d", len(sp), len(items))
	}
	for i, r := range sp {
		if _, ok := r.Measures["spStep"]; !ok || r.Measures["snr"] != items[i].Measures["snr"] || r.R != items[i].R {
			t.Fatalf("measured peak %d: %+v", i, r)
		}
	}
	// mask.regions of disks covers about Σ π r² (the disks do not overlap)
	var area float64
	for _, r := range items {
		area += r.Area
	}
	cov := evalData(t, e, call(t, e, "coverage", call(t, e, "mask.regions", img.ID, graph.NodeArg{ID: pk.ID}).ID), 1).(float64)
	if got := cov * 256 * 192; math.Abs(got-area) > 0.05*area {
		t.Errorf("mask.regions(peaks) covers %.0f px², the disks %.0f", got, area)
	}
	for _, c := range []struct {
		op  string
		raw []any
	}{
		{"overlay", []any{graph.NodeArg{ID: pk.ID}, map[string]any{"labels": "id"}}},
		{"heal", []any{graph.NodeArg{ID: pk.ID}}},
	} {
		evalValue(t, e, call(t, e, c.op, img.ID, c.raw...), 1)
	}
	sheet := call(t, e, "sheet", 0, graph.NodeArg{ID: pk.ID}, map[string]any{"views": []any{graph.NodeArg{ID: img.ID}}, "max": 6.0})
	evalValue(t, e, sheet, 1)
	if s := graph.Short(sheet); !strings.Contains(s, "cells in region order") {
		t.Errorf("sheet of peaks: caption %q", s)
	}
}
