package ops

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

// The hot list of GCV-003 §7.4 on the colleague's labelled tiles (§13.4):
// blobs of the unstretched DoG above 2 × its p99, measured with
// img.measure(regions) (core and ring sized from each region's area) and
// ranked by snr. Gated like TestRealViewRanking (GOJACV_TILES, GOJACV_GOLD).
// The tiles store density as their code value (DUST-002 script 82): they
// are opened with encoding "linear", as the tile's JSON says. The labels
// change as the colleague annotates, so the tests compare methods and
// check relative properties instead of today's counts.

func goldDirOrSkip(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("GOJACV_GOLD")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, "dustds", "gold")
	}
	if m, _ := filepath.Glob(filepath.Join(dir, "*.json")); len(m) == 0 {
		t.Skipf("no gold labels in %s (set GOJACV_GOLD)", dir)
	}
	return dir
}

// openTile opens a DUST-002 tile as its JSON says: encoding "linear" (the
// code value is density).
func openTile(t testing.TB, e *graph.Engine, tif string) *graph.Node {
	t.Helper()
	img, err := e.Open(tif, map[string]any{"encoding": "linear"})
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// hotList builds the measured hot regions of a tile, in detection order
// (largest first); rankBySNR orders them.
func hotList(t testing.TB, e *graph.Engine, img *graph.Node) *graph.Node {
	t.Helper()
	mk := func(name string, recv graph.NodeID, raw ...any) *graph.Node {
		r, err := e.Call(name, recv, raw)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return r.Node()
	}
	gray := mk("toGray", img.ID, "average")
	dog := mk("dog", gray.ID, 1.0, 4.0, map[string]any{"polarity": "bright"})
	st, err := e.Evaluate(context.Background(), mk("stats", dog.ID).ID, graph.EvalOptions{Scale: 1})
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	white := 2 * st.(*graph.DataValue).Data.(map[string]any)["p99"].(float64) // awaited first: options are literals
	lv := mk("levels", dog.ID, map[string]any{"black": 0.0, "white": white})
	hot := mk("components", mk("mask.luminance", lv.ID, map[string]any{"above": 0.99}).ID, map[string]any{"minArea": 4.0})
	m := mk("measure", dog.ID, graph.NodeArg{ID: hot.ID}, map[string]any{"stats": []any{"max"}, "prefix": "dog"})
	return mk("measure", img.ID, graph.NodeArg{ID: m.ID})
}

// rankBy returns the regions ordered by a measure, largest first (null
// last), as an agent's sort((a, b) => b.snr - a.snr) does.
func rankBy(items []*graph.Region, key string) []*graph.Region {
	out := append([]*graph.Region(nil), items...)
	v := func(r *graph.Region) float64 {
		if x, ok := r.Measures[key]; ok && !math.IsNaN(x) {
			return x
		}
		if key == "area" {
			return r.Area
		}
		return math.Inf(-1)
	}
	sort.SliceStable(out, func(i, j int) bool { return v(out[i]) > v(out[j]) })
	return out
}

func quartiles(v []float64) [3]float64 {
	if len(v) == 0 {
		return [3]float64{}
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	q := func(p float64) float64 { return s[int(math.Round(p*float64(len(s)-1)))] }
	return [3]float64{q(0.25), q(0.5), q(0.75)}
}

// labelledTile is one tile with gold labels: its file, origin and points.
type labelledTile struct {
	id, tif string
	ox, oy  float64
	pts     []goldPoint
}

// labelledTiles lists the tiles that have both gold labels and a tile file.
func labelledTiles(t *testing.T) []labelledTile {
	t.Helper()
	tiles := tilesDir(t)
	goldDir := goldDirOrSkip(t)
	files, _ := filepath.Glob(filepath.Join(goldDir, "*.json"))
	ids := map[string]bool{}
	for _, f := range files {
		b := filepath.Base(f)
		if strings.HasSuffix(b, ".p1.json") || strings.HasSuffix(b, ".final.json") {
			ids[strings.TrimSuffix(strings.TrimSuffix(b, ".p1.json"), ".final.json")] = true
		}
	}
	var out []labelledTile
	for id := range ids {
		tif := filepath.Join(tiles, id+".tif")
		raw, err := os.ReadFile(filepath.Join(tiles, id+".json"))
		if err != nil {
			continue
		}
		var meta struct {
			X float64 `json:"origin_x"`
			Y float64 `json:"origin_y"`
		}
		if json.Unmarshal(raw, &meta) != nil {
			continue
		}
		pts := goldPoints(t, goldDir, id)
		if _, err := os.Stat(tif); err != nil || len(pts) == 0 {
			continue
		}
		out = append(out, labelledTile{id, tif, meta.X, meta.Y, pts})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].id < out[j].id })
	if len(out) == 0 {
		t.Skip("no labelled tile has its tile file")
	}
	return out
}

