package graph

import (
	"fmt"
	"strings"
)

// Order warnings (GCV-002 §7.3): DECORATION_FIRST, a pixel op after a
// decoration (overlay, annotate), computes on the drawn marks
// (levels({ auto }) counts the tint); the builder says so with the
// reordered chain as the fix. An op after a crop sees only the crop, which
// is how an agent works locally: crop's help says so, and nothing warns
// (GCV-005 D22).

func (b *Builder) orderWarnings(op *OpDef, args Args, recv *Node) []Warning {
	if op.Class != PixelOp || op.Internal || op.Lazy || op.On != KindImage {
		return nil
	}
	d := b.inChain(recv, func(n *Node) bool { return n.Op.Class == Decoration })
	if d == nil {
		return nil
	}
	call := ShortCall(op, args)
	if !strings.Contains(call, "(") {
		call += "(…)"
	}
	return []Warning{{Code: WDecorationFirst, Message: fmt.Sprintf(
		"%s after %s sees the drawn marks; put %s last. fix: img.%s.%s(…)", op.Name, d.Op.Name, d.Op.Name, call, d.Op.Name)}}
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
