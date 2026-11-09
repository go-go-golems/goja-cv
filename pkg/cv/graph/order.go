package graph

import (
	"fmt"
	"strings"
)

// Order warnings (GCV-002 §7.2, §7.3): two orders of ops give a view that
// is not what the agent meant, and the builder says so with the reordered
// chain as the fix.
//
//   - CROP_FIRST: an op with a halo (a blur, a high-pass) after a crop sees
//     only the window and its reflected border. This is the first reader of
//     OpDef.Halo.
//   - DECORATION_FIRST: a pixel op after a decoration (overlay, annotate)
//     computes on the drawn marks (levels({ auto }) counts the tint).

func (b *Builder) orderWarnings(op *OpDef, args Args, recv *Node) []Warning {
	if op.Class != PixelOp || op.Internal || op.Lazy || (op.On != KindImage && op.On != KindMask) {
		return nil
	}
	var ws []Warning
	call := ShortCall(op, args)
	if !strings.Contains(call, "(") {
		call += "(…)"
	}
	if hasHalo(op, args) {
		if c := b.inChain(recv, func(n *Node) bool { return n.Op.Class == GeometricOp }); c != nil {
			ws = append(ws, Warning{Code: WCropFirst, Message: fmt.Sprintf(
				"%s after crop sees only the %s px window (and its mirrored border); put crop last. fix: %s.%s.crop(%s)",
				op.Name, c.Frame.Size(), receiverVar(op.On), call, c.Frame)})
		}
	}
	if d := b.inChain(recv, func(n *Node) bool { return n.Op.Class == Decoration }); d != nil {
		ws = append(ws, Warning{Code: WDecorationFirst, Message: fmt.Sprintf(
			"%s after %s sees the drawn marks; put %s last. fix: img.%s.%s(…)", op.Name, d.Op.Name, d.Op.Name, call, d.Op.Name)})
	}
	return ws
}

// inChain returns the first node on the receiver chain from n (n included)
// that match accepts.
func (b *Builder) inChain(n *Node, match func(*Node) bool) *Node {
	for n != nil {
		if match(n) {
			return n
		}
		if !n.HasReceiver() {
			return nil
		}
		n, _ = b.Node(n.Inputs[0])
	}
	return nil
}

// hasHalo reports whether the op reads a border around each output pixel
// with these arguments (a lifted length counts as one).
func hasHalo(op *OpDef, args Args) (out bool) { //nolint:nonamedreturns // recover needs a named result
	if op.Halo == nil {
		return false
	}
	defer func() {
		if recover() != nil {
			out = true // a lifted argument (param, measurement): assume a halo
		}
	}()
	return op.Halo(args) > 0
}
