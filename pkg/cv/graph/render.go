package graph

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"golang.org/x/sync/errgroup"
)

// Rendering (GCV-002 §7.2, Foundations §F5). A look or a sheet cell may ask
// for a render scale above 1 (zoom). Evaluate never computes above scale 1;
// Render dispatches on the op class:
//
//   - data (Regions, Value): evaluated as usual (at 1 or the AnalysisScale);
//   - a macro node: its expansion's output;
//   - a geometric op (crop): its receiver rendered in the window ∩ its frame;
//   - a decoration (overlay, annotate): run at the render scale on Image and
//     Mask inputs rendered in the window;
//   - a pixel op or a source: evaluated at scale 1 over its whole frame
//     (cached as usual), cut to the window and upsampled.
//
// The render window (doc px) bounds every upsampled and decorated buffer by
// the look's size. Render results are cached under a key of the node's
// scale-1 signature, the scale, the interpolation and the window.

// MaxRenderScale is the largest render scale (zoom 16).
const MaxRenderScale = 16

// RenderOptions select a render.
type RenderOptions struct {
	Scale  float64       // image px per doc px, in (0, MaxRenderScale]
	Interp buffer.Interp // upsampling above scale 1
	Window *Frame        // doc-px rectangle to render (integer origin); nil = the node's frame
	OnWarn func(Warning) // evaluation-time warnings, as EvalOptions.OnWarn
}

// Render computes node id for a look or a sheet cell. At scale ≤ 1 it is
// Evaluate, cut to the window. Above 1 it renders by op class (see above).
// An Image or Mask result covers the window: its Frame is the window and its
// buffer is ScaledSize(window, scale).
func (e *Evaluator) Render(ctx context.Context, id NodeID, o RenderOptions) (Value, error) {
	n, ok := e.b.Node(id)
	if !ok {
		return nil, Errorf(EInternal, "unknown node #%d", id)
	}
	if !(o.Scale > 0 && o.Scale <= MaxRenderScale) {
		return nil, Errorf(EArgRange, "render scale %g is outside (0, %d].", o.Scale, MaxRenderScale)
	}
	s := QuantizeScale(o.Scale)
	img := n.Kind == KindImage || n.Kind == KindMask
	win := n.Frame
	if o.Window != nil && img {
		win = intersect(*o.Window, n.Frame)
		if win.Empty() {
			return nil, Errorf(EArgRange, "render window %s does not overlap the image's frame %s.", *o.Window, n.Frame)
		}
	}
	if !img {
		s = 1 // data does not upsample
	}
	if s <= 1 {
		r := e.newRun(s, o.OnWarn)
		v, err := r.eval(ctx, id)
		if err != nil {
			return nil, err
		}
		r.replayTree(id, map[NodeID]bool{})
		if !img || win == n.Frame {
			return v, nil
		}
		return cutValue(v.(*ImageValue), win)
	}
	rr := &renderRun{e: e, one: e.newRun(1, o.OnWarn), s: s, interp: o.Interp}
	v, err := rr.render(ctx, n, win)
	if err != nil {
		return nil, err
	}
	rr.one.replayTree(id, map[NodeID]bool{})
	return v, nil
}

// Render computes node id for a look (see Evaluator.Render).
func (e *Engine) Render(ctx context.Context, id NodeID, o RenderOptions) (Value, error) {
	return e.ev.Render(ctx, id, o)
}

type renderRun struct {
	e      *Evaluator
	one    *evalRun // the scale-1 evaluation that pixel ops and data use
	s      float64
	interp buffer.Interp
}

func (rr *renderRun) render(ctx context.Context, n *Node, win Frame) (Value, error) {
	if err := ctx.Err(); err != nil {
		return nil, ctxError(err)
	}
	if n.Kind != KindImage && n.Kind != KindMask {
		return rr.one.eval(ctx, n.ID)
	}
	if IsMacro(n) {
		return rr.render(ctx, rr.e.node(n.Inputs[len(n.Inputs)-1]), win)
	}
	switch n.Op.Class {
	case GeometricOp:
		in := rr.e.node(n.Inputs[0])
		w := intersect(win, n.Frame)
		v, err := rr.render(ctx, in, w)
		if err != nil {
			return nil, err
		}
		iv := v.(*ImageValue)
		out := *iv
		out.Enc = n.Enc
		return &out, nil
	case Decoration:
		return rr.cached(n, win, func() (Value, error) { return rr.decorate(ctx, n, win) })
	case PixelOp:
	}
	return rr.cached(n, win, func() (Value, error) {
		v, err := rr.one.eval(ctx, n.ID)
		if err != nil {
			return nil, err
		}
		c, err := cutValue(v.(*ImageValue), win)
		if err != nil {
			return nil, err
		}
		up, err := buffer.Upsample(ctx, c.Buf, rr.s, rr.interp)
		if err != nil {
			return nil, err
		}
		return c.withBuffer(up, win), nil
	})
}

