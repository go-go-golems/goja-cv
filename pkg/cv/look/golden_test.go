package look

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/goja-cv/internal/testimage"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// The golden-look harness renders a fixed list of looks of the planted scan
// and compares their 8-bit images (before the look number is burned in and
// before JPEG encoding) with PNGs under testdata/golden.
//
//	go test ./pkg/cv/look -run TestGoldenLooks -update            rewrite the goldens
//	go test ./pkg/cv/look -run TestGoldenLooks -golden.tolerant   apply the one-time tolerance rule
//
// Without flags the comparison is exact. The tolerance rule (GCV-001 guide
// §13.3) is for the one commit that changes the display path: levels and mask
// looks may change; every other look may differ by at most 1 code on at most
// 1 % of its samples; speck overlays must keep the number of specks; charts
// may differ on at most 1 % of their samples (a flipped bin count moves a bar
// edge by a whole pixel).
var (
	updateGolden   = flag.Bool("update", false, "rewrite the golden looks")
	tolerantGolden = flag.Bool("golden.tolerant", false, "apply the GCV-001 §13.3 tolerance rule instead of exact comparison")
)

const goldenDir = "testdata/golden"

// goldenClass says how a look may change under the tolerance rule.
type goldenClass int

const (
	classStrict  goldenClass = iota // ≤ 1 code on ≤ 1 % of samples
	classLevels                     // expected to change (levels remaps in linear space)
	classMask                       // expected to change (masks display their values)
	classSpecks                     // overlay of detect.specks: the speck count must not change
	classChart                      // histogram chart: a bin count flip moves a bar edge by a pixel; ≤ 1 % of samples
	classOverlay                    // overlay tints are composited in encoded space (display colours): tinted pixels change
)

type goldenCase struct {
	name  string
	class goldenClass
	build func(t *testing.T, e *graph.Engine, img graph.NodeID) graph.NodeID
	opts  Options
}

