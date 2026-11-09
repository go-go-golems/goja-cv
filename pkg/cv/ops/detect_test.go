package ops

import (
	"context"
	"math"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/goja-cv/internal/testimage"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// plantedEngine writes the planted-fault scan as a 16-bit TIFF and opens it.
func plantedEngine(t *testing.T) (*graph.Engine, *graph.Node, testimage.Planted) {
	t.Helper()
	path := filepath.Join(t.TempDir(), testimage.PlantedTIFF)
	p, err := testimage.WritePlanted(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	e := graph.NewEngine(NewRegistry(), graph.EngineOptions{})
	img, err := e.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return e, img, p
}

// matchSpecks matches regions to planted specks by centroid distance (≤ 2.5
// doc px) and returns recall, false positives and whether the star matched.
func matchSpecks(rv *graph.RegionsValue, p testimage.Planted) (found int, fp int, star bool) {
	used := map[int]bool{}
	for _, r := range rv.Items {
		cx, cy := r.Centroid[0], r.Centroid[1]
		if math.Hypot(cx-p.Star.X, cy-p.Star.Y) < 4 {
			star = true
			continue
		}
		best, bi := math.Inf(1), -1
		for i, s := range p.Specks {
			if d := math.Hypot(s.X-cx, s.Y-cy); d < best && !used[i] {
				best, bi = d, i
			}
		}
		if best <= 2.5 {
			used[bi] = true
			found++
		} else {
			fp++
		}
	}
	return found, fp, star
}

func TestDetectSpecksOnPlantedScan(t *testing.T) {
	e, img, p := plantedEngine(t)
	specks := call(t, e, "detect.specks", img.ID)
	keep := call(t, e, "where", specks.ID, map[string]any{"score": map[string]any{"gte": DustScore}})
	all := regionsAt(t, e, specks, 1)
	kept := regionsAt(t, e, keep, 1)
	fAll, fpAll, starAll := matchSpecks(all, p)
	found, fp, star := matchSpecks(kept, p)
	t.Logf("planted %d specks + 1 star; candidates %d (found %d, fp %d, star %v); score ≥ %.1f: %d (found %d, fp %d, star %v)",
		len(p.Specks), len(all.Items), fAll, fpAll, starAll, DustScore, len(kept.Items), found, fp, star)
	if recall := float64(found) / float64(len(p.Specks)); recall < 0.95 {
		t.Fatalf("recall %.2f < 0.95", recall)
	}
	if fp > 2 || star {
		t.Fatalf("false positives %d, star kept %v", fp, star)
	}
	if !starAll {
		t.Log("note: the star was not even a candidate")
	}
	// polarity split matches the planted one
	var dark, bright int
	for _, r := range kept.Items {
		if r.Tags["polarity"] == "dark" {
			dark++
		} else {
			bright++
		}
	}
	if dark != 30 || bright != 10 {
		t.Fatalf("polarity dark %d bright %d", dark, bright)
	}
	// doc-px identity: the same regions at preview scale 0.5 and at 1.0
	half := regionsAt(t, e, keep, 0.5)
	if len(half.Items) != len(kept.Items) {
		t.Fatalf("count differs: %d vs %d", len(half.Items), len(kept.Items))
	}
	for i := range half.Items {
		if half.Items[i].ID != kept.Items[i].ID || half.Items[i].Box != kept.Items[i].Box || half.Items[i].Centroid != kept.Items[i].Centroid {
			t.Fatalf("region %d differs between scales: %+v vs %+v", i, half.Items[i], kept.Items[i])
		}
	}
	if e.Evaluator().Runs(specks.ID) != 1 {
		t.Fatalf("detect.specks ran %d times for two scales", e.Evaluator().Runs(specks.ID))
	}
	// analysing at half resolution still finds most specks, in doc px
	lo := regionsAt(t, e, call(t, e, "where", call(t, e, "detect.specks", img.ID, map[string]any{"analysisScale": 0.5}).ID,
		map[string]any{"score": map[string]any{"gte": DustScore}}), 1)
	fLo, fpLo, starLo := matchSpecks(lo, p)
	t.Logf("analysisScale 0.5: %d regions, found %d, fp %d, star %v", len(lo.Items), fLo, fpLo, starLo)
	if fLo < 30 || fpLo > 3 {
		t.Fatalf("half-resolution detection found %d (fp %d)", fLo, fpLo)
	}
	// check.dust
	cd := evalValue(t, e, call(t, e, "check.dust", img.ID), 0.5).(*graph.DataValue).Data.(map[string]any)
	if cd["verdict"] != "dusty" || cd["count"].(float64) != float64(len(kept.Items)) || cd["maxArea"].(float64) <= 0 {
		t.Fatalf("check.dust %v", cd)
	}
	t.Logf("check.dust: %v", cd)
	clean := evalValue(t, e, call(t, e, "check.dust", call(t, e, "gaussian", img.ID, 6).ID), 1).(*graph.DataValue).Data.(map[string]any)
	if clean["verdict"] != "clean" {
		t.Fatalf("blurred scan: %v", clean)
	}
}