// TestRealHotListMeasures: the hot list's snr separates labelled dust from
// the other spots, and ranking by snr finds more gold points among the
// first 30 spots than ranking by area (the detector's order).
func TestRealHotListMeasures(t *testing.T) {
	var dustSNR, dustChroma, otherSNR, otherChroma []float64
	nGold, bySNR, byArea, nAny, nRegions := 0, 0, 0, 0, 0
	tiles := labelledTiles(t)
	for _, lt := range tiles {
		e := graph.NewEngine(NewRegistry(), graph.EngineOptions{})
		v, err := e.Evaluate(context.Background(), hotList(t, e, openTile(t, e, lt.tif)).ID, graph.EvalOptions{Scale: 1})
		if err != nil {
			t.Fatal(err)
		}
		items := v.(*graph.RegionsValue).Items
		nRegions += len(items)
		near := func(r *graph.Region, p goldPoint, d float64) bool {
			return math.Hypot(r.Centroid[0]-(p.X-lt.ox), r.Centroid[1]-(p.Y-lt.oy)) <= d
		}
		inTop := func(ranked []*graph.Region, p goldPoint) bool {
			for _, r := range ranked[:min(30, len(ranked))] {
				if near(r, p, 4) {
					return true
				}
			}
			return false
		}
		snrRank, areaRank := rankBy(items, "snr"), rankBy(items, "area")
		for _, p := range lt.pts {
			nGold++
			if inTop(snrRank, p) {
				bySNR++
			}
			if inTop(areaRank, p) {
				byArea++
			}
			for _, r := range items {
				if near(r, p, 4) {
					nAny++
					break
				}
			}
			if p.Type != "dust" {
				continue
			}
			for _, r := range items {
				if near(r, p, 4) {
					dustSNR = append(dustSNR, r.Measures["snr"])
					if c := r.Measures["stepChroma"]; !math.IsNaN(c) { // null below |snr| 1
						dustChroma = append(dustChroma, c)
					}
					break
				}
			}
		}
		for _, r := range items {
			labelled := false
			for _, p := range lt.pts {
				if near(r, p, p.R+4) {
					labelled = true
					break
				}
			}
			if !labelled {
				otherSNR = append(otherSNR, r.Measures["snr"])
				if c := r.Measures["stepChroma"]; !math.IsNaN(c) {
					otherChroma = append(otherChroma, c)
				}
			}
		}
	}
	ds, dc, os_, oc := quartiles(dustSNR), quartiles(dustChroma), quartiles(otherSNR), quartiles(otherChroma)
	pct := func(n int) float64 { return 100 * float64(n) / float64(nGold) }
	t.Logf("hot list: %d tiles, %d regions (%.1f per tile), %d gold points: %.1f %% within 4 px of a top-30 spot by snr, %.1f %% by area, %.1f %% of any spot",
		len(tiles), nRegions, float64(nRegions)/float64(len(tiles)), nGold, pct(bySNR), pct(byArea), pct(nAny))
	t.Logf("labelled dust (%d matched): snr median %.2f (middle half %.2f–%.2f), stepChroma median %.2f (%.2f–%.2f)", len(dustSNR), ds[1], ds[0], ds[2], dc[1], dc[0], dc[2])
	t.Logf("unlabelled spots (%d): snr median %.2f (middle half %.2f–%.2f), stepChroma median %.2f (%.2f–%.2f)", len(otherSNR), os_[1], os_[0], os_[2], oc[1], oc[0], oc[2])
	if len(dustSNR) == 0 || len(otherSNR) == 0 {
		t.Skip("no matched dust or no unlabelled spot")
	}
	if ds[1] < 2*os_[1] || ds[0] < os_[1] {
		t.Errorf("snr does not separate labelled dust (median %.2f, lower quartile %.2f) from the other spots (median %.2f)", ds[1], ds[0], os_[1])
	}
	if bySNR <= byArea {
		t.Errorf("ranking by snr puts %d gold points in the top 30, ranking by area %d: snr should rank labelled spots earlier", bySNR, byArea)
	}
}

