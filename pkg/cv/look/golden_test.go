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
//	go test ./pkg/cv/look -run TestGoldenLooks -update   rewrite the goldens
//
// The comparison is exact: a phase that moves a golden names it and
// regenerates it (GCV-004 §14.3). The speck overlay also keeps its number of
// specks (counts.json).
var updateGolden = flag.Bool("update", false, "rewrite the golden looks")

const goldenDir = "testdata/golden"

type goldenCase struct {
	name  string
	build func(t *testing.T, e *graph.Engine, img graph.NodeID) graph.NodeID
	opts  Options
}

// specksCase is the overlay of detect.specks, whose speck count is checked too.
const specksCase = "specks-overlay"

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
		{"source", func(_ *testing.T, _ *graph.Engine, img graph.NodeID) graph.NodeID { return img }, Options{}},
		{"exposure", c("exposure", 0.5), Options{}},
		{"gaussian", c("gaussian", 2.0), Options{}},
		{"contrast", c("contrast", 0.3), Options{}},
		{"levels-stats", func(t *testing.T, e *graph.Engine, img graph.NodeID) graph.NodeID {
			v, err := e.Evaluate(context.Background(), call(t, e, "stats", img).ID, graph.EvalOptions{Scale: 1})
			if err != nil {
				t.Fatal(err)
			}
			s := v.(*graph.DataValue).Data.(map[string]any) // awaited first: options are literals
			return call(t, e, "levels", img, map[string]any{"black": s["p01"], "white": s["p99"]}).ID
		}, Options{}},
		{"levels-gamma", c("levels", map[string]any{"black": 0.02, "white": 0.6, "gamma": 1.4}), Options{}},
		{"togray", c("toGray"), Options{}},
		{"mask-luminance", lum, Options{}},
		{"mask-color", c("mask.color", map[string]any{"hue": 30.0, "tol": 40.0, "minSat": 0.05}), Options{}},
		{"mask-box", c("mask.box", []any{40.0, 30.0, 100.5, 80.25}), Options{}},
		{"mask-ellipse", c("mask.ellipse", []any{40.0, 30.0, 100.0, 80.0}), Options{}},
		{"mask-gradient-edge", c("mask.gradient", map[string]any{"from": "top", "to": 0.6}), Options{}},
		{"mask-gradient-points", c("mask.gradient", map[string]any{"from": []any{0.0, 0.0}, "to": []any{200.0, 150.0}}), Options{}},
		{"mask-flood", c("mask.flood", []any{128.0, 96.0}, map[string]any{"tol": 0.08}), Options{}},
		{"mask-feather", func(t *testing.T, e *graph.Engine, img graph.NodeID) graph.NodeID {
			return call(t, e, "feather", lum(t, e, img), 6.0).ID
		}, Options{}},
		{"mask-refine", func(t *testing.T, e *graph.Engine, img graph.NodeID) graph.NodeID {
			return call(t, e, "refine", lum(t, e, img), graph.NodeArg{ID: img}).ID
		}, Options{}},
		{"mask-overlay", func(t *testing.T, e *graph.Engine, img graph.NodeID) graph.NodeID {
			return call(t, e, "overlay", img, graph.NodeArg{ID: lum(t, e, img)}).ID
		}, Options{}},
		{"specks-overlay", func(t *testing.T, e *graph.Engine, img graph.NodeID) graph.NodeID {
			sp := call(t, e, "detect.specks", img)
			return call(t, e, "overlay", img, graph.NodeArg{ID: sp.ID}, map[string]any{"labels": "id"}).ID
		}, Options{}},
		{"histogram", c("histogram"), Options{}},
		{"histogram-r", c("histogram", map[string]any{"channel": "r"}), Options{}},
		// GCV-003 Phase 5: a profile across the star as a line chart
		{"profile-chart", c("profile", []any{180.5, 46.5}, []any{220.5, 46.5}, map[string]any{"width": 3.0}), Options{}},
		{"compare-side", compareCase("side"), Options{}},
		{"compare-split", compareCase("split"), Options{}},
		// GCV-002: a 32 × 32 crop at zoom 8 with rulers every 4 doc px and a grid every 8
		{"ruler-crop-zoom8", c("crop", []any{100.0, 40.0, 32.0, 32.0}), Options{Zoom: 8, Ruler: 4, Grid: 8}},
		{"grid-gray-crop-zoom4", func(t *testing.T, e *graph.Engine, img graph.NodeID) graph.NodeID {
			return call(t, e, "crop", call(t, e, "toGray", img).ID, []any{180.0, 20.0, 48.0, 40.0}).ID
		}, Options{Zoom: 4, Grid: 4}},
		// GCV-002 Phase 6: decorations at zoom and a sheet
		{"overlay-crop-zoom8", func(t *testing.T, e *graph.Engine, img graph.NodeID) graph.NodeID {
			sp := call(t, e, "detect.specks", img)
			c := call(t, e, "crop", img, []any{180.0, 20.0, 48.0, 40.0})
			return call(t, e, "overlay", c.ID, graph.NodeArg{ID: sp.ID}, map[string]any{"labels": "id"}).ID
		}, Options{Zoom: 8}},
		{"annotate-crop-zoom4", func(t *testing.T, e *graph.Engine, img graph.NodeID) graph.NodeID {
			a := call(t, e, "annotate", img, []any{
				map[string]any{"circle": []any{200.5, 46.5, 6.0}, "color": "lime", "label": "star"},
				map[string]any{"point": []any{160.0, 30.0}},
				map[string]any{"box": []any{170.0, 60.0, 12.0, 10.0}, "color": "#ff8800"},
				map[string]any{"polyline": []any{[]any{152.0, 75.0}, []any{165.0, 70.0}, []any{180.0, 80.0}}, "color": "cyan"},
				map[string]any{"text": "hair?", "at": []any{185.0, 25.0}},
			}, map[string]any{"width": 1.0})
			return call(t, e, "crop", a.ID, []any{150.0, 20.0, 64.0, 64.0}).ID
		}, Options{Zoom: 4}},
		{"sheet-2x2", func(t *testing.T, e *graph.Engine, img graph.NodeID) graph.NodeID {
			v := call(t, e, "view", img, "speck")
			return call(t, e, "sheet", 0, []any{[]any{200.5, 46.5}, []any{40.0, 150.0, 30.0, 12.0}},
				map[string]any{"views": []any{graph.NodeArg{ID: img}, graph.NodeArg{ID: v.ID}}, "size": 24.0, "zoom": 4.0, "labels": "index"}).ID
		}, Options{}},
	}
}

func compareCase(mode string) func(*testing.T, *graph.Engine, graph.NodeID) graph.NodeID {
	return func(t *testing.T, e *graph.Engine, img graph.NodeID) graph.NodeID {
		other := call(t, e, "exposure", img, 0.3)
		return call(t, e, "compare", 0, []any{graph.NodeArg{ID: img}, graph.NodeArg{ID: other.ID}}, map[string]any{"mode": mode}).ID
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
			if gc.name == specksCase {
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
			if gc.name == specksCase && counts[gc.name] != wantCounts[gc.name] {
				t.Errorf("speck count %d, golden %d", counts[gc.name], wantCounts[gc.name])
			}
			switch {
			case d.shape != "":
				t.Fatalf("shape changed: %s", d)
			case d.differ != 0:
				t.Errorf("look differs from its golden: %s", d)
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