// cached memoizes a render result under (scale-1 signature, scale, interp,
// window).
func (rr *renderRun) cached(n *Node, win Frame, compute func() (Value, error)) (Value, error) {
	key := rr.key(n, win)
	if v, ok := rr.e.cache.Get(key); ok {
		rr.one.replay(key)
		return v, nil
	}
	v, err := compute()
	if err != nil {
		return nil, err
	}
	rr.e.cache.Put(key, v)
	return v, nil
}

func (rr *renderRun) key(n *Node, win Frame) Signature {
	sig := rr.one.sig(n.ID)
	h := sha256.New()
	h.Write([]byte("render"))
	h.Write(sig[:])
	_ = binary.Write(h, binary.LittleEndian, int64(math.Round(rr.s*1e4)))
	_ = binary.Write(h, binary.LittleEndian, int64(rr.interp))
	for _, x := range []int{win.X, win.Y, win.W, win.H} {
		_ = binary.Write(h, binary.LittleEndian, int64(x))
	}
	var out Signature
	copy(out[:], h.Sum(nil))
	return out
}

// decorate runs a decoration at the render scale: Image and Mask inputs
// rendered in the window, other inputs evaluated as usual.
func (rr *renderRun) decorate(ctx context.Context, n *Node, win Frame) (Value, error) {
	ins := make([]Value, len(n.Inputs))
	g, gctx := errgroup.WithContext(ctx)
	for i, id := range n.Inputs {
		in := rr.e.node(id)
		g.Go(func() error {
			var v Value
			var err error
			if in.Kind == KindImage || in.Kind == KindMask {
				v, err = rr.render(gctx, in, intersect(win, in.Frame))
			} else {
				v, err = rr.one.eval(gctx, id)
			}
			ins[i] = v
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	args, err := resolveArgs(n, ins)
	if err != nil {
		return nil, rr.e.wrapWithPath(err, n)
	}
	if n.Op.Identity != nil && n.NExplicit > 0 && n.Op.Identity(args) {
		return ins[0], nil
	}
	ec := &EvalCtx{Eval: rr.e, Node: n, Scale: rr.s, Params: rr.one.params, Window: win}
	out, err := safeRun(ctx, n.Op, ec, ins[:n.NExplicit], args)
	if err != nil {
		return nil, rr.e.wrapWithPath(err, n)
	}
	iv, ok := out.(*ImageValue)
	if !ok {
		return nil, rr.e.wrapWithPath(Errorf(EInternal, "%s returned no image", n.Op.Name), n)
	}
	if isInputValue(out, ins) {
		c := *iv
		iv = &c
	}
	iv.Frame, iv.Enc = win, n.Enc
	rr.e.countRun(n.ID)
	key := rr.key(n, win)
	if ws := ec.warnings(); len(ws) > 0 {
		rr.e.keepWarnings(key, ws)
		rr.one.replay(key)
	}
	return iv, nil
}

func isInputValue(v Value, ins []Value) bool {
	for _, in := range ins {
		if in == v {
			return true
		}
	}
	return false
}

// intersect is the rectangle common to a and b (empty when they do not meet).
func intersect(a, b Frame) Frame {
	x0, y0 := max(a.X, b.X), max(a.Y, b.Y)
	x1, y1 := min(a.X+a.W, b.X+b.W), min(a.Y+a.H, b.Y+b.H)
	if x1 <= x0 || y1 <= y0 {
		return Frame{X: x0, Y: y0}
	}
	return Frame{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// Intersect is the rectangle common to a and b (empty when they do not meet).
func Intersect(a, b Frame) Frame { return intersect(a, b) }

// cutValue cuts window win (doc px, inside v's frame) out of v at v's
// scale. Below scale 1 the cut is aligned to buffer pixels: it covers the
// buffer pixels the window touches.
func cutValue(v *ImageValue, win Frame) (*ImageValue, error) {
	if win == v.Frame {
		return v, nil
	}
	sx, sy := AxisScales(v)
	x0 := int(math.Floor(float64(win.X-v.Frame.X) * sx))
	y0 := int(math.Floor(float64(win.Y-v.Frame.Y) * sy))
	x1 := min(v.Buf.W, max(x0+1, int(math.Ceil(float64(win.X+win.W-v.Frame.X)*sx))))
	y1 := min(v.Buf.H, max(y0+1, int(math.Ceil(float64(win.Y+win.H-v.Frame.Y)*sy))))
	b, err := buffer.Cut(v.Buf, x0, y0, x1-x0, y1-y0)
	if err != nil {
		return nil, Errorf(EInternal, "cut %s of %s: %v", win, v.Frame, err)
	}
	return v.withBuffer(b, win), nil
}

// withBuffer is v with another buffer and frame (same kind and encoding).
func (v *ImageValue) withBuffer(b *buffer.Buffer, f Frame) *ImageValue {
	out := *v
	out.Buf, out.Frame = b, f
	return &out
}
