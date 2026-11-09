package ops

import (
	"context"
	"encoding/json"
	stdimage "image"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-go-golems/goja-cv/internal/testimage"
	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// Real-data tests (GCV-001 guide §13.4), gated on the files being present:
//
//	GOJACV_TILES     directory of the DUST-002 tiles (default ~/dustdet/cv_tiles)
//	GOJACV_DNG       a VueScan LinearRaw DNG (e.g. raw0027.dng); unset: skipped
//	GOJACV_DNG_FULL  1: also decode the whole DNG (638 MB) and compare the window

const realTile = "1d955ef4b27d3d88_1536_1536"

func tilesDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("GOJACV_TILES")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skip("no home directory")
		}
		dir = filepath.Join(home, "dustdet", "cv_tiles")
	}
	if _, err := os.Stat(filepath.Join(dir, realTile+".tif")); err != nil {
		t.Skipf("real tiles not found in %s (set GOJACV_TILES)", dir)
	}
	return dir
}

func evalImage(t *testing.T, e *graph.Engine, n *graph.Node) *graph.ImageValue {
	t.Helper()
	v, err := e.Evaluate(context.Background(), n.ID, graph.EvalOptions{Scale: 1})
	if err != nil {
		t.Fatalf("evaluate %s: %v", graph.Label(n), err)
	}
	return v.(*graph.ImageValue)
}

func maxDiff(a, b *buffer.Buffer) float64 {
	var d float64
	for i := range a.Pix {
		d = math.Max(d, math.Abs(float64(a.Pix[i])-float64(b.Pix[i])))
	}
	return d
}

// TestRealTileReEncodedAsLinear: the tile stores sRGB_encode(v); its linear
// buffer written as a plain 16-bit TIFF and opened with encoding "linear"
// gives the same numbers, masks and stats: the colleague's workaround is no
// longer needed.
func TestRealTileReEncodedAsLinear(t *testing.T) {
	dir := tilesDir(t)
	e := graph.NewEngine(NewRegistry(), graph.EngineOptions{})
	tile := mustOpen(t, e, filepath.Join(dir, realTile+".tif"), nil)
	if tile.Enc != buffer.EncSRGB {
		t.Fatalf("tile encoding %v", tile.Enc)
	}
	lin := call(t, e, graph.AsOp, tile.ID, "linear")
	lin1 := evalImage(t, e, lin)
	vPath := filepath.Join(t.TempDir(), realTile+"_v.tif")
	if err := testimage.WriteFile(vPath, lin1.Buf, 16, buffer.Meta{}); err != nil {
		t.Fatal(err)
	}
	v := mustOpen(t, e, vPath, map[string]any{"encoding": "linear"})
	lin2 := evalImage(t, e, v)
	if d := maxDiff(lin1.Buf, lin2.Buf); d > 0.5/65535+1e-7 {
		t.Fatalf("linear buffers differ by %g", d)
	}
	m1 := evalImage(t, e, call(t, e, "mask.luminance", lin.ID, map[string]any{"above": 0.9}))
	m2 := evalImage(t, e, call(t, e, "mask.luminance", v.ID, map[string]any{"above": 0.9}))
	if d := maxDiff(m1.Buf, m2.Buf); d > 1e-3 {
		t.Fatalf("masks differ by %g", d)
	}
	s1 := evalData(t, e, call(t, e, "stats", lin.ID), 1).(map[string]any)
	s2 := evalData(t, e, call(t, e, "stats", v.ID), 1).(map[string]any)
	for k, want := range map[string]float64{"meanL": 0.33567, "p01": 0.0085, "p99": 0.918} {
		a, b := s1[k].(float64), s2[k].(float64)
		if math.Abs(a-b) > 1e-4 || math.Abs(a-want) > 5e-4 {
			t.Errorf("%s: tile %g, re-encoded %g, guide %g", k, a, b, want)
		}
	}
	t.Logf("tile stats: meanL %.5f p01 %.5f p99 %.5f", s1["meanL"], s1["p01"], s1["p99"])
}

