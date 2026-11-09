package ops

import (
	"bytes"
	"context"
	"encoding/json"
	stdimage "image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/look"
)

// TestRealViewRanking ports the colleague's view metric (DUST-002
// scripts/85-cv-views.py; GCV-002 guide §13.4) to the speck preset: for each
// labelled tile, render img.view("speck") as the look a vision model gets,
// find all local maxima (5 × 5, a plateau is one bump) on its luminance,
// and rank each labelled point's peak (within r + 2 px, r ≤ 12) among them
// (ties count against the point). The colleague measured a median rank of
// 18 and 78 % of points among the 50 most visible bumps for his best view.
//
// Gated on GOJACV_TILES (default ~/dustdet/cv_tiles) and GOJACV_GOLD
// (default ~/dustds/gold); labels are {tile}.final.json or {tile}.p1.json
// (as the colleague's script read them, whether complete or not), points of
// type dust, hair or scratch in absolute raw-scan coordinates, converted
// with the tile's origin_x, origin_y.
func TestRealViewRanking(t *testing.T) {
	tiles := tilesDir(t)
	goldDir := os.Getenv("GOJACV_GOLD")
	if goldDir == "" {
		home, _ := os.UserHomeDir()
		goldDir = filepath.Join(home, "dustds", "gold")
	}
	labels, _ := filepath.Glob(filepath.Join(goldDir, "*.p1.json"))
	finals, _ := filepath.Glob(filepath.Join(goldDir, "*.final.json"))
	if len(labels)+len(finals) == 0 {
		t.Skipf("no gold labels in %s (set GOJACV_GOLD)", goldDir)
	}
	ids := map[string]bool{}
	for _, f := range append(labels, finals...) {
		b := filepath.Base(f)
		ids[strings.TrimSuffix(strings.TrimSuffix(b, ".p1.json"), ".final.json")] = true
	}
	var ranks []int
	nTiles := 0
	for id := range ids {
		tif := filepath.Join(tiles, id+".tif")
		if _, err := os.Stat(tif); err != nil {
			continue
		}
		pts := goldPoints(t, goldDir, id)
		if len(pts) == 0 {
			continue
		}
		var meta struct {
			X float64 `json:"origin_x"`
			Y float64 `json:"origin_y"`
		}
		raw, err := os.ReadFile(filepath.Join(tiles, id+".json"))
		if err != nil || json.Unmarshal(raw, &meta) != nil {
			continue
		}
		nTiles++
		L := speckLook(t, tif)
		peaks := bumpPeaks(L)
		for _, p := range pts {
			x, y := p.X-meta.X, p.Y-meta.Y
			rr := math.Min(p.R, 12) + 2
			pk, ok := peakNear(L, x, y, rr)
			if !ok {
				continue
			}
			n := 0
			for _, v := range peaks {
				if v >= pk-0.5 {
					n++
				}
			}
			ranks = append(ranks, max(n, 1))
		}
	}
	if len(ranks) == 0 {
		t.Skip("no labelled tile has its tile file")
	}
	sort.Ints(ranks)
	median := ranks[len(ranks)/2]
	top50 := 0
	for _, r := range ranks {
		if r <= 50 {
			top50++
		}
	}
	frac := float64(top50) / float64(len(ranks))
	t.Logf("speck view: %d points on %d tiles, median rank %d, top50 %.1f %%", len(ranks), nTiles, median, 100*frac)
	if median > 30 || frac < 0.70 {
		t.Fatalf("speck view ranks labelled specks too late: median %d (want ≤ 30), top50 %.1f %% (want ≥ 70 %%)", median, 100*frac)
	}
}

type goldPoint struct {
	X, Y, R float64
	Type    string
}

