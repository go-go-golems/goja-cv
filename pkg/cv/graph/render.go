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
// buffer is exactly ScaledSize(window, scale), which the one exit enforces.
func (e *Evaluator) Render(ctx context.Context, id NodeID, o RenderOptions) (Value, error) {
	n, win, s, err := e.renderRequest(id, o)
	if err != nil {
		return nil, err
	}
	v, err := e.renderBranch(ctx, n, win, s, o)
	if err != nil || !isImageKind(n.Kind) {
		return v, err
	}
	return exactSize(ctx, v.(*ImageValue), win, s) // the only place the size is decided
}

func isImageKind(k Kind) bool { return k == KindImage || k == KindMask }

// renderRequest validates a render and returns the node, its window (the
// node's frame for data) and the quantized scale (1 for data).
func (e *Evaluator) renderRequest(id NodeID, o RenderOptions) (*Node, Frame, float64, error) {
	n, ok := e.b.Node(id)
	if !ok {
		return nil, Frame{}, 0, Errorf(EInternal, "unknown node #%d", id)
	}
	if !(o.Scale > 0 && o.Scale <= MaxRenderScale) {
		return nil, Frame{}, 0, Errorf(EArgRange, "render scale %g is outside (0, %d].", o.Scale, MaxRenderScale)
	}
	if !isImageKind(n.Kind) {
		return n, n.Frame, 1, nil // data does not upsample
	}
	win := n.Frame
	if o.Window != nil {
		win = intersect(*o.Window, n.Frame)
		if win.Empty() {
			return nil, Frame{}, 0, Errorf(EArgRange, "render window %s does not overlap the image's frame %s.", *o.Window, n.Frame)
		}
	}
	return n, win, QuantizeScale(o.Scale), nil
}

// renderBranch computes the value: data as usual, ≤ 1 evaluated and cut,
// > 1 rendered by op class.
func (e *Evaluator) renderBranch(ctx context.Context, n *Node, win Frame, s float64, o RenderOptions) (Value, error) {
	if s <= 1 {
		r := e.newRun(s, o.OnWarn)
		v, err := r.eval(ctx, n.ID)
		if err != nil {
			return nil, err
		}
		r.replayTree(n.ID, map[NodeID]bool{})
		if !isImageKind(n.Kind) {
			return v, nil
		}
		return cutValue(v.(*ImageValue), win, s)
	}
	rr := &renderRun{e: e, one: e.newRun(1, o.OnWarn), s: s, interp: o.Interp}
	v, err := rr.render(ctx, n, win)
	if err != nil {
		return nil, err
	}
	rr.one.replayTree(n.ID, map[NodeID]bool{})
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
		c, err := cutValue(v.(*ImageValue), win, 1)
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
		g.Go(func() (err error) { //nolint:nonamedreturns // recover needs named results
			defer Recover(&err, "rendering "+n.Op.Name+"'s input")
			var v Value
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
	args := resolveArgs(n, ins)
	if n.Op.Identity != nil && n.HasReceiver() && n.Op.Identity(args) {
		return ins[0], nil
	}
	ec := &EvalCtx{Eval: rr.e, Node: n, Scale: rr.s, Window: win}
	out, err := safeRun(ctx, n.Op, ec, receiverValue(n, ins), args)
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

// exactSize returns v when its buffer is ScaledSize(win, s) and otherwise v
// resized to exactly that size: only results computed at another scale (a
// sheet, composed at its AnalysisScale 1) are resized.
func exactSize(ctx context.Context, v *ImageValue, win Frame, s float64) (*ImageValue, error) {
	w, h := buffer.ScaledSize(win.W, win.H, s)
	if v.Buf.W == w && v.Buf.H == h {
		return v, nil
	}
	b, err := buffer.Resize(ctx, v.Buf, w, h)
	if err != nil {
		return nil, Errorf(EInternal, "resize %s to %d×%d: %v", win, w, h, err)
	}
	b.Scale = s
	out := v.withBuffer(b, win)
	out.origin = nil
	return out, nil
}

// cutValue cuts window win (doc px, inside v's frame) out of v, a value
// evaluated at scale s. A buffer at scale s gives exactly ScaledSize(win, s)
// buffer px, starting at the buffer pixel under the window's corner and
// shifted left or up by any overflow at the buffer's edge; the value
// records where its buffer starts in doc px (the rule crop follows below
// scale 1). A buffer at another scale (an op with an AnalysisScale) is cut
// to the buffer pixels the window touches, and exactSize resizes it.
func cutValue(v *ImageValue, win Frame, s float64) (*ImageValue, error) {
	if win == v.Frame {
		return v, nil
	}
	sx, sy := AxisScales(v)
	ox, oy := v.BufferOrigin()
	fx, fy := (float64(win.X)-ox)*sx, (float64(win.Y)-oy)*sy
	w, h := buffer.ScaledSize(win.W, win.H, s)
	if fw, fh := buffer.ScaledSize(v.Frame.W, v.Frame.H, s); v.Buf.W != fw || v.Buf.H != fh {
		w = max(1, int(math.Ceil((float64(win.X+win.W)-ox)*sx))-int(math.Floor(fx)))
		h = max(1, int(math.Ceil((float64(win.Y+win.H)-oy)*sy))-int(math.Floor(fy)))
	}
	w, h = min(w, v.Buf.W), min(h, v.Buf.H)
	x0 := min(max(0, int(math.Floor(fx))), v.Buf.W-w)
	y0 := min(max(0, int(math.Floor(fy))), v.Buf.H-h)
	b, err := buffer.Cut(v.Buf, x0, y0, w, h)
	if err != nil {
		return nil, Errorf(EInternal, "cut %s of %s: %v", win, v.Frame, err)
	}
	out := v.withBuffer(b, win)
	out.origin = nil
	if bx, by := ox+float64(x0)/sx, oy+float64(y0)/sy; bx != float64(win.X) || by != float64(win.Y) {
		out.origin = &[2]float64{bx, by}
	}
	return out, nil
}

// withBuffer is v with another buffer and frame (same kind and encoding).
func (v *ImageValue) withBuffer(b *buffer.Buffer, f Frame) *ImageValue {
	out := *v
	out.Buf, out.Frame = b, f
	return &out
}
