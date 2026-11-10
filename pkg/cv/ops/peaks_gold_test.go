package ops

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph/graphtest"
)

// Gold-based tests of peaks, top-hats and line openings (GCV-005 §14.4,
// §15.2), gated like TestRealViewRanking (GOJACV_TILES, GOJACV_GOLD). The
// labels change as the colleague annotates, so every assertion compares two
// methods on the same points in one run (or bounds a quantity that does not
// depend on the count), and every test logs its numbers.
//
// The gold r is dustview's fit radius, about twice the speck's own radius
// (GCV-004's re-tune: median snr 1.56 with a core of r / 2, 0.86 with r).
// Coverage and position use it as the label's extent (a point is covered by
// a candidate within max(r, 3) px, as in the prototype and the hot-list
// numbers); a measurement of a gold point uses a core of r / 2.

// goldSession is one labelled tile opened in its own engine.
type goldSession struct {
	lt  labelledTile
	e   *graph.Engine
	img *graph.Node
}

func goldSessions(t *testing.T) []goldSession {
	t.Helper()
	var out []goldSession
	for _, lt := range labelledTiles(t) {
		e := graph.NewEngine(NewRegistry(), graphtest.Options(t))
		out = append(out, goldSession{lt, e, openTile(t, e, lt.tif)})
	}
	return out
}

// node builds an op call on recv.
func (g goldSession) node(t *testing.T, name string, recv graph.NodeID, raw ...any) *graph.Node {
	t.Helper()
	cr, err := g.e.Call(name, recv, raw)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return cr.Node()
}

func (g goldSession) eval(t *testing.T, n *graph.Node) graph.Value {
	t.Helper()
	v, err := g.e.Evaluate(context.Background(), n.ID, graph.EvalOptions{Scale: 1})
	if err != nil {
		t.Fatalf("%s: %v", n.Op.Name, err)
	}
	return v
}

func (g goldSession) regions(t *testing.T, n *graph.Node) []*graph.Region {
	t.Helper()
	return g.eval(t, n).(*graph.RegionsValue).Items
}

// peaks of the tile (bright).
func (g goldSession) peaks(t *testing.T, radii []float64, snr float64) []*graph.Region {
	t.Helper()
	rs := make([]any, len(radii))
	for i, r := range radii {
		rs[i] = r
	}
	return g.regions(t, g.node(t, "peaks", g.img.ID, map[string]any{"radius": rs, "snr": snr}))
}

// hot is the DUST-002 helpers' hot list at level: components (min area 4)
// of the speck view above level.
func (g goldSession) hot(t *testing.T, level float64) []*graph.Region {
	t.Helper()
	sp := g.node(t, "view", g.img.ID, "speck")
	m := g.node(t, "mask.luminance", sp.ID, map[string]any{"above": level})
	return g.regions(t, g.node(t, "components", m.ID, map[string]any{"minArea": 4.0}))
}

// snapped are the hot centroids moved to the point of highest snr within
// 2 px (core 3, ring [6, 12] on the tile): the helpers' mem.probe.
func (g goldSession) snapped(t *testing.T, hot []*graph.Region) [][2]float64 {
	t.Helper()
	var grid []any
	for _, r := range hot {
		for dy := -2; dy <= 2; dy++ {
			for dx := -2; dx <= 2; dx++ {
				if dx*dx+dy*dy <= 4 {
					grid = append(grid, []any{r.Centroid[0] + float64(dx), r.Centroid[1] + float64(dy)})
				}
			}
		}
	}
	if len(grid) == 0 {
		return nil
	}
	m := g.regions(t, g.node(t, "measure", g.img.ID, grid, map[string]any{"coreRadius": 3.0, "ringRadii": []any{6.0, 12.0}}))
	out := make([][2]float64, len(hot))
	per := len(grid) / len(hot)
	for i := range hot {
		best := m[i*per]
		for _, r := range m[i*per : (i+1)*per] {
			if r.Measures["snr"] > best.Measures["snr"] {
				best = r
			}
		}
		out[i] = best.Centroid
	}
	return out
}

// local is a gold point in the tile's frame.
func (g goldSession) local(p goldPoint) (float64, float64) { return p.X - g.lt.ox, p.Y - g.lt.oy }

func centres(rs []*graph.Region) [][2]float64 {
	out := make([][2]float64, len(rs))
	for i, r := range rs {
		out[i] = r.Centroid
	}
	return out
}

func nearestDist(pts [][2]float64, x, y float64) float64 {
	d := math.Inf(1)
	for _, p := range pts {
		d = math.Min(d, math.Hypot(p[0]-x, p[1]-y))
	}
	return d
}

