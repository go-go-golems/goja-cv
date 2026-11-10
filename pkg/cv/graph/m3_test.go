package graph

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
)

// m3Engine extends the test registry with a mask maker, a colour maker, a
// Regions maker
// with an analysis scale, a kind-typed option, a box arg and a validator.
func m3Engine(t *testing.T) *Engine {
	t.Helper()
	r, _ := testRegistry(t)
	r.MustRegister(
		OpDef{Name: "rgb", On: KindImage, Returns: KindImage, Channels: Three,
			Run: func(_ context.Context, _ *EvalCtx, in []Value, _ Args) (Value, error) {
				src := in[0].(*ImageValue)
				out := buffer.NewLikeC(src.Buf, 3)
				for c := 0; c < 3; c++ {
					copy(out.Plane(c), src.Buf.Plane(0))
				}
				return NewImage(out), nil
			}},
		OpDef{Name: "mask.half", On: KindImage, Returns: KindImage, Channels: One,
			Run: func(_ context.Context, ec *EvalCtx, in []Value, _ Args) (Value, error) {
				src := in[0].(*ImageValue)
				m := buffer.New(src.Buf.W, src.Buf.H, 1, ec.Scale)
				for y := 0; y < m.H; y++ {
					for x := 0; x < m.W/2; x++ {
						m.Pix[y*m.W+x] = 1
					}
				}
				return NewImage(m), nil
			}},
		OpDef{Name: "blobs", On: KindImage, Returns: KindRegions,
			Args: []ArgSpec{
				{Name: "box", Type: "number[]", Unit: DocPx, Len: 4, Positional: true},
				{Name: "minArea", Type: "number", Unit: DocPx2, Default: 0.0},
				{Name: "analysisScale", Type: "number", Default: 1.0, Min: F(0.01), Max: F(1)},
			},
			AnalysisScale: func(a Args) float64 { return a.Raw("analysisScale").(float64) },
			Run: func(_ context.Context, ec *EvalCtx, in []Value, a Args) (Value, error) {
				src := in[0].(*ImageValue)
				rv := NewRegions(src.Buf.W, src.Buf.H, ec.Scale, src.Frame)
				b := a.Lengths("box", ec.Scale)
				var pix []int32
				for y := int(b[1]); y < int(b[1]+b[3]); y++ {
					for x := int(b[0]); x < int(b[0]+b[2]); x++ {
						pix = append(pix, int32(y*src.Buf.W+x))
					}
				}
				if float64(len(pix)) >= a.Area("minArea", ec.Scale) {
					rv.Items = append(rv.Items, rv.NewRegion(1, pix))
				}
				return rv, nil
			}},
		OpDef{Name: "paint", On: KindImage, Returns: KindImage,
			Args: []ArgSpec{{Name: "mask", Type: "Mask", Optional: true}, {Name: "v", Type: "number", Default: 1.0}},
			Run: func(_ context.Context, _ *EvalCtx, in []Value, a Args) (Value, error) {
				src := in[0].(*ImageValue)
				out := src.Buf.Clone()
				if m, ok := a.Value("mask").(*ImageValue); ok {
					for i, w := range m.Buf.Pix {
						out.Pix[i] = out.Pix[i]*(1-w) + float32(a.Number("v"))*w
					}
				}
				return NewImage(out), nil
			}},
		OpDef{Name: "pred", On: KindImage, Returns: KindValue,
			Args: []ArgSpec{{Name: "p", Type: "any", Positional: true}},
			Validate: func(a Args) error {
				if _, ok := a.Raw("p").(map[string]any); !ok {
					return Errorf(EArgType, "pred needs an object")
				}
				return nil
			},
			Run: func(_ context.Context, _ *EvalCtx, _ []Value, a Args) (Value, error) {
				return &DataValue{Data: a.Raw("p")}, nil
			}},
	)
	return NewEngine(r, EngineOptions{})
}

