package look

import (
	"context"
	"image"
	"strconv"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/text"
)

func TestTicksAndLabelThinning(t *testing.T) {
	x := axis{origin: 300, scale: 8, n: 256}
	// ruler 8 at zoom 8: ticks 64 px apart, every one labelled
	ts := ticks(x, 8, text.Width("300", labelPx)+labelGap)
	if len(ts) != 5 || ts[0].doc != 300 || ts[0].px != 0 || ts[1].px != 64 || ts[4].doc != 332 || !ts[1].labeled {
		t.Fatalf("ruler 8: %+v", ts)
	}
	// ruler 1 at zoom 8: ticks 8 px apart; labels 30 px wide need 34 px: every fifth
	ts = ticks(x, 1, text.Width("300", labelPx)+labelGap)
	var lab []int
	for _, tk := range ts {
		if tk.px != (tk.doc-300)*8 {
			t.Fatalf("tick %d at px %d", tk.doc, tk.px)
		}
		if tk.labeled {
			lab = append(lab, tk.doc)
		}
	}
	if len(ts) != 33 || strings.Trim(strings.Join(strings.Fields(fmtInts(lab)), ","), "[]") != "300,305,310,315,320,325,330" {
		t.Fatalf("ruler 1: %d ticks, labelled %v", len(ts), lab)
	}
	if got := autoTick(8); got != 8 {
		t.Fatalf("auto tick at zoom 8: %g, want 8 (64 px)", got)
	}
	if got := autoTick(0.25); got != 256 {
		t.Fatalf("auto tick at 0.25: %g, want 256", got)
	}
}

func fmtInts(xs []int) string {
	s := make([]string, len(xs))
	for i, x := range xs {
		s[i] = strconv.Itoa(x)
	}
	return "[" + strings.Join(s, " ") + "]"
}

// readNumber reads the digits drawn at font size labelPx starting at (x, y)
// by matching glyphs.
func readNumber(m image.Image, x, y int) string {
	out := ""
	for {
		d := ""
		for _, r := range "0123456789" {
			ok := true
			for row := 0; row < text.Height(1) && ok; row++ {
				for col := 0; col < text.Width("0", 1) && ok; col++ {
					rr, _, _, _ := m.At(x+col*labelPx+labelPx/2, y+row*labelPx+labelPx/2).RGBA()
					ok = (rr>>8 > 128) == text.Pixel(r, col, row)
				}
			}
			if ok {
				d = string(r)
				break
			}
		}
		if d == "" {
			return out
		}
		out += d
		x += text.Width("0", labelPx)
	}
}

func TestRulerLabelsReadBackAbsoluteCoordinates(t *testing.T) {
	ctx := context.Background()
	e, img := newEngine(t, writeFlat(t, 512, 384, 0.1))
	c := call(t, e, "crop", img.ID, []any{300.0, 100.0, 32.0, 32.0})
	l, err := New(e, c.ID, Options{Zoom: 8, Ruler: 8})
	if err != nil {
		t.Fatal(err)
	}
	if got := l.Caption(); got != "look: open(flat.png) → crop[300,100,32,32] (256×256 + ruler, 1 image px = 0.125 doc px, doc x 300–332, y 100–132)" {
		t.Fatalf("caption %q", got)
	}
	m, err := l.image(ctx)
	if err != nil {
		t.Fatal(err)
	}
	lay := layoutRulers(axis{origin: 300, scale: 8, n: 256}, axis{origin: 100, scale: 8, n: 256}, 8)
	if b := m.Bounds(); b.Dx() != 256+lay.left+lay.right || b.Dy() != 256+lay.top+lay.bottom || lay.top != 28 || lay.bottom != 28 {
		t.Fatalf("ruled look %v, layout %+v", b, lay)
	}
	lh := text.Height(labelPx)
	for _, tk := range lay.xt {
		s := strconv.Itoa(tk.doc)
		w := text.Width(s, labelPx)
		x0 := min(max(lay.left+tk.px-w/2, lay.left), m.Bounds().Dx()-w)
		if got := readNumber(m, x0, lay.top-tickLen-2-lh); got != s {
			t.Fatalf("x label at tick %d reads %q", tk.doc, got)
		}
	}
	for _, tk := range lay.yt {
		s := strconv.Itoa(tk.doc)
		w := text.Width(s, labelPx)
		y0 := min(max(lay.top+tk.px-lh/2, lay.top), m.Bounds().Dy()-lh)
		if got := readNumber(m, lay.left-tickLen-2-w, y0); got != s {
			t.Fatalf("y label at tick %d reads %q", tk.doc, got)
		}
	}
	// the tick at doc x 308 is the left edge of doc pixel 308
	if r, _, _, _ := m.At(lay.left+8*8, lay.top-1).RGBA(); r>>8 != 225 {
		t.Fatalf("tick mark at 308 missing")
	}
	// zoom and ruler refuse charts
	h := call(t, e, "histogram", img.ID)
	if _, err := New(e, h.ID, Options{Ruler: -1}); err == nil {
		t.Fatal("ruler on a chart")
	}
}