func goldPoints(t *testing.T, dir, id string) []goldPoint {
	t.Helper()
	for _, suffix := range []string{"final", "p1"} {
		raw, err := os.ReadFile(filepath.Join(dir, id+"."+suffix+".json"))
		if err != nil {
			continue
		}
		var a struct {
			Points []struct {
				X, Y, R float64
				Type    string
			} `json:"points"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		var out []goldPoint
		for _, p := range a.Points {
			if p.Type == "dust" || p.Type == "hair" || p.Type == "scratch" {
				r := p.R
				if r == 0 {
					r = 3
				}
				out = append(out, goldPoint{p.X, p.Y, r, p.Type})
			}
		}
		return out
	}
	return nil
}

// speckLook renders img.view("speck") as a look (maxSide 1024, as the
// colleague's script did) and returns its 8-bit luminance.
func speckLook(t *testing.T, tif string) [][]float64 {
	t.Helper()
	e := graph.NewEngine(NewRegistry(), graph.EngineOptions{})
	img, err := e.Open(tif, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw := []any{"speck"}
	if sp := os.Getenv("GOJACV_VIEW_SPACE"); sp != "" { // experiments: the same view in another space
		raw = append(raw, map[string]any{"space": sp})
	}
	cr, err := e.Call("view", img.ID, raw)
	if err != nil {
		t.Fatal(err)
	}
	l, err := look.New(e, cr.Node().ID, look.Options{MaxSide: 1024})
	if err != nil {
		t.Fatal(err)
	}
	data, _, err := l.Render(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	m, _, err := stdimage.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	b := m.Bounds()
	L := make([][]float64, b.Dy())
	for y := range L {
		L[y] = make([]float64, b.Dx())
		for x := range L[y] {
			r, g, bl, _ := m.At(b.Min.X+x, b.Min.Y+y).RGBA()
			L[y][x] = 0.2126*float64(r>>8) + 0.7152*float64(g>>8) + 0.0722*float64(bl>>8)
		}
	}
	return L
}

// bumpPeaks returns the peak of every bump: 8-connected plateaus of pixels
// that are the maximum of their 5 × 5 neighbourhood and above 0.
func bumpPeaks(L [][]float64) []float64 {
	h, w := len(L), len(L[0])
	isMax := make([][]bool, h)
	for y := 0; y < h; y++ {
		isMax[y] = make([]bool, w)
		for x := 0; x < w; x++ {
			v := L[y][x]
			if v <= 0 {
				continue
			}
			m := v
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					yy, xx := min(max(y+dy, 0), h-1), min(max(x+dx, 0), w-1)
					m = math.Max(m, L[yy][xx])
				}
			}
			isMax[y][x] = v >= m
		}
	}
	seen := make([][]bool, h)
	for y := range seen {
		seen[y] = make([]bool, w)
	}
	var peaks []float64
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !isMax[y][x] || seen[y][x] {
				continue
			}
			pk := 0.0
			stack := [][2]int{{x, y}}
			seen[y][x] = true
			for len(stack) > 0 {
				p := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				pk = math.Max(pk, L[p[1]][p[0]])
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						xx, yy := p[0]+dx, p[1]+dy
						if xx >= 0 && yy >= 0 && xx < w && yy < h && isMax[yy][xx] && !seen[yy][xx] {
							seen[yy][xx] = true
							stack = append(stack, [2]int{xx, yy})
						}
					}
				}
			}
			peaks = append(peaks, pk)
		}
	}
	return peaks
}

// peakNear is the largest value within distance rr of (x, y) (pixel centres).
func peakNear(L [][]float64, x, y, rr float64) (float64, bool) {
	h, w := len(L), len(L[0])
	best, ok := 0.0, false
	for yy := max(0, int(y-rr-1)); yy < min(h, int(y+rr+2)); yy++ {
		for xx := max(0, int(x-rr-1)); xx < min(w, int(x+rr+2)); xx++ {
			if math.Hypot(float64(xx)+0.5-x, float64(yy)+0.5-y) <= rr {
				if !ok || L[yy][xx] > best {
					best, ok = L[yy][xx], true
				}
			}
		}
	}
	return best, ok
}