func TestKindTypedArgsStaticLenValidate(t *testing.T) {
	e := m3Engine(t)
	src := mustCall(t, e, "fill", 0, 0.0)
	m := mustCall(t, e, "mask.half", src.ID)
	if m.Kind != KindImage || m.Channels != 1 {
		t.Fatal(m.Kind, m.Channels)
	}
	p := mustCall(t, e, "paint", src.ID, map[string]any{"mask": NodeArg{m.ID}})
	if len(p.Inputs) != 2 || !p.HasReceiver() {
		t.Fatalf("kind-typed arg not lifted: %v", p.Inputs)
	}
	v, err := e.Evaluate(context.Background(), p.ID, EvalOptions{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	b := v.(*ImageValue).Buf
	if b.Pix[0] != 1 || b.Pix[b.W-1] != 0 {
		t.Fatalf("paint through mask: %v %v", b.Pix[0], b.Pix[b.W-1])
	}
	// any one-channel image is a mask; a colour image is refused with the
	// toGray fix, regions with the mask.regions fix
	if _, err := e.Call("paint", src.ID, []any{map[string]any{"mask": NodeArg{src.ID}}}); err != nil {
		t.Fatalf("a grey image as mask: %v", err)
	}
	col := mustCall(t, e, "rgb", src.ID)
	_, err = e.Call("paint", src.ID, []any{map[string]any{"mask": NodeArg{col.ID}}})
	if ce := codeOf(t, err); ce.Code != EWrongKind || !strings.Contains(ce.Message, "one-channel image (a mask); this image has 3 channels") || !strings.HasSuffix(ce.Fix, ".toGray()") {
		t.Fatalf("colour image as mask: %+v", ce)
	}
	rg := mustCall(t, e, "blobs", src.ID, []any{0.0, 0.0, 4.0, 4.0})
	_, err = e.Call("paint", src.ID, []any{map[string]any{"mask": NodeArg{rg.ID}}})
	if ce := codeOf(t, err); ce.Code != EWrongKind || ce.Fix != "img.mask.regions(regions)" {
		t.Fatalf("regions as mask: %+v", ce)
	}
	_, err = e.Call("paint", src.ID, []any{map[string]any{"mask": 0.5}})
	if ce := codeOf(t, err); ce.Code != EWrongKind {
		t.Fatalf("number as mask: %+v", ce)
	}
	// Len
	_, err = e.Call("blobs", src.ID, []any{[]any{1, 2, 3}})
	if ce := codeOf(t, err); ce.Code != EArgType || !strings.Contains(ce.Hint, "[x, y, w, h]") {
		t.Fatalf("box len: %+v", ce)
	}
	// options are literal values
	_, err = e.Call("blobs", src.ID, []any{[]any{0, 0, 4, 4}, map[string]any{"analysisScale": NodeArg{m.ID}}})
	if ce := codeOf(t, err); ce.Code != EArgType || !strings.Contains(ce.Message, "literal") {
		t.Fatalf("literal: %+v", ce)
	}
	// Validate runs at build time
	_, err = e.Call("pred", src.ID, []any{3})
	if ce := codeOf(t, err); ce.Code != EArgType {
		t.Fatalf("validate: %+v", ce)
	}
}

// TestAnalysisScaleMakesRegionsScaleFree: a Regions op with an analysis
// scale evaluates its inputs at that scale whatever the evaluation scale, so
// the regions (doc px) are identical at preview and at 1.0, and computed once.
func TestAnalysisScaleMakesRegionsScaleFree(t *testing.T) {
	e := m3Engine(t)
	src := mustCall(t, e, "fill", 0, 0.0, map[string]any{"w": 400, "h": 200})
	bl := mustCall(t, e, "blobs", src.ID, []any{10, 20, 30, 40}, map[string]any{"minArea": 100})
	var first *RegionsValue
	for _, sc := range []float64{0.25, 0.5, 1} {
		v, err := e.Evaluate(context.Background(), bl.ID, EvalOptions{Scale: sc})
		if err != nil {
			t.Fatal(err)
		}
		rv := v.(*RegionsValue)
		if len(rv.Items) != 1 || rv.Scale != 1 {
			t.Fatalf("at %g: %d items, raster scale %g", sc, len(rv.Items), rv.Scale)
		}
		if first == nil {
			first = rv
		} else if rv.Items[0].Box != first.Items[0].Box || rv.Items[0].Area != first.Items[0].Area {
			t.Fatalf("at %g: %v vs %v", sc, rv.Items[0].Box, first.Items[0].Box)
		}
	}
	if got := first.Items[0].Box; got != [4]float64{10, 20, 30, 40} || first.Items[0].Area != 1200 {
		t.Fatalf("box %v area %v", got, first.Items[0].Area)
	}
	if e.Evaluator().Runs(bl.ID) != 1 || e.Evaluator().Runs(src.ID) != 1 {
		t.Fatalf("runs blobs=%d src=%d", e.Evaluator().Runs(bl.ID), e.Evaluator().Runs(src.ID))
	}
	// at analysis scale 0.5 the box is found in a 200×100 raster, still in doc px
	half := mustCall(t, e, "blobs", src.ID, []any{10, 20, 30, 40}, map[string]any{"analysisScale": 0.5})
	v, err := e.Evaluate(context.Background(), half.ID, EvalOptions{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	if rv := v.(*RegionsValue); rv.W != 200 || rv.Items[0].Box != [4]float64{10, 20, 30, 40} {
		t.Fatalf("half: %dx%d %v", rv.W, rv.H, rv.Items[0].Box)
	}
	lines := e.Explain(half.ID)
	if !strings.Contains(lines[0], "(analysed at scale 0.5)") || !strings.Contains(lines[1], "Image 200×100") {
		t.Fatalf("explain: %q", lines)
	}
}

// TestRasterizeKeepsAreaAtAnyScale: regions found at one scale rasterize to
// masks whose sum (in target px²) is the doc area times scale².
func TestRasterizeKeepsAreaAtAnyScale(t *testing.T) {
	rv := NewRegions(100, 60, 1, Frame{W: 100, H: 60})
	var pix []int32
	for y := 7; y < 20; y++ {
		for x := 13; x < 30; x++ {
			pix = append(pix, int32(y*100+x))
		}
	}
	rv.Items = []*Region{rv.NewRegion(1, pix)}
	area := rv.Items[0].Area
	for _, sc := range []float64{1, 0.5, 0.37, 0.25, 0.1} {
		w, h := buffer.ScaledSize(100, 60, sc)
		m, err := rv.Rasterize(context.Background(), FrameRaster(rv.Frame, sc))
		if err != nil {
			t.Fatal(err)
		}
		var s float64
		for _, p := range m.Pix {
			if p < 0 || p > 1 {
				t.Fatalf("value %v outside [0,1]", p)
			}
			s += float64(p)
		}
		want := area * float64(w) / 100 * float64(h) / 60
		if math.Abs(s-want) > 1e-3*want {
			t.Fatalf("scale %g: rasterized area %g, want %g", sc, s, want)
		}
	}
	d := rv.Items[0].Data(rv.Schema()).Vals
	if d["area"] != 221.0 || d["aspect"].(float64) < 1 {
		t.Fatalf("data %v", d)
	}
	if s, err := rv.Records(); err != nil || len(s) != 1 {
		t.Fatalf("records %v", s)
	}
}

func TestAwaitingTooManyRegionsIsAnError(t *testing.T) {
	rv := NewRegions(1, 1, 1, Frame{W: 1, H: 1})
	rv.Items = make([]*Region, MaxRegionItems+1)
	for i := range rv.Items {
		rv.Items[i] = &Region{ID: i + 1}
	}
	_, err := rv.Records()
	if ce, ok := AsError(err); !ok || ce.Code != ETooMany || !strings.Contains(ce.Fix, "minArea") || !strings.Contains(ce.Message, "100001 regions") {
		t.Fatalf("summary of %d regions: %v", len(rv.Items), err)
	}
	rv.Items = rv.Items[:MaxRegionItems]
	if s, err := rv.Records(); err != nil || len(s) != MaxRegionItems {
		t.Fatalf("summary at the limit: %v", err)
	}
}

// TestCacheWarningsLeaveWithTheirEntry: evaluation-time warnings are kept
// with the cached value and dropped when it is evicted or too large.
func TestCacheWarningsLeaveWithTheirEntry(t *testing.T) {
	c := NewCache(100)
	v := &DataValue{Data: 1.0} // Bytes() > 0
	n := v.Bytes()
	k := func(i byte) Signature { return Signature{i} }
	w := []Warning{{Code: "X", Message: "m"}}
	c.SetWarnings(k(1), w)
	if !c.Put(k(1), v) || len(c.Warnings(k(1))) != 1 {
		t.Fatal("warnings not kept with their value")
	}
	for i := byte(2); int64(i-1)*n <= 100; i++ {
		c.SetWarnings(k(i), w)
		c.Put(k(i), v)
	}
	if c.Contains(k(1)) || c.Warnings(k(1)) != nil {
		t.Fatal("an evicted value kept its warnings")
	}
	big := NewCache(1)
	big.SetWarnings(k(9), w)
	if big.Put(k(9), v) || big.Warnings(k(9)) != nil {
		t.Fatal("a value too large for the cache kept its warnings")
	}
	if len(c.warns) > len(c.items) {
		t.Fatalf("%d warning entries for %d values", len(c.warns), len(c.items))
	}
}