func TestChainCaptions(t *testing.T) {
	e, img := newEngine(t, testTIFF)
	g := call(t, e, "gaussian", call(t, e, "exposure", img.ID, 0.5).ID, 2.0)
	l, _ := New(e, g.ID, Options{})
	if got := l.Caption(); got != "look: open(gradient-64x48-16bit.tif) → exposure(0.5) → gaussian(2) (64×48, 1 image px = 1.0 doc px)" {
		t.Fatalf("plain chain: %q", got)
	}
	n := g
	for i := 0; i < 12; i++ {
		n = call(t, e, "exposure", n.ID, 0.01*float64(i+1))
	}
	l, _ = New(e, n.ID, Options{Label: "long"})
	if got := l.Caption(); got != "look: long — open(gradient-64x48-16bit.tif) → … → exposure(0.1) → exposure(0.11) → exposure(0.12) (64×48, 1 image px = 1.0 doc px)" {
		t.Fatalf("long chain: %q", got)
	}
	if got := e.Chain(call(t, e, "stats", img.ID).ID); got != "open(gradient-64x48-16bit.tif) → stats" {
		t.Fatalf("value chain %q", got)
	}
	_ = graph.Label
}

// TestRulerLabelsKeepAGap: the label at the start of an axis is shifted
// inwards to stay inside the image span; a neighbour it would touch is
// dropped (a crop at x 1619 printed "16191651"). Every kept label reads back
// and adjacent labels are at least labelGap apart, on both axes.
func TestRulerLabelsKeepAGap(t *testing.T) {
	ctx := context.Background()
	e, img := newEngine(t, writeFlat(t, 1800, 64, 0.1))
	for _, tc := range []struct {
		name     string
		box      []any
		opts     Options
		wantX    string // the x labels kept, in order
		wantY    string
		x0, y0   float64
		sx       float64
		xSp, ySp float64
	}{
		// ruler: true at zoom 1.5 ticks every 32 doc px (48 image px): 1651 would start 12 px inside 1619
		{name: "field case", box: []any{1619.0, 10.0, 60.0, 40.0}, opts: Options{Zoom: 1.5, Ruler: -1}, wantX: "1619", wantY: "10,42", x0: 1619, y0: 10, sx: 1.5, xSp: 32},
		// ruler 2 at zoom 10: y ticks 20 px apart, labels 14 px tall; 12 would touch 10 (shifted down)
		// and 18 (shifted up at the bottom end) would touch 16
		{name: "dense y", box: []any{1619.0, 10.0, 8.0, 8.0}, opts: Options{Zoom: 10, Ruler: 2}, wantX: "1619", wantY: "10,14,16", x0: 1619, y0: 10, sx: 10, xSp: 2},
	} {
		c := call(t, e, "crop", img.ID, tc.box)
		l, err := New(e, c.ID, tc.opts)
		if err != nil {
			t.Fatal(err)
		}
		m, err := l.image(ctx)
		if err != nil {
			t.Fatal(err)
		}
		w, h := int(tc.box[2].(float64)*tc.sx), int(tc.box[3].(float64)*tc.sx)
		lay := layoutRulers(axis{origin: tc.x0, scale: tc.sx, n: w}, axis{origin: tc.y0, scale: tc.sx, n: h}, tc.xSp)
		width := func(s string) int { return text.Width(s, labelPx) }
		lh := text.Height(labelPx)
		xs := placeLabels(lay.xt, lay.left, lay.left, m.Bounds().Dx(), width)
		ys := placeLabels(lay.yt, lay.top, lay.top, m.Bounds().Dy(), func(string) int { return lh })
		var gotX, gotY []string
		for i, p := range xs {
			if got := readNumber(m, p.at, lay.top-tickLen-2-lh); got != p.text {
				t.Errorf("%s: x label %q reads %q", tc.name, p.text, got)
			}
			if i > 0 && p.at < xs[i-1].at+width(xs[i-1].text)+labelGap {
				t.Errorf("%s: x labels %q and %q are closer than %d px", tc.name, xs[i-1].text, p.text, labelGap)
			}
			gotX = append(gotX, p.text)
		}
		for i, p := range ys {
			if got := readNumber(m, lay.left-tickLen-2-width(p.text), p.at); got != p.text {
				t.Errorf("%s: y label %q reads %q", tc.name, p.text, got)
			}
			if i > 0 && p.at < ys[i-1].at+lh+labelGap {
				t.Errorf("%s: y labels %q and %q are closer than %d px", tc.name, ys[i-1].text, p.text, labelGap)
			}
			gotY = append(gotY, p.text)
		}
		if g := strings.Join(gotX, ","); g != tc.wantX {
			t.Errorf("%s: x labels %s, want %s", tc.name, g, tc.wantX)
		}
		if g := strings.Join(gotY, ","); g != tc.wantY {
			t.Errorf("%s: y labels %s, want %s", tc.name, g, tc.wantY)
		}
	}
}
