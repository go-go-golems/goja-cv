package look

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/ops"
)

var testTIFF = filepath.Join("..", "..", "..", "testdata", "gradient-64x48-16bit.tif")

func newEngine(t *testing.T, path string) (*graph.Engine, *graph.Node) {
	t.Helper()
	e := graph.NewEngine(ops.NewRegistry(), graph.EngineOptions{})
	img, err := e.Open(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	return e, img
}

func call(t *testing.T, e *graph.Engine, name string, recv graph.NodeID, raw ...any) *graph.Node {
	t.Helper()
	n, _, err := e.Call(name, recv, raw)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return n
}

// writeFlat writes a w×h flat sRGB-grey PNG and returns its path.
func writeFlat(t *testing.T, w, h int, v float32) string {
	t.Helper()
	b := buffer.New(w, h, 3, 1)
	for i := range b.Pix {
		b.Pix[i] = v
	}
	p := filepath.Join(t.TempDir(), "flat.png")
	enc, err := buffer.Convert(context.Background(), b, buffer.EncSRGB, buffer.SpaceLinear, buffer.SpaceEncoded)
	if err != nil {
		t.Fatal(err)
	}
	if err := buffer.WriteFile(context.Background(), p, enc, "png8"); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLookCaptionScaleAndSize(t *testing.T) {
	e, img := newEngine(t, writeFlat(t, 3200, 1600, 0.2))
	l, err := New(e, img.ID, Options{Label: "before"})
	if err != nil {
		t.Fatal(err)
	}
	if w, h := l.Size(); w != 1568 || h != 784 {
		t.Fatalf("size %d×%d, want 1568×784", w, h)
	}
	if got, want := l.Caption(), "look: before (1568×784, 1 image px = 2.04 doc px)"; got != want {
		t.Fatalf("caption %q, want %q", got, want)
	}
	l.AssignNumber(3)
	l.AssignNumber(9) // first number wins
	if got, want := l.ResultPlaceholder(), "look 3: before (1568×784, 1 image px = 2.04 doc px)"; got != want {
		t.Fatalf("placeholder %q, want %q", got, want)
	}

	small, err := New(e, img.ID, Options{MaxSide: 400})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := small.Caption(), "look: open(flat.png) (400×200, 1 image px = 8.0 doc px)"; got != want {
		t.Fatalf("caption %q, want %q", got, want)
	}
	items, err := small.ResultContent(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].MIMEType != "image/jpeg" {
		t.Fatalf("items %+v", items)
	}
	m, err := jpeg.Decode(bytes.NewReader(items[0].Data))
	if err != nil {
		t.Fatal(err)
	}
	if b := m.Bounds(); b.Dx() != 400 || b.Dy() != 200 {
		t.Fatalf("decoded %v", b)
	}

	if _, err := New(e, img.ID, Options{MaxSide: 4000}); err == nil || !strings.Contains(err.Error(), graph.EArgRange) {
		t.Fatalf("maxSide 4000: %v", err)
	}
}

func TestLookBurnsNumberIntoGrayPNG(t *testing.T) {
	e, img := newEngine(t, testTIFF)
	gray := call(t, e, "toGray", img.ID)
	l, err := New(e, gray.ID, Options{Label: "gray"})
	if err != nil {
		t.Fatal(err)
	}
	l.AssignNumber(7)
	data, mime, err := l.Render(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if mime != "image/png" {
		t.Fatalf("mime %s, want image/png for a single-channel look", mime)
	}
	m, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	g, ok := m.(*image.Gray)
	if !ok {
		t.Fatalf("decoded %T", m)
	}
	// Read the digit back from the font grid.
	s := GlyphScale(64, 48)
	box := NumberBox(7, 64, 48)
	for row := 0; row < glyphH; row++ {
		for col := 0; col < glyphW; col++ {
			x, y := s+col*s+s/2, s+row*s+s/2
			on := g.GrayAt(x, y).Y == 255
			if want := GlyphPixel(7, col, row); on != want {
				t.Fatalf("glyph pixel (%d,%d) at (%d,%d) = %v, want %v", col, row, x, y, on, want)
			}
			if !on && g.GrayAt(x, y).Y != 0 {
				t.Fatalf("box pixel (%d,%d) = %d, want black", x, y, g.GrayAt(x, y).Y)
			}
		}
	}
	// Outside the box the image is the plain gray rendering.
	shown, err := e.Display(gray.ID)
	if err != nil {
		t.Fatal(err)
	}
	v, err := e.Evaluate(context.Background(), shown, graph.EvalOptions{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	ref, _ := buffer.Quantize(context.Background(), v.(*graph.ImageValue).Buf, 8)
	for _, p := range []image.Point{{box.Max.X, 0}, {0, box.Max.Y}, {63, 47}, {30, 30}} {
		if got, want := g.GrayAt(p.X, p.Y), ref.(*image.Gray).GrayAt(p.X, p.Y); got != want {
			t.Fatalf("pixel %v = %v, want %v", p, got, want)
		}
	}
}

func TestLookColourIsJPEGWithNumber(t *testing.T) {
	e, img := newEngine(t, writeFlat(t, 320, 200, 0.5))
	l, _ := New(e, img.ID, Options{})
	l.AssignNumber(12)
	data, mime, err := l.Render(context.Background())
	if err != nil || mime != "image/jpeg" {
		t.Fatalf("render: %v %s", err, mime)
	}
	m, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	lum := func(x, y int) uint8 { return color.GrayModel.Convert(m.At(x, y)).(color.Gray).Y }
	s := GlyphScale(320, 200)
	if v := lum(s/2, s/2); v > 40 {
		t.Fatalf("box corner luminance %d, want dark", v)
	}
	// "1": the middle column of the first digit is lit in every row.
	if v := lum(s+2*s+s/2, s+3*s+s/2); v < 200 {
		t.Fatalf("digit pixel luminance %d, want bright", v)
	}
	if v := lum(300, 180); v < 150 || v > 230 {
		t.Fatalf("background luminance %d, want the flat grey", v)
	}
}

func TestLookKindsAndChart(t *testing.T) {
	e, img := newEngine(t, testTIFF)
	stats := call(t, e, "stats", img.ID)
	if _, err := New(e, stats.ID, Options{}); err == nil || !strings.Contains(err.Error(), graph.EWrongKind) {
		t.Fatalf("stats look: %v", err)
	}
	hist := call(t, e, "histogram", img.ID)
	l, err := New(e, hist.ID, Options{Label: "hist"})
	if err != nil {
		t.Fatal(err)
	}
	l.AssignNumber(1)
	if got := l.Caption(); got != "look 1: hist (512×204 chart)" {
		t.Fatalf("caption %q", got)
	}
	data, mime, err := l.Render(context.Background())
	if err != nil || mime != "image/png" {
		t.Fatalf("chart: %v %s", err, mime)
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
}

func TestLookMarshalsWithoutBytes(t *testing.T) {
	e, img := newEngine(t, testTIFF)
	l, _ := New(e, img.ID, Options{Label: "x"})
	js, err := json.Marshal(l) //nolint:staticcheck // SA9005 is the point: Look must marshal to {} (no image bytes)
	if err != nil {
		t.Fatal(err)
	}
	if string(js) != "{}" {
		t.Fatalf("json %s, want {}", js)
	}
}