func goldenCases() []goldenCase {
	c := func(name string, raw ...any) func(*testing.T, *graph.Engine, graph.NodeID) graph.NodeID {
		return func(t *testing.T, e *graph.Engine, img graph.NodeID) graph.NodeID {
			return call(t, e, name, img, raw...).ID
		}
	}
	lum := func(t *testing.T, e *graph.Engine, img graph.NodeID) graph.NodeID {
		return call(t, e, "mask.luminance", img, map[string]any{"above": 0.5}).ID
	}
	return []goldenCase{
		{"source", classStrict, func(_ *testing.T, _ *graph.Engine, img graph.NodeID) graph.NodeID { return img }, Options{}},
		{"exposure", classStrict, c("exposure", 0.5), Options{}},
		{"gaussian", classStrict, c("gaussian", 2.0), Options{}},
		{"contrast", classStrict, c("contrast", 0.3), Options{}},
		{"levels-stats", classLevels, func(t *testing.T, e *graph.Engine, img graph.NodeID) graph.NodeID {
			s := call(t, e, "stats", img)
			p01 := call(t, e, "field", s.ID, "p01")
			p99 := call(t, e, "field", s.ID, "p99")
			return call(t, e, "levels", img, map[string]any{"black": graph.NodeArg{ID: p01.ID}, "white": graph.NodeArg{ID: p99.ID}}).ID
		}, Options{}},
		{"levels-gamma", classLevels, c("levels", map[string]any{"black": 0.02, "white": 0.6, "gamma": 1.4}), Options{}},
		{"togray", classStrict, c("toGray"), Options{}},
		{"mask-luminance", classMask, lum, Options{}},
		{"mask-color", classMask, c("mask.color", map[string]any{"hue": 30.0, "tol": 40.0, "minSat": 0.05}), Options{}},
		{"mask-box", classMask, c("mask.box", []any{40.0, 30.0, 100.5, 80.25}), Options{}},
		{"mask-ellipse", classMask, c("mask.ellipse", []any{40.0, 30.0, 100.0, 80.0}), Options{}},
		{"mask-gradient-edge", classMask, c("mask.gradient", map[string]any{"from": "top", "to": 0.6}), Options{}},
		{"mask-gradient-points", classMask, c("mask.gradient", map[string]any{"from": []any{0.0, 0.0}, "to": []any{200.0, 150.0}}), Options{}},
		{"mask-flood", classMask, c("mask.flood", []any{128.0, 96.0}, map[string]any{"tol": 0.08}), Options{}},
		{"mask-feather", classMask, func(t *testing.T, e *graph.Engine, img graph.NodeID) graph.NodeID {
			return call(t, e, "feather", lum(t, e, img), 6.0).ID
		}, Options{}},
		{"mask-refine", classMask, func(t *testing.T, e *graph.Engine, img graph.NodeID) graph.NodeID {
			return call(t, e, "refine", lum(t, e, img), graph.NodeArg{ID: img}).ID
		}, Options{}},
		{"mask-overlay", classOverlay, func(t *testing.T, e *graph.Engine, img graph.NodeID) graph.NodeID {
			return call(t, e, "overlay", lum(t, e, img), graph.NodeArg{ID: img}).ID
		}, Options{}},
		{"specks-overlay", classSpecks, func(t *testing.T, e *graph.Engine, img graph.NodeID) graph.NodeID {
			sp := call(t, e, "detect.specks", img)
			return call(t, e, "overlay", img, graph.NodeArg{ID: sp.ID}, map[string]any{"labels": "id"}).ID
		}, Options{}},
		{"histogram", classChart, c("histogram"), Options{}},
		{"histogram-r", classChart, c("histogram", map[string]any{"channel": "r"}), Options{}},
		{"compare-side", classStrict, compareCase("side"), Options{}},
		{"compare-split", classStrict, compareCase("split"), Options{}},
		{"compare-diff", classStrict, compareCase("diff"), Options{}},
		// GCV-002: a 32 × 32 crop at zoom 8 with rulers every 4 doc px and a grid every 8
		{"ruler-crop-zoom8", classStrict, c("crop", []any{100.0, 40.0, 32.0, 32.0}), Options{Zoom: 8, Ruler: 4, Grid: 8}},
		{"grid-gray-crop-zoom4", classStrict, func(t *testing.T, e *graph.Engine, img graph.NodeID) graph.NodeID {
			return call(t, e, "crop", call(t, e, "toGray", img).ID, []any{180.0, 20.0, 48.0, 40.0}).ID
		}, Options{Zoom: 4, Grid: 4}},
	}
}

func compareCase(mode string) func(*testing.T, *graph.Engine, graph.NodeID) graph.NodeID {
	return func(t *testing.T, e *graph.Engine, img graph.NodeID) graph.NodeID {
		other := call(t, e, "exposure", img, 0.3)
		return call(t, e, "compare", img, []any{graph.NodeArg{ID: other.ID}}, map[string]any{"mode": mode}).ID
	}
}

// writePlanted writes the planted scan into a temporary directory.
func writePlanted(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), testimage.PlantedTIFF)
	if _, err := testimage.WritePlanted(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	return p
}

func encodePNG(t *testing.T, m image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, m); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// pixelDiff compares two images sample by sample (8-bit codes).
type pixelDiff struct {
	maxCode int
	differ  int // differing samples
	samples int
	shape   string // non-empty when the images cannot be compared
}

func samplesOf(m image.Image) (int, int, int, func(x, y int) []int) {
	b := m.Bounds()
	if g, ok := m.(*image.Gray); ok {
		return b.Dx(), b.Dy(), 1, func(x, y int) []int { return []int{int(g.GrayAt(b.Min.X+x, b.Min.Y+y).Y)} }
	}
	return b.Dx(), b.Dy(), 3, func(x, y int) []int {
		r, g, bl, _ := m.At(b.Min.X+x, b.Min.Y+y).RGBA()
		return []int{int(r >> 8), int(g >> 8), int(bl >> 8)}
	}
}

