package ops

import (
	"math"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

func imgBuf(t *testing.T, e *graph.Engine, n *graph.Node) *buffer.Buffer {
	t.Helper()
	return evalValue(t, e, n, 1).(*graph.ImageValue).Buf
}

func TestSubIsSignedAndDerived(t *testing.T) {
	e, img, _ := plantedEngine(t)
	g := call(t, e, "gaussian", img.ID, 3.0, map[string]any{"space": "encoded"})
	d := call(t, e, "sub", img.ID, graph.NodeArg{ID: g.ID})
	if d.Enc != buffer.EncLinear || d.Space != buffer.SpaceLinear {
		t.Fatalf("sub statics %s/%s, want derived (linear/linear)", d.Enc, d.Space)
	}
	s := evalData(t, e, call(t, e, "stats", d.ID), 1).(map[string]any)
	if p01, p99 := s["p01"].(float64), s["p99"].(float64); !(p01 < 0 && p99 > 0) {
		t.Fatalf("sub lost its sign: p01 %g p99 %g", p01, p99)
	}
	// zero shows as mid-grey after .add(0.5): a look of d.add(0.5) of a flat area is code 128
	z := call(t, e, "add", call(t, e, "sub", img.ID, graph.NodeArg{ID: img.ID}).ID, 0.5)
	if zb := encodedOf(t, e, z); zb.Pix[0] != 0.5 || zb.Pix[len(zb.Pix)-1] != 0.5 {
		t.Fatalf("img − img + 0.5 = %g", zb.Pix[0])
	}
	// a.sub(b).add(b) == a within float error, in encoded space
	back := imgBuf(t, e, call(t, e, "add", d.ID, graph.NodeArg{ID: g.ID}))
	a := encodedOf(t, e, img)
	for i := range a.Pix {
		if math.Abs(float64(back.Pix[i]-a.Pix[i])) > 1e-6 {
			t.Fatalf("sample %d: %g, want %g", i, back.Pix[i], a.Pix[i])
		}
	}
}

func TestArithmeticProperties(t *testing.T) {
	e, img, _ := plantedEngine(t)
	b := call(t, e, "exposure", call(t, e, "gaussian", img.ID, 2.0).ID, -0.7)
	a, bb := encodedOf(t, e, img), encodedOf(t, e, b)
	mx := imgBuf(t, e, call(t, e, "max", img.ID, graph.NodeArg{ID: b.ID}))
	mn := imgBuf(t, e, call(t, e, "min", img.ID, graph.NodeArg{ID: b.ID}))
	ab := imgBuf(t, e, call(t, e, "abs", call(t, e, "sub", img.ID, graph.NodeArg{ID: b.ID}).ID))
	ng := imgBuf(t, e, call(t, e, "mul", img.ID, -1.0))
	for i := range a.Pix {
		if mx.Pix[i] < a.Pix[i] || mx.Pix[i] < bb.Pix[i] || mn.Pix[i] > a.Pix[i] || mn.Pix[i] > bb.Pix[i] {
			t.Fatalf("sample %d: max %g min %g of %g and %g", i, mx.Pix[i], mn.Pix[i], a.Pix[i], bb.Pix[i])
		}
		if ab.Pix[i] < 0 || math.Abs(float64(ab.Pix[i]-float32(math.Abs(float64(a.Pix[i]-bb.Pix[i]))))) > 1e-6 {
			t.Fatalf("sample %d: abs %g", i, ab.Pix[i])
		}
		if ng.Pix[i] != -a.Pix[i] {
			t.Fatalf("mul(-1) sample %d: %g", i, ng.Pix[i])
		}
	}
	// identities: add(0), mul(1)
	for name, k := range map[string]float64{"add": 0, "mul": 1, "sub": 0} {
		n := call(t, e, name, img.ID, k)
		if got := imgBuf(t, e, n); got.Pix[777] != a.Pix[777] {
			t.Fatalf("%s(%g) changed a sample", name, k)
		}
	}
}

func TestArithmeticBroadcastFramesAndKinds(t *testing.T) {
	e, img, _ := plantedEngine(t)
	gray := call(t, e, "toGray", img.ID, "average", map[string]any{"space": "encoded"})
	d := imgBuf(t, e, call(t, e, "sub", img.ID, graph.NodeArg{ID: gray.ID}))
	a, g := encodedOf(t, e, img), imgBuf(t, e, gray)
	if d.C != 3 || math.Abs(float64(d.Plane(2)[300]-(a.Plane(2)[300]-g.Plane(0)[300]))) > 1e-6 {
		t.Fatalf("gray operand not broadcast: %d channels", d.C)
	}
	c1 := call(t, e, "crop", img.ID, []any{0.0, 0.0, 64.0, 64.0})
	c2 := call(t, e, "crop", img.ID, []any{10.0, 0.0, 64.0, 64.0})
	_, err := e.Evaluate(t.Context(), call(t, e, "sub", c1.ID, graph.NodeArg{ID: c2.ID}).ID, graph.EvalOptions{Scale: 1})
	if ce, ok := graph.AsError(err); !ok || ce.Code != graph.EFrame || !strings.Contains(ce.Hint, "crop both alike") {
		t.Fatalf("different frames: %+v", err)
	}
	// a measurement is not an operand: options and operands are literal numbers or images
	if _, err := e.Call("sub", img.ID, []any{graph.NodeArg{ID: call(t, e, "stats", img.ID).ID}}); err == nil {
		t.Fatal("a Value node accepted as an operand")
	} else if ce, _ := graph.AsError(err); ce.Code != graph.EArgType || !strings.Contains(ce.Fix, "await") {
		t.Fatalf("Value operand: %+v", ce)
	}
	// regions are not an operand
	if _, err := e.Call("add", img.ID, []any{graph.NodeArg{ID: call(t, e, "detect.specks", img.ID).ID}}); err == nil {
		t.Fatal("regions accepted as an operand")
	}
}