// TestRealTileAbsoluteCoordinates: with the origin from the tile's JSON,
// region centroids are raw-scan coordinates.
func TestRealTileAbsoluteCoordinates(t *testing.T) {
	dir := tilesDir(t)
	raw, err := os.ReadFile(filepath.Join(dir, realTile+".json"))
	if err != nil {
		t.Skip(err)
	}
	var meta struct {
		OriginX int `json:"origin_x"`
		OriginY int `json:"origin_y"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatal(err)
	}
	e := graph.NewEngine(NewRegistry(), graph.EngineOptions{})
	tile := mustOpen(t, e, filepath.Join(dir, realTile+".tif"), map[string]any{"origin": []any{float64(meta.OriginX), float64(meta.OriginY)}})
	if tile.Frame != (graph.Frame{X: meta.OriginX, Y: meta.OriginY, W: 1024, H: 1024}) {
		t.Fatalf("frame %s", tile.Frame)
	}
	v, err := e.Evaluate(context.Background(), call(t, e, "detect.specks", tile.ID).ID, graph.EvalOptions{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	items := v.(*graph.RegionsValue).Items
	if len(items) == 0 {
		t.Fatal("no specks")
	}
	for _, r := range items {
		if r.Centroid[0] < float64(meta.OriginX) || r.Centroid[0] >= float64(meta.OriginX+1024) ||
			r.Centroid[1] < float64(meta.OriginY) || r.Centroid[1] >= float64(meta.OriginY+1024) {
			t.Fatalf("speck %d centroid %v outside the raw-scan window", r.ID, r.Centroid)
		}
	}
	t.Logf("%d specks, first at raw (%.1f, %.1f)", len(items), items[0].Centroid[0], items[0].Centroid[1])
}

// TestRealDNGWindow: a VueScan DNG probes as linear and a 1024² window opens
// in under a second, in scan coordinates.
func TestRealDNGWindow(t *testing.T) {
	path := os.Getenv("GOJACV_DNG")
	if path == "" {
		t.Skip("set GOJACV_DNG to a VueScan LinearRaw DNG (e.g. raw0027.dng)")
	}
	if _, err := os.Stat(path); err != nil {
		t.Skip(err)
	}
	info, err := buffer.ProbeFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Format != "dng" || info.Bits != 16 || info.Encoding != buffer.EncLinear || info.Channels != 3 {
		t.Fatalf("probe %+v", info)
	}
	t.Logf("probe: %d×%d, %d-bit %s, %s, notes %v", info.W, info.H, info.Bits, info.Format, info.Encoding, info.Notes)
	if filepath.Base(path) == "raw0027.dng" && (info.W != 7256 || info.H != 7326) {
		t.Fatalf("raw0027.dng is %d×%d, want 7256×7326", info.W, info.H)
	}
	e := graph.NewEngine(NewRegistry(), graph.EngineOptions{})
	start := time.Now()
	win := mustOpen(t, e, path, map[string]any{"box": []any{1536.0, 1536.0, 1024.0, 1024.0}})
	wv := evalImage(t, e, win)
	took := time.Since(start)
	t.Logf("window open + decode: %v", took)
	if win.Frame != (graph.Frame{X: 1536, Y: 1536, W: 1024, H: 1024}) || wv.Buf.W != 1024 {
		t.Fatalf("window frame %s buffer %v", win.Frame, wv.Buf)
	}
	if took > time.Second {
		t.Errorf("window took %v, want under a second", took)
	}
	if os.Getenv("GOJACV_DNG_FULL") != "1" {
		return
	}
	full, _, err := buffer.DecodeFile(context.Background(), path, buffer.DecodeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	box := stdimage.Rect(1536, 1536, 2560, 2560)
	win2, _, err := buffer.DecodeFile(context.Background(), path, buffer.DecodeOptions{Box: &box})
	if err != nil {
		t.Fatal(err)
	}
	for c := 0; c < 3; c++ {
		for y := 0; y < 1024; y++ {
			for x := 0; x < 1024; x++ {
				if win2.Plane(c)[y*1024+x] != full.Plane(c)[(1536+y)*full.W+1536+x] {
					t.Fatalf("window (%d, %d, %d) differs from the full decode", x, y, c)
				}
			}
		}
	}
}