func comparePixels(got, want image.Image) pixelDiff {
	gw, gh, gc, gs := samplesOf(got)
	ww, wh, wc, ws := samplesOf(want)
	if gw != ww || gh != wh || gc != wc {
		return pixelDiff{shape: fmt.Sprintf("got %d×%d×%d, want %d×%d×%d", gw, gh, gc, ww, wh, wc)}
	}
	d := pixelDiff{samples: gw * gh * gc}
	for y := 0; y < gh; y++ {
		for x := 0; x < gw; x++ {
			a, b := gs(x, y), ws(x, y)
			for i := range a {
				if v := abs(a[i] - b[i]); v > 0 {
					d.differ++
					d.maxCode = max(d.maxCode, v)
				}
			}
		}
	}
	return d
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (d pixelDiff) String() string {
	if d.shape != "" {
		return d.shape
	}
	return fmt.Sprintf("largest difference %d codes, %d of %d samples differ (%.3f %%)", d.maxCode, d.differ, d.samples, 100*float64(d.differ)/float64(max(1, d.samples)))
}

func TestGoldenLooks(t *testing.T) {
	ctx := context.Background()
	e, img := newEngine(t, writePlanted(t))
	counts := map[string]int{}
	countsPath := filepath.Join(goldenDir, "counts.json")
	wantCounts := map[string]int{}
	if !*updateGolden {
		raw, err := os.ReadFile(countsPath)
		if err != nil {
			t.Fatalf("read %s: %v (run with -update to create the goldens)", countsPath, err)
		}
		if err := json.Unmarshal(raw, &wantCounts); err != nil {
			t.Fatal(err)
		}
	}
	for _, gc := range goldenCases() {
		t.Run(gc.name, func(t *testing.T) {
			id := gc.build(t, e, img.ID)
			l, err := New(e, id, gc.opts)
			if err != nil {
				t.Fatal(err)
			}
			m, err := l.image(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if gc.class == classSpecks {
				n, ok := e.Node(id)
				if !ok {
					t.Fatal("no overlay node")
				}
				v, err := e.Evaluate(ctx, n.Inputs[1], graph.EvalOptions{Scale: 1})
				if err != nil {
					t.Fatal(err)
				}
				counts[gc.name] = len(v.(*graph.RegionsValue).Items)
			}
			path := filepath.Join(goldenDir, gc.name+".png")
			if *updateGolden {
				if err := os.MkdirAll(goldenDir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, encodePNG(t, m), 0o644); err != nil { // #nosec G306 -- test data
					t.Fatal(err)
				}
				return
			}
			f, err := os.Open(path)
			if err != nil {
				t.Fatalf("open golden: %v (run with -update)", err)
			}
			defer func() { _ = f.Close() }()
			want, err := png.Decode(f)
			if err != nil {
				t.Fatal(err)
			}
			d := comparePixels(m, want)
			if gc.class == classSpecks && counts[gc.name] != wantCounts[gc.name] {
				t.Errorf("speck count %d, golden %d", counts[gc.name], wantCounts[gc.name])
			}
			switch {
			case d.shape != "":
				t.Fatalf("shape changed: %s", d)
			case d.differ == 0:
				return
			case !*tolerantGolden:
				t.Errorf("look differs from its golden: %s", d)
			case gc.class == classLevels || gc.class == classMask || gc.class == classSpecks || gc.class == classOverlay:
				t.Logf("expected change: %s", d)
			case gc.class == classChart && float64(d.differ) <= 0.01*float64(d.samples):
				t.Logf("chart within tolerance: %s", d)
			case d.maxCode <= 1 && float64(d.differ) <= 0.01*float64(d.samples):
				t.Logf("within tolerance: %s", d)
			default:
				t.Errorf("beyond tolerance: %s", d)
			}
		})
	}
	if *updateGolden {
		js, err := json.MarshalIndent(counts, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(countsPath, append(js, '\n'), 0o644); err != nil { // #nosec G306 -- test data
			t.Fatal(err)
		}
	}
}
