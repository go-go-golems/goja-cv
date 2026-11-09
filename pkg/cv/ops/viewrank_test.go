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
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/look"
)

// TestRealViewRanking ports the colleague's view metric (DUST-002
// scripts/85-cv-views.py; GCV-002 guide §13.4) to the speck preset: for each
// labelled tile, render the view as the look a vision model gets, find all
// local maxima (5 × 5, a plateau is one bump) on its luminance, and rank
// each labelled point's peak (within r + 2 px, r ≤ 12) among them (ties
// count against the point). The labels change as the colleague annotates,
// so the test compares views instead of asserting today's ranks: the speck
// view must rank labelled specks earlier than the plain stretched tile
// (levels({ auto: true })).
//
// Gated on GOJACV_TILES (default ~/dustdet/cv_tiles) and GOJACV_GOLD
// (default ~/dustds/gold); labels are {tile}.final.json or {tile}.p1.json,
// points of type dust, hair or scratch in absolute raw-scan coordinates,
// converted with the tile's origin_x, origin_y. Tiles are opened with
// encoding "linear" (their code value is density, DUST-002 script 82).
func TestRealViewRanking(t *testing.T) {
	type rank struct {
		median int
		top50  float64
		n      int
	}
	views := map[string]func(e *graph.Engine, img *graph.Node) *graph.Node{
		"speck": func(e *graph.Engine, img *graph.Node) *graph.Node {
			cr, err := e.Call("view", img.ID, []any{"speck"})
			if err != nil {
				t.Fatal(err)
			}
			return cr.Node()
		},
		"levels": func(e *graph.Engine, img *graph.Node) *graph.Node {
			cr, err := e.Call("levels", img.ID, []any{map[string]any{"auto": true}})
			if err != nil {
				t.Fatal(err)
			}
			return cr.Node()
		},
	}
	res := map[string]rank{}
	tiles := labelledTiles(t)
	for name, view := range views {
		var ranks []int
		for _, lt := range tiles {
			L := viewLook(t, lt.tif, view)
			peaks := bumpPeaks(L)
			for _, p := range lt.pts {
				pk, ok := peakNear(L, p.X-lt.ox, p.Y-lt.oy, math.Min(p.R, 12)+2)
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
			t.Skip("no labelled point lies in its tile")
		}
		sort.Ints(ranks)
		top := 0
		for _, r := range ranks {
			if r <= 50 {
				top++
			}
		}
		res[name] = rank{ranks[len(ranks)/2], float64(top) / float64(len(ranks)), len(ranks)}
		t.Logf("%s view: %d points on %d tiles, median rank %d, top50 %.1f %%", name, len(ranks), len(tiles), res[name].median, 100*res[name].top50)
	}
	sp, lv := res["speck"], res["levels"]
	if sp.median >= lv.median || sp.top50 <= lv.top50 {
		t.Fatalf("the speck view ranks labelled specks no earlier than levels({ auto: true }): median %d vs %d, top50 %.1f %% vs %.1f %%",
			sp.median, lv.median, 100*sp.top50, 100*lv.top50)
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

// viewLook renders a view of a tile (opened with encoding "linear") as a
// look (maxSide 1024, as the colleague's script did) and returns its 8-bit
// luminance.
func viewLook(t *testing.T, tif string, view func(e *graph.Engine, img *graph.Node) *graph.Node) [][]float64 {
	t.Helper()
	e := graph.NewEngine(NewRegistry(), graph.EngineOptions{})
	l, err := look.New(e, view(e, openTile(t, e, tif)).ID, look.Options{MaxSide: 1024})
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
