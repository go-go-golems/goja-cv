package ops

import (
	"context"
	"math"
	"math/rand"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph/graphtest"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

func TestHighpassAndDoGAreTheHandWrittenChains(t *testing.T) {
	e, img, _ := plantedEngine(t)
	hp := call(t, e, "highpass", img.ID, 3.0)
	chain := call(t, e, "sub", img.ID, graph.NodeArg{ID: call(t, e, "gaussian", img.ID, 3.0).ID})
	if !graph.IsMacro(hp) || hp.Inputs[len(hp.Inputs)-1] != chain.ID || graph.Label(hp) != "highpass" {
		t.Fatalf("highpass(3) is %s with last input #%d, want the macro over #%d", graph.Label(hp), hp.Inputs[len(hp.Inputs)-1], chain.ID)
	}
	// the steps follow the receiver: file units here...
	g, _ := e.Node(chain.Inputs[1])
	if g.Op.Name != "gaussian" || g.Space != buffer.SpaceEncoded {
		t.Fatalf("highpass steps: %s in %s", g.Op.Name, g.Space)
	}
	// ...and linear light on img.as("linear")
	hl := call(t, e, "highpass", call(t, e, graph.AsOp, img.ID, "linear").ID, 3.0)
	sl, _ := e.Node(hl.Inputs[len(hl.Inputs)-1])
	if gl, _ := e.Node(sl.Inputs[1]); gl.Op.Name != "gaussian" || gl.Space != buffer.SpaceLinear {
		t.Fatalf("highpass of a linear receiver: %s in %s", gl.Op.Name, gl.Space)
	}
	d := call(t, e, "dog", img.ID, 1.0, 4.0, map[string]any{"polarity": "bright"})
	want := call(t, e, "max", call(t, e, "sub", call(t, e, "gaussian", img.ID, 1.0).ID,
		graph.NodeArg{ID: call(t, e, "gaussian", img.ID, 4.0).ID}).ID, 0.0)
	if d.Inputs[len(d.Inputs)-1] != want.ID {
		t.Fatal("dog(1, 4, bright) is not max(G1 − G4, 0)")
	}
	if d.Enc != buffer.EncLinear {
		t.Fatalf("dog is %s, want derived", d.Enc)
	}
	_, err := e.Call("dog", img.ID, []any{4.0, 1.0})
	if ce, ok := graph.AsError(err); !ok || ce.Code != graph.EArgRange || ce.Fix != "img.dog(1, 4)" {
		t.Fatalf("dog(4, 1): %+v", err)
	}
	c := call(t, e, "crop", img.ID, []any{10.0, 10.0, 64.0, 64.0})
	if ws, _ := e.Call("highpass", c.ID, []any{3.0}); len(ws.Warnings) != 1 || !strings.HasPrefix(ws.Warnings[0].Message, "highpass after crop") {
		t.Fatalf("highpass after crop: %+v", ws.Warnings)
	}
}

func TestDoGPolaritiesOnPlantedSpecks(t *testing.T) {
	e, img, p := plantedEngine(t)
	gray := call(t, e, "toGray", img.ID, "average")
	at := func(n *graph.Node, x, y float64) float32 {
		b := imgBuf(t, e, n)
		return b.Plane(0)[int(y)*b.W+int(x)]
	}
	bright := call(t, e, "dog", gray.ID, 1.0, 4.0, map[string]any{"polarity": "bright"})
	dark := call(t, e, "dog", gray.ID, 1.0, 4.0, map[string]any{"polarity": "dark"})
	both := call(t, e, "dog", gray.ID, 1.0, 4.0)
	var nb, nd int
	for _, s := range p.Specks {
		if s.RX < 1.5 {
			continue
		}
		b, d, v := at(bright, s.X, s.Y), at(dark, s.X, s.Y), at(both, s.X, s.Y)
		if s.Dark {
			nd++
			if !(d > 0.01 && b == 0 && v < 0) {
				t.Errorf("dark speck at (%g, %g): bright %g dark %g both %g", s.X, s.Y, b, d, v)
			}
		} else {
			nb++
			if !(b > 0.01 && d == 0 && v > 0) {
				t.Errorf("bright speck at (%g, %g): bright %g dark %g both %g", s.X, s.Y, b, d, v)
			}
		}
	}
	if nb == 0 || nd == 0 {
		t.Fatalf("checked %d bright and %d dark specks", nb, nd)
	}
}

func TestLocalContrast(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	graphtest.RegisterSource(reg)
	e := graph.NewEngine(reg, graphtest.Options(t))
	flat, err := graphtest.Source(e, "linear", graph.Frame{W: 64, H: 48}, 0.4)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range imgBuf(t, e, call(t, e, "localContrast", flat.ID, 4.0)).Pix {
		if v != 0 {
			t.Fatalf("local contrast of a constant image: %g", v)
		}
	}
	// white noise: unit spread away from the borders
	rng := rand.New(rand.NewSource(7)) // #nosec G404 -- deterministic test noise
	b := buffer.New(256, 256, 1, 1)
	for i := range b.Pix {
		b.Pix[i] = float32(0.5 + 0.05*rng.NormFloat64())
	}
	out, err := kernels.LocalContrast(ctx, b, 6)
	if err != nil {
		t.Fatal(err)
	}
	var s, s2 float64
	n := 0
	for y := 40; y < 216; y++ {
		for x := 40; x < 216; x++ {
			v := float64(out.Pix[y*256+x])
			s += v
			s2 += v * v
			n++
		}
	}
	mean, sd := s/float64(n), math.Sqrt(s2/float64(n)-(s/float64(n))*(s/float64(n)))
	if math.Abs(mean) > 0.05 || sd < 0.85 || sd > 1.15 {
		t.Fatalf("local contrast of white noise: mean %g, spread %g", mean, sd)
	}
}

func TestLevelsAuto(t *testing.T) {
	e, img, _ := plantedEngine(t)
	c := call(t, e, "crop", img.ID, []any{150.0, 20.0, 64.0, 64.0})
	auto := call(t, e, "levels", c.ID, map[string]any{"auto": true})
	s := evalData(t, e, call(t, e, "stats", c.ID), 1).(map[string]any)
	ref := call(t, e, "levels", c.ID, map[string]any{"black": s["p01"], "white": s["p99"]})
	ab, rb := imgBuf(t, e, auto), imgBuf(t, e, ref)
	for i := range ab.Pix {
		if math.Abs(float64(ab.Pix[i]-rb.Pix[i])) > 1e-5 {
			t.Fatalf("levels auto sample %d: %g, want %g (p01/p99 of the crop)", i, ab.Pix[i], rb.Pix[i])
		}
	}
	as := evalData(t, e, call(t, e, "stats", auto.ID), 1).(map[string]any)
	// channels map separately and the luminance is encode(Y) of linear light,
	// so its p01 and p99 land a little off 0 and 1
	if as["p01"].(float64) > 0.03 || math.Abs(as["p99"].(float64)-1) > 5e-3 {
		t.Fatalf("after auto: p01 %v p99 %v", as["p01"], as["p99"])
	}
	// not short-circuited as "black 0, white 1, gamma 1"
	if e.Evaluator().Runs(auto.ID) != 1 {
		t.Fatal("levels({ auto: true }) did not run")
	}
	pct := call(t, e, "levels", c.ID, map[string]any{"auto": []any{0.005, 0.995}})
	if pct.ID == auto.ID {
		t.Fatal("explicit percentiles intern like auto: true")
	}
	_, err := e.Call("levels", img.ID, []any{map[string]any{"auto": true, "white": 0.5}})
	if ce, ok := graph.AsError(err); !ok || ce.Code != graph.EArgType || !strings.Contains(ce.Fix, "levels({ auto: true })") {
		t.Fatalf("auto with white: %+v", err)
	}
	_, err = e.Call("levels", img.ID, []any{map[string]any{"box": []any{0.0, 0.0, 8.0, 8.0}}})
	if ce, ok := graph.AsError(err); !ok || ce.Code != graph.EUnknownOption || ce.Fix != "img.crop([x, y, w, h]).levels({ auto: true })" {
		t.Fatalf("levels box: %+v", err)
	}
	if _, err := e.Call("levels", img.ID, []any{map[string]any{"auto": []any{0.9, 0.1}}}); err == nil {
		t.Fatal("auto [0.9, 0.1] accepted")
	}
}

func TestEveryViewPresetBuildsAndRenders(t *testing.T) {
	ctx := context.Background()
	e, img, _ := plantedEngine(t)
	for _, p := range Presets {
		v := call(t, e, "view", img.ID, p.Name)
		if !graph.IsMacro(v) || graph.Label(v) != "view" {
			t.Fatalf("view(%s) is %s", p.Name, graph.Label(v))
		}
		if got := e.Chain(v.ID); got != "open(planted-256x192-16bit.tif) → view("+p.Name+")" {
			t.Fatalf("chain %q", got)
		}
		shown, err := e.Display(call(t, e, "crop", v.ID, []any{100.0, 40.0, 32.0, 32.0}).ID)
		if err != nil {
			t.Fatal(err)
		}
		r, err := e.Render(ctx, shown, graph.RenderOptions{Scale: 8})
		if err != nil {
			t.Fatalf("render view(%s): %v", p.Name, err)
		}
		iv := r.(*graph.ImageValue)
		lo, hi := float32(math.Inf(1)), float32(math.Inf(-1))
		for _, x := range iv.Buf.Pix {
			lo, hi = min(lo, x), max(hi, x)
		}
		if iv.Buf.W != 256 || !(hi > lo) {
			t.Fatalf("view(%s) at zoom 8: %d px wide, values %g–%g", p.Name, iv.Buf.W, lo, hi)
		}
		if p.Doc == "" {
			t.Fatalf("view(%s) has no description", p.Name)
		}
	}
	_, err := e.Call("view", img.ID, []any{"dust"})
	if ce, ok := graph.AsError(err); !ok || !strings.HasPrefix(ce.Hint, `Did you mean "speck"? scene: `) || ce.Fix != `view("speck")` {
		t.Fatalf("unknown preset: %+v / hint %q fix %q", err, ce.Hint, ce.Fix)
	}
}