// TestRealGoldCirclesMeasureHigherSNR (D17): gold dust points measured as
// circles sized from their label stand out more than with a fixed 3 px core
// and the GCV-003 ring [6, 12]. The gold r is dustview's fit radius, twice
// the speck's half-level radius, so the circle passed is { cx, cy, r: r / 2 }
// (core r / 2, ring [0.75 r, 1.25 r]); with the label's r itself as the core
// the ring lies on the speck's tail (logged for comparison).
func TestRealGoldCirclesMeasureHigherSNR(t *testing.T) {
	var half, full, fixed []float64
	for _, lt := range labelledTiles(t) {
		e := graph.NewEngine(NewRegistry(), graph.EngineOptions{})
		img := openTile(t, e, lt.tif)
		var halves, fulls, points []any
		for _, p := range lt.pts {
			if p.Type != "dust" {
				continue
			}
			x, y := p.X-lt.ox, p.Y-lt.oy
			halves = append(halves, map[string]any{"cx": x, "cy": y, "r": p.R / 2})
			fulls = append(fulls, map[string]any{"cx": x, "cy": y, "r": p.R})
			points = append(points, []any{x, y})
		}
		if len(points) == 0 {
			continue
		}
		for _, run := range []struct {
			items []any
			opts  map[string]any
			out   *[]float64
		}{
			{halves, map[string]any{}, &half},
			{fulls, map[string]any{}, &full},
			{points, map[string]any{"coreRadius": 3.0, "ringRadii": []any{6.0, 12.0}}, &fixed},
		} {
			cr, err := e.Call("measure", img.ID, []any{run.items, run.opts})
			if err != nil {
				t.Fatal(err)
			}
			v, err := e.Evaluate(context.Background(), cr.Node().ID, graph.EvalOptions{Scale: 1})
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range v.(*graph.RegionsValue).Items {
				if s := r.Measures["snr"]; !math.IsNaN(s) {
					*run.out = append(*run.out, s)
				}
			}
		}
	}
	if len(half) == 0 {
		t.Skip("no gold dust points")
	}
	qh, qr, qf := quartiles(half), quartiles(full), quartiles(fixed)
	t.Logf("gold dust (%d points): snr median %.2f (middle half %.2f–%.2f) with r / 2, %.2f (%.2f–%.2f) with the label's r, %.2f (%.2f–%.2f) with a 3 px core",
		len(half), qh[1], qh[0], qh[2], qr[1], qr[0], qr[2], qf[1], qf[0], qf[2])
	if qh[1] <= qf[1] {
		t.Errorf("circles sized from the label: median snr %.2f, not above the fixed 3 px core's %.2f", qh[1], qf[1])
	}
}