func reach(p goldPoint) float64 { return math.Max(p.R, 3) }

// covered counts the points that have a candidate within max(r, 3) px.
func (g goldSession) covered(pts []goldPoint, cands [][2]float64) int {
	n := 0
	for _, p := range pts {
		x, y := g.local(p)
		if nearestDist(cands, x, y) <= reach(p) {
			n++
		}
	}
	return n
}

// notNear counts the candidates farther than max(r, 3) px from every gold
// point (of any type).
func (g goldSession) notNear(cands [][2]float64) int {
	n := 0
	for _, c := range cands {
		near := false
		for _, p := range g.lt.pts {
			x, y := g.local(p)
			if math.Hypot(c[0]-x, c[1]-y) <= reach(p) {
				near = true
				break
			}
		}
		if !near {
			n++
		}
	}
	return n
}

func dust(pts []goldPoint, keep func(goldPoint) bool) []goldPoint {
	var out []goldPoint
	for _, p := range pts {
		if p.Type == "dust" && (keep == nil || keep(p)) {
			out = append(out, p)
		}
	}
	return out
}

func medianOf(v []float64) float64 {
	if len(v) == 0 {
		return math.NaN()
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	if len(s)%2 == 1 {
		return s[len(s)/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}

func pctOf(n, of int) float64 { return 100 * float64(n) / float64(max(1, of)) }

// TestRealPeaksPosition: for dust points with r ≤ 6, the nearest peak
// (radius [2, 4], snr 0.5) within r lies at a median distance ≤ 1.5 px,
// below the median of the probe-snapped hot-list centroids (speck view at
// 0.3, best snr within 2 px).
func TestRealPeaksPosition(t *testing.T) {
	var dp, dh []float64
	nPts := 0
	for _, g := range goldSessions(t) {
		pts := dust(g.lt.pts, func(p goldPoint) bool { return p.R <= 6 })
		if len(pts) == 0 {
			continue
		}
		pk := centres(g.peaks(t, []float64{2, 4}, 0.5))
		hs := g.snapped(t, g.hot(t, 0.3))
		for _, p := range pts {
			nPts++
			x, y := g.local(p)
			if d := nearestDist(pk, x, y); d <= p.R {
				dp = append(dp, d)
			}
			if d := nearestDist(hs, x, y); d <= p.R {
				dh = append(dh, d)
			}
		}
	}
	if nPts == 0 {
		t.Skip("no dust point with r ≤ 6")
	}
	mp, mh := medianOf(dp), medianOf(dh)
	p90 := func(v []float64) float64 {
		s := append([]float64(nil), v...)
		sort.Float64s(s)
		if len(s) == 0 {
			return math.NaN()
		}
		return s[int(0.9*float64(len(s)-1))]
	}
	t.Logf("position (%d dust points, r ≤ 6): peaks matched %d, distance median %.2f px, p90 %.2f px; probe-snapped hot list matched %d, median %.2f px, p90 %.2f px",
		nPts, len(dp), mp, p90(dp), len(dh), mh, p90(dh))
	if mp > 1.5 {
		t.Errorf("peaks: median distance %.2f px to gold, want ≤ 1.5", mp)
	}
	if len(dh) > 0 && mp >= mh {
		t.Errorf("peaks: median distance %.2f px, not below the snapped hot list's %.2f", mp, mh)
	}
}

// TestRealPeaksCoverage: dust covered by peaks (radius [2, 4, 8], snr 1)
// within max(r, 3) px is at least the hot list's coverage at level 0.3
// minus 2 points; logs the snr sweep (0.5, 1, 1.4, 2: coverage, peaks per
// tile, peaks not near gold) and the time per tile.
func TestRealPeaksCoverage(t *testing.T) {
	levels := []float64{0.5, 1, 1.4, 2}
	cov := make([]int, len(levels))
	count := make([][]float64, len(levels))
	far := make([][]float64, len(levels))
	var hotCov, nDust int
	var hotN, hotFar, times []float64
	tiles := goldSessions(t)
	for _, g := range tiles {
		pts := dust(g.lt.pts, nil)
		nDust += len(pts)
		for i, s := range levels {
			t0 := time.Now()
			pk := centres(g.peaks(t, []float64{2, 4, 8}, s))
			if s == 1 {
				times = append(times, time.Since(t0).Seconds())
			}
			cov[i] += g.covered(pts, pk)
			count[i] = append(count[i], float64(len(pk)))
			far[i] = append(far[i], float64(g.notNear(pk)))
		}
		hc := centres(g.hot(t, 0.3))
		hotCov += g.covered(pts, hc)
		hotN = append(hotN, float64(len(hc)))
		hotFar = append(hotFar, float64(g.notNear(hc)))
	}
	if nDust == 0 {
		t.Skip("no dust points")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "coverage within max(r, 3) px on %d tiles, %d dust points:", len(tiles), nDust)
	for i, s := range levels {
		fmt.Fprintf(&b, "\n  peaks [2, 4, 8] snr ≥ %-3g: covered %d (%.1f %%), peaks per tile median %.0f (min %.0f, max %.0f), not near gold median %.0f",
			s, cov[i], pctOf(cov[i], nDust), medianOf(count[i]), minOf(count[i]), maxOf(count[i]), medianOf(far[i]))
	}
	fmt.Fprintf(&b, "\n  hot list at 0.3:        covered %d (%.1f %%), spots per tile median %.0f (min %.0f, max %.0f), not near gold median %.0f",
		hotCov, pctOf(hotCov, nDust), medianOf(hotN), minOf(hotN), maxOf(hotN), medianOf(hotFar))
	fmt.Fprintf(&b, "\n  peaks time per tile at snr 1 (graph, uncached): median %.3f s, max %.3f s", medianOf(times), maxOf(times))
	t.Log(b.String())
	if cov[1] < hotCov-2 {
		t.Errorf("peaks at snr 1 cover %d dust points, the hot list %d", cov[1], hotCov)
	}
}

func minOf(v []float64) float64 {
	m := math.Inf(1)
	for _, x := range v {
		m = math.Min(m, x)
	}
	return m
}

func maxOf(v []float64) float64 {
	m := math.Inf(-1)
	for _, x := range v {
		m = math.Max(m, x)
	}
	return m
}

// TestRealPeaksDarkAreas: dust whose ring mean (core r / 2, ring
// [0.75 r, 1.25 r]) is below the tile's 25th luminance percentile is
// covered by peaks (snr 1) at least as often as by the hot list.
func TestRealPeaksDarkAreas(t *testing.T) {
	var nPts, byPeaks, byHot int
	for _, g := range goldSessions(t) {
		pts := dust(g.lt.pts, nil)
		if len(pts) == 0 {
			continue
		}
		p25 := g.eval(t, g.node(t, "stats", g.img.ID, map[string]any{"percentiles": []any{0.25}})).(*graph.DataValue).Data.(map[string]any)
		q := percentileOf(t, p25, 0.25)
		var circles []any
		for _, p := range pts {
			x, y := g.local(p)
			circles = append(circles, map[string]any{"cx": x, "cy": y, "r": p.R / 2})
		}
		m := g.regions(t, g.node(t, "measure", g.img.ID, circles))
		var darkPts []goldPoint
		for i, r := range m {
			if r.Measures["ringMean"] < q {
				darkPts = append(darkPts, pts[i])
			}
		}
		if len(darkPts) == 0 {
			continue
		}
		nPts += len(darkPts)
		byPeaks += g.covered(darkPts, centres(g.peaks(t, []float64{2, 4, 8}, 1)))
		byHot += g.covered(darkPts, centres(g.hot(t, 0.3)))
	}
	t.Logf("dark-area dust (ring below the tile's p25): %d points; peaks cover %d, the hot list %d", nPts, byPeaks, byHot)
	if nPts < 5 {
		return
	}
	if byPeaks < byHot {
		t.Errorf("dark-area dust: peaks cover %d of %d, the hot list %d", byPeaks, nPts, byHot)
	}
}

// percentileOf reads stats' requested percentile p.
func percentileOf(t *testing.T, s map[string]any, p float64) float64 {
	t.Helper()
	v, ok := s["percentiles"].(map[string]any)[PercentileKey(p)].(float64)
	if !ok {
		t.Fatalf("stats: no percentile %g in %v", p, s)
	}
	return v
}

// TestRealTopHatSaturated: dust with r ≥ 6 or a core maximum within 2 % of
// the tile's maximum is covered by components of the top-hat
// L − L.erode(12).dilate(12) above its own p99 at least as often as by the
// hot list; peaks with radii [2, 4, 8, 16] are logged.
func TestRealTopHatSaturated(t *testing.T) {
	var nPts, byTop, byHot, byPeaks int
	for _, g := range goldSessions(t) {
		pts := dust(g.lt.pts, nil)
		if len(pts) == 0 {
			continue
		}
		L := g.node(t, "toGray", g.img.ID)
		tileMax := percentileOf(t, g.eval(t, g.node(t, "stats", L.ID, map[string]any{"percentiles": []any{1.0}})).(*graph.DataValue).Data.(map[string]any), 1)
		var circles []any
		for _, p := range pts {
			x, y := g.local(p)
			circles = append(circles, map[string]any{"cx": x, "cy": y, "r": p.R / 2})
		}
		m := g.regions(t, g.node(t, "measure", L.ID, circles, map[string]any{"stats": []any{"max"}}))
		var sat []goldPoint
		for i, r := range m {
			if pts[i].R >= 6 || r.Measures["maxL"] >= 0.98*tileMax {
				sat = append(sat, pts[i])
			}
		}
		if len(sat) == 0 {
			continue
		}
		nPts += len(sat)
		open := g.node(t, "dilate", g.node(t, "erode", L.ID, 12.0).ID, 12.0)
		th := g.node(t, "sub", L.ID, graph.NodeArg{ID: open.ID})
		p99 := g.eval(t, g.node(t, "stats", th.ID)).(*graph.DataValue).Data.(map[string]any)["p99"].(float64)
		comps := g.regions(t, g.node(t, "components", g.node(t, "mask.luminance", th.ID, map[string]any{"above": p99}).ID, map[string]any{"minArea": 4.0}))
		byTop += g.covered(sat, centres(comps))
		byHot += g.covered(sat, centres(g.hot(t, 0.3)))
		byPeaks += g.covered(sat, centres(g.peaks(t, []float64{2, 4, 8, 16}, 1)))
	}
	t.Logf("saturated or large dust (r ≥ 6 or core max ≥ 98 %% of the tile's): %d points; top-hat components cover %d, the hot list %d, peaks [2, 4, 8, 16] %d", nPts, byTop, byHot, byPeaks)
	if nPts < 5 {
		return
	}
	if byTop < byHot {
		t.Errorf("saturated dust: the top-hat covers %d of %d, the hot list %d", byTop, nPts, byHot)
	}
}

// TestRealLineOpening: subtracting the maximum of eight line openings of
// radius 12 from the speck view leaves fewer elongated components (aspect >
// 5) at level 0.3, and keeps at least 98 % of the gold dust coverage. The
// guide's radius 10 removed more lines (40 → 3) but also four covered dust
// points of 164 (97.6 %): two saturated blobs (fit radius 18.5 and 28) hold
// 20 px segments; radius 12 gives 40 → 8 and 163 of 164 (GCV-005 deviation).
func TestRealLineOpening(t *testing.T) {
	const lineR = 12.0
	var before, after, covBefore, covAfter, nDust int
	for _, g := range goldSessions(t) {
		S := g.node(t, "view", g.img.ID, "speck")
		var lines *graph.Node
		for a := 0.0; a < 180; a += 22.5 {
			o := g.node(t, "dilate", g.node(t, "erode", S.ID, lineR, map[string]any{"line": a}).ID, lineR, map[string]any{"line": a})
			if lines == nil {
				lines = o
			} else {
				lines = g.node(t, "max", lines.ID, graph.NodeArg{ID: o.ID})
			}
		}
		clean := g.node(t, "sub", S.ID, graph.NodeArg{ID: lines.ID})
		comps := func(x *graph.Node) []*graph.Region {
			return g.regions(t, g.node(t, "components", g.node(t, "mask.luminance", x.ID, map[string]any{"above": 0.3}).ID, map[string]any{"minArea": 4.0}))
		}
		elongated := func(rs []*graph.Region) int {
			n := 0
			for _, r := range rs {
				if r.Measures["aspect"] > 5 {
					n++
				}
			}
			return n
		}
		cb, ca := comps(S), comps(clean)
		before += elongated(cb)
		after += elongated(ca)
		pts := dust(g.lt.pts, nil)
		nDust += len(pts)
		covBefore += g.covered(pts, centres(cb))
		covAfter += g.covered(pts, centres(ca))
		for _, p := range pts {
			x, y := g.local(p)
			if nearestDist(centres(cb), x, y) <= reach(p) && nearestDist(centres(ca), x, y) > reach(p) {
				t.Logf("dust lost by the line opening: %s (%.0f, %.0f) r %.1f", g.lt.id, p.X, p.Y, p.R)
			}
		}
	}
	t.Logf("line openings (radius 12, 8 angles) on the speck view at 0.3: elongated components (aspect > 5) %d → %d; dust covered %d → %d of %d", before, after, covBefore, covAfter, nDust)
	if before > 0 && after >= before {
		t.Errorf("elongated components %d → %d: the line opening removed none", before, after)
	}
	if float64(covAfter) < 0.98*float64(covBefore) {
		t.Errorf("dust coverage %d → %d: below 98 %%", covBefore, covAfter)
	}
}
