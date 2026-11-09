package ops

import (
	"context"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/internal/testimage"
	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph/graphtest"
)

func openErr(t *testing.T, e *graph.Engine, path string, opts map[string]any) *graph.Error {
	t.Helper()
	_, err := e.Open(path, opts)
	ce, ok := graph.AsError(err)
	if !ok {
		t.Fatalf("open %s %v: want a cv error, got %v", path, opts, err)
	}
	return ce
}

func mustOpen(t *testing.T, e *graph.Engine, path string, opts map[string]any) *graph.Node {
	t.Helper()
	n, err := e.Open(path, opts)
	if err != nil {
		t.Fatalf("open %s %v: %v", path, opts, err)
	}
	return n
}

func TestOpenEncodingsWindowsAndOrigins(t *testing.T) {
	ctx := context.Background()
	src, _ := testimage.PlantedScan() // linear values
	dir := t.TempDir()
	linTIF := filepath.Join(dir, "lin.tif")
	if err := testimage.WriteTIFF(linTIF, src, testimage.TIFFOptions{Bits: 16, Compression: "lzw", Predictor: true,
		Description: buffer.FormatMetadata(buffer.EncLinear, nil)}); err != nil {
		t.Fatal(err)
	}
	plainTIF := filepath.Join(dir, "plain.tif")
	if err := testimage.WriteFile(plainTIF, src, 16, buffer.Meta{}); err != nil {
		t.Fatal(err)
	}
	windowTIF := filepath.Join(dir, "window.tif")
	if err := testimage.WriteFile(windowTIF, src, 16, buffer.Meta{Encoding: buffer.EncLinear, Origin: &[2]int{1536, 1024}}); err != nil {
		t.Fatal(err)
	}
	dng := filepath.Join(dir, "scan.dng")
	if err := testimage.WriteDNG(dng, src, testimage.DNGOptions{}); err != nil {
		t.Fatal(err)
	}
	npy := filepath.Join(dir, "tile.npy")
	if err := testimage.WriteFile(npy, src, 32, buffer.Meta{}); err != nil {
		t.Fatal(err)
	}
	e := graph.NewEngine(NewRegistry(), graphtest.Options(t))

	// the file says linear: values are what is stored
	lin := mustOpen(t, e, linTIF, nil)
	if lin.Enc != buffer.EncLinear || lin.Space != buffer.SpaceEncoded || lin.Frame != (graph.Frame{W: 256, H: 192}) {
		t.Fatalf("linear tiff: %s %s", graph.Statics(lin), lin.Frame)
	}
	lv := evalValue(t, e, lin, 1).(*graph.ImageValue)
	for i := range src.Pix {
		if math.Abs(float64(lv.Buf.Pix[i]-src.Pix[i])) > 0.5/65535+1e-7 {
			t.Fatalf("linear tiff pixel %d: %g vs %g", i, lv.Buf.Pix[i], src.Pix[i])
		}
	}
	// the same codes read as sRGB (the TIFF default), or overridden to linear
	plain := mustOpen(t, e, plainTIF, nil)
	over := mustOpen(t, e, plainTIF, map[string]any{"encoding": "linear"})
	if plain.Enc != buffer.EncSRGB || over.Enc != buffer.EncLinear || plain.ID == over.ID {
		t.Fatalf("encoding override: %v %v", plain.Enc, over.Enc)
	}
	// both open in file units (the stored codes); in linear they differ
	pv, ov := evalValue(t, e, plain, 1).(*graph.ImageValue), evalValue(t, e, over, 1).(*graph.ImageValue)
	pl := evalValue(t, e, call(t, e, graph.AsOp, plain.ID, "linear"), 1).(*graph.ImageValue)
	if math.Abs(float64(ov.Buf.Pix[100]-lv.Buf.Pix[100])) > 1e-7 || pv.Buf.Pix[100] != ov.Buf.Pix[100] ||
		math.Abs(float64(pl.Buf.Pix[100])-buffer.SRGBToLinear(float64(ov.Buf.Pix[100]))) > 1e-6 {
		t.Fatalf("plain %g (linear %g), override %g, linear %g", pv.Buf.Pix[100], pl.Buf.Pix[100], ov.Buf.Pix[100], lv.Buf.Pix[100])
	}
	// the mask of a linear-encoded file thresholds the stored values: no conversion node
	m := call(t, e, "mask.luminance", lin.ID, map[string]any{"above": 0.5})
	if g, _ := e.Node(m.Inputs[len(m.Inputs)-1]); g.Op.Name != "toGray" || g.Inputs[0] != lin.ID { // the luminance input
		t.Fatal("mask.luminance on linear data must read toGray(source) directly")
	}

	// a window of a DNG keeps the file's coordinates
	box := []any{37.0, 21.0, 100.0, 80.0}
	win := mustOpen(t, e, dng, map[string]any{"box": box})
	if win.Frame != (graph.Frame{X: 37, Y: 21, W: 100, H: 80}) || win.Enc != buffer.EncLinear {
		t.Fatalf("dng window frame %s %v", win.Frame, win.Enc)
	}
	wv := evalValue(t, e, win, 1).(*graph.ImageValue)
	full := evalValue(t, e, mustOpen(t, e, dng, nil), 1).(*graph.ImageValue)
	for c := 0; c < 3; c++ {
		for y := 0; y < 80; y++ {
			for x := 0; x < 100; x++ {
				if wv.Buf.Plane(c)[y*100+x] != full.Buf.Plane(c)[(21+y)*256+37+x] {
					t.Fatalf("window pixel (%d, %d, %d) differs from the full decode", x, y, c)
				}
			}
		}
	}
	info, _ := graph.SourceInfo(win)
	if info["format"] != "dng" || info["encoding"] != "linear" || !strings.Contains(stringsOf(info["notes"]), "camera RGB primaries") {
		t.Fatalf("source info %v", info)
	}
	s := evalData(t, e, call(t, e, "stats", win.ID, map[string]any{"box": []any{50.0, 30.0, 10.0, 10.0}}), 1).(map[string]any)
	if s["area"] != 100.0 {
		t.Fatalf("stats in a window: %v", s["area"])
	}
	if ce := callErr(t, e, "stats", win.ID, map[string]any{"box": []any{0.0, 0.0, 10.0, 10.0}}); ce.Code != graph.EArgRange {
		t.Fatalf("box outside the window: %v", ce)
	}

	// origin declared by the call or recorded in the file
	o := mustOpen(t, e, npy, map[string]any{"origin": []any{1536.0, 1536.0}})
	if o.Frame != (graph.Frame{X: 1536, Y: 1536, W: 256, H: 192}) || o.Enc != buffer.EncLinear {
		t.Fatalf("npy with origin: %s %v", o.Frame, o.Enc)
	}
	rec := mustOpen(t, e, windowTIF, nil)
	if rec.Frame != (graph.Frame{X: 1536, Y: 1024, W: 256, H: 192}) || rec.Enc != buffer.EncLinear {
		t.Fatalf("recorded origin: %s", rec.Frame)
	}
	if n := mustOpen(t, e, windowTIF, map[string]any{"origin": []any{0.0, 0.0}}); n.Frame.X != 0 {
		t.Fatal("origin must override the recorded one")
	}
	if n := mustOpen(t, e, windowTIF, map[string]any{"box": []any{10.0, 10.0, 20.0, 20.0}}); n.Frame != (graph.Frame{X: 1546, Y: 1034, W: 20, H: 20}) {
		t.Fatalf("a window of a recorded window: %s", n.Frame)
	}
	if _, err := e.Evaluate(ctx, o.ID, graph.EvalOptions{Scale: 0.5}); err != nil {
		t.Fatal(err)
	}

	// errors
	if ce := openErr(t, e, dng, map[string]any{"box": box, "origin": []any{0.0, 0.0}}); ce.Code != graph.EArgType || !strings.Contains(ce.Message, "box already sets the origin") {
		t.Fatalf("box and origin: %+v", ce)
	}
	if ce := openErr(t, e, dng, map[string]any{"box": []any{200.0, 100.0, 100.0, 100.0}}); ce.Code != graph.EArgRange || !strings.Contains(ce.Fix, "[156, 92, 100, 100]") {
		t.Fatalf("box outside the file: %+v", ce)
	}
	if ce := openErr(t, e, dng, map[string]any{"encoding": "gamma"}); ce.Code != graph.EArgType ||
		!strings.Contains(ce.Message, `cv.open.encoding = "gamma" is not one of "srgb", "linear".`) || ce.Fix != `cv.open(path, { encoding: "srgb" })` {
		t.Fatalf("bad encoding: %+v", ce)
	}
	if ce := openErr(t, e, dng, map[string]any{"name": "scan"}); ce.Code != graph.EUnknownOption {
		t.Fatalf("as in the engine: %+v", ce)
	}
	cfa := filepath.Join(dir, "cfa.dng")
	if err := testimage.WriteDNG(cfa, src, testimage.DNGOptions{CFA: true}); err != nil {
		t.Fatal(err)
	}
	if ce := openErr(t, e, cfa, nil); ce.Code != graph.EFormat || !strings.Contains(ce.Message, "CFA (mosaiced) raw data is not supported; goja-cv reads LinearRaw DNGs") {
		t.Fatalf("CFA: %+v", ce)
	}
}

func stringsOf(v any) string {
	var out []string
	if l, ok := v.([]any); ok {
		for _, e := range l {
			s, _ := e.(string)
			out = append(out, s)
		}
	}
	return strings.Join(out, "; ")
}
