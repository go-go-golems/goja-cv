package graph

// Macros (GCV-001 §7.8): an op defined as a small graph of other ops, built
// at intern time. The binder binds a macro's arguments and resolves its
// space first, then calls Expand. A macro that builds one node gets a macro
// node of its own (its inputs are the call's inputs, then the expansion's
// output), which gives it an identity and a label while the expansion's
// nodes stay shared with the same hand-written chain. Evaluating a macro
// node evaluates its last input. A macro that builds several nodes (split)
// returns them.

// ExpandFunc builds the op's graph from other ops at intern time. args are
// bound; the steps follow recv's space unless they convert it with as.
type ExpandFunc func(x *Expander, recv *Node, args Args) ([]*Node, error)

// Expander lets an ExpandFunc call other ops; it collects their warnings.
type Expander struct {
	b        *Builder
	warnings []Warning
}

// Call interns op name on recv with raw arguments through the builder
// (binding, space resolution, conversions). It may call Internal ops.
func (x *Expander) Call(name string, recv *Node, args ...any) (*Node, error) {
	n, ws, err := x.b.internOp(name, recv, args...)
	if err != nil {
		return nil, err
	}
	x.warnings = append(x.warnings, ws...)
	return n, nil
}

// expand runs a macro's Expand and interns its macro node (one output) or
// returns its outputs.
func (b *Builder) expand(op *OpDef, args Args, recv *Node, warnings []Warning) (CallResult, error) {
	x := &Expander{b: b}
	out, err := op.Expand(x, recv, args)
	if err != nil {
		return CallResult{}, err
	}
	ws := append(warnings, x.warnings...)
	switch len(out) {
	case 0:
		return CallResult{}, Errorf(EInternal, "macro %s built nothing", op.Name)
	case 1:
		ins := append(append([]*Node(nil), args.ins...), out[0])
		return CallResult{Nodes: []*Node{b.internNode(op, args, ins)}, Warnings: ws}, nil
	}
	return CallResult{Nodes: out, Warnings: ws}, nil
}

// IsMacro reports whether n is a macro node: its value is its last input's.
func IsMacro(n *Node) bool { return n.Op.Expand != nil }

// Node is the call's node when it built exactly one (nil otherwise).
func (r CallResult) Node() *Node {
	if len(r.Nodes) != 1 {
		return nil
	}
	return r.Nodes[0]
}