// hotPoints are the centroids (buffer px) of the hot list of the harness
// tile, its linear buffer and that buffer's luminance plane.
func hotPoints(tb testing.TB) (*buffer.Buffer, []float32, [][2]float64, []float64) {
	tb.Helper()
	dir := os.Getenv("GOJACV_TILES")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, "dustdet", "cv_tiles")
	}
	tif := filepath.Join(dir, realTile+".tif")
	if _, err := os.Stat(tif); err != nil {
		tb.Skipf("real tiles not found in %s (set GOJACV_TILES)", dir)
	}
	e := graph.NewEngine(NewRegistry(), graph.EngineOptions{})
	v, err := e.Evaluate(context.Background(), hotList(tb, e, openTile(tb, e, tif)).ID, graph.EvalOptions{Scale: 1})
	if err != nil {
		tb.Fatal(err)
	}
	ranked := rankBy(v.(*graph.RegionsValue).Items, "snr")
	var pts [][2]float64
	var cores []float64
	for _, r := range ranked[:min(28, len(ranked))] {
		pts = append(pts, [2]float64{r.Centroid[0], r.Centroid[1]})
		cores = append(cores, math.Max(1, math.Sqrt(r.Area/math.Pi)))
	}
	lin, _, err := buffer.DecodeFile(context.Background(), tif, buffer.DecodeOptions{}) // the code value is density (linear)
	if err != nil {
		tb.Fatal(err)
	}
	gray, err := kernels.ToGray(context.Background(), lin, kernels.GrayWeights["rec709"])
	if err != nil {
		tb.Fatal(err)
	}
	return lin, gray.Plane(0), pts, cores
}

// probeNative is measure's work for the hot list: one probe per
// spot in a window around it.
func probeNative(b *buffer.Buffer, gray []float32, pts [][2]float64, cores []float64) {
	for i, p := range pts {
		kernels.ProbePoint(context.Background(), b, p[0], p[1], kernels.CoreRingOptions{Core: cores[i], R1: 1.5 * cores[i], R2: 2.5 * cores[i], Gray: gray})
	}
}

// probeHelper is the colleague's mem.probe for the same spots: three
// whole-image ellipse masks, a minus, and two whole-image weighted stats per
// spot (56 stats calls for 28 spots).
func probeHelper(tb testing.TB, b *buffer.Buffer, gray []float32, pts [][2]float64, cores []float64) {
	ctx := context.Background()
	for i, p := range pts {
		c := cores[i]
		ell := func(r float64) *buffer.Buffer {
			m, err := kernels.EllipseMask(ctx, b.W, b.H, 1, p[0]-r, p[1]-r, 2*r, 2*r)
			if err != nil {
				tb.Fatal(err)
			}
			return m
		}
		core, outer, inner := ell(c), ell(2.5*c), ell(1.5*c)
		ring, err := kernels.Combine(ctx, outer, inner, func(x, y float32) float32 { return min(x, 1-y) })
		if err != nil {
			tb.Fatal(err)
		}
		if _, err := kernels.StatsWeighted(ctx, b, gray, core.Plane(0), nil); err != nil {
			tb.Fatal(err)
		}
		if _, err := kernels.StatsWeighted(ctx, b, gray, ring.Plane(0), []float64{0.1, 0.9}); err != nil {
			tb.Fatal(err)
		}
	}
}

func BenchmarkHotList(b *testing.B) {
	buf, gray, pts, cores := hotPoints(b)
	b.Run("measure", func(b *testing.B) {
		for range b.N {
			probeNative(buf, gray, pts, cores)
		}
	})
	b.Run("helper", func(b *testing.B) {
		for range b.N {
			probeHelper(b, buf, gray, pts, cores)
		}
	})
}

// TestHotListIsFasterThanTheHelper: measuring the hot list on a 1024² tile
// at scale 1 (warm: the image is decoded once) is at least 20 times faster
// than the helper's 56 whole-image stats calls.
func TestHotListIsFasterThanTheHelper(t *testing.T) {
	if testing.Short() {
		t.Skip("benchmark")
	}
	buf, gray, pts, cores := hotPoints(t)
	native := testing.Benchmark(func(b *testing.B) {
		for range b.N {
			probeNative(buf, gray, pts, cores)
		}
	})
	helper := testing.Benchmark(func(b *testing.B) {
		for range b.N {
			probeHelper(b, buf, gray, pts, cores)
		}
	})
	ratio := float64(helper.NsPerOp()) / float64(max(1, native.NsPerOp()))
	t.Logf("%d spots on a %d×%d tile: measure %.2f ms, helper %.1f ms: %.0f× faster", len(pts), buf.W, buf.H,
		float64(native.NsPerOp())/1e6, float64(helper.NsPerOp())/1e6, ratio)
	if ratio < 20 {
		t.Fatalf("measure is only %.1f× faster than the helper, want ≥ 20×", ratio)
	}
}
