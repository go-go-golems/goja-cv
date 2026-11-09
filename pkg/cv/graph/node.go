package graph

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sync"
)

// NodeID is a node handle; 0 is never a valid node.
type NodeID uint64

// Node is an immutable, hash-consed graph node.
type Node struct {
	ID        NodeID
	Op        *OpDef
	Inputs    []NodeID // receiver (if any), explicit inputs, then lifted args
	NExplicit int      // number of leading inputs passed to Run as in[]
	Args      Args     // canonical, defaults filled, lifted args as InputRef
	Kind      Kind
	Hash      [32]byte // sha256(op key, canonical args, input hashes)
	Frame     Frame    // the rectangle the result covers in doc px (empty for pure data)
}

// Builder interns nodes: structurally identical calls return the same node.
type Builder struct {
	mu    sync.Mutex
	byKey map[[32]byte]*Node
	nodes map[NodeID]*Node
	next  NodeID
}

// NewBuilder returns an empty builder.
func NewBuilder() *Builder {
	return &Builder{byKey: map[[32]byte]*Node{}, nodes: map[NodeID]*Node{}}
}

// Node returns the node with id.
func (b *Builder) Node(id NodeID) (*Node, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n, ok := b.nodes[id]
	return n, ok
}

// Len is the number of interned nodes.
func (b *Builder) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.nodes)
}

func (b *Builder) kindOf(id NodeID) (Kind, bool) {
	n, ok := b.Node(id)
	if !ok {
		return "", false
	}
	return n.Kind, true
}

// Intern checks raw call arguments against op (types, units, hard limits,
// unknown option keys), lifts node-valued arguments into inputs, and returns
// the existing node with the same structure or a new one. receiver is 0 for
// source ops. trusted holds Go-side values for hidden arguments. Warnings
// (RANGE) are returned alongside the node.
func (b *Builder) Intern(op *OpDef, receiver NodeID, raw []any, trusted map[string]any) (*Node, []Warning, error) {
	var recv *Node
	if receiver != 0 {
		var ok bool
		if recv, ok = b.Node(receiver); !ok {
			return nil, nil, Errorf(EInternal, "unknown receiver node #%d", receiver)
		}
		if !accepts(op.On, recv.Kind) {
			return nil, nil, Errorf(EWrongKind, "%s is %s method; the receiver is %s.", op.Name, A(string(op.On)), A(string(recv.Kind)))
		}
	} else if op.On != KindNone {
		return nil, nil, Errorf(EInternal, "%s needs %s receiver", op.Name, A(string(op.On)))
	}
	raw, err := b.resolveBuilt(raw, receiver)
	if err != nil {
		return nil, nil, err
	}
	bd, err := bind(op, recv != nil, raw, trusted, b.kindOf)
	if err != nil {
		return nil, nil, err
	}
	var all []NodeID
	if recv != nil {
		all = append(all, recv.ID)
	}
	all = append(all, bd.inputs...)
	nExplicit := len(all)
	all = append(all, bd.lifted...)

	inputs := make([]*Node, len(all))
	for i, id := range all {
		in, ok := b.Node(id)
		if !ok {
			return nil, nil, Errorf(EInternal, "unknown input node #%d", id)
		}
		inputs[i] = in
	}
	args := Args{op: op, vals: bd.args}
	if op.Validate != nil {
		if err := op.Validate(args); err != nil {
			return nil, nil, err
		}
	}
	if recv != nil {
		if err := checkInFrame(op, args, recv.Frame); err != nil {
			return nil, nil, err
		}
	}
	return b.internNode(op, args, inputs, nExplicit), bd.warnings, nil
}

// internNode returns the node for (op, canonical args, inputs), creating it
// if it does not exist yet.
func (b *Builder) internNode(op *OpDef, args Args, inputs []*Node, nExplicit int) *Node {
	h := hashNode(op, args, inputs)
	b.mu.Lock()
	defer b.mu.Unlock()
	if n, ok := b.byKey[h]; ok {
		return n
	}
	ids := make([]NodeID, len(inputs))
	for i, in := range inputs {
		ids[i] = in.ID
	}
	b.next++
	n := &Node{ID: b.next, Op: op, Inputs: ids, NExplicit: nExplicit, Args: args, Kind: op.Returns, Hash: h}
	n.Frame = nodeFrame(op, args, inputs)
	b.byKey[h], b.nodes[n.ID] = n, n
	return n
}

// checkInFrame refuses literal boxes that miss frame and points outside it,
// for arguments declared InFrame (E_ARG_RANGE with the nearest valid value).
func checkInFrame(op *OpDef, args Args, frame Frame) error {
	if frame.Empty() {
		return nil
	}
	for i := range op.Args {
		s := &op.Args[i]
		if !s.InFrame {
			continue
		}
		l, ok := args.vals[s.Name].([]any)
		if !ok {
			continue
		}
		f := make([]float64, len(l))
		for j, e := range l {
			v, ok := e.(float64)
			if !ok {
				return nil // a lifted coordinate: checked when evaluated
			}
			f[j] = v
		}
		where := op.Name + "." + s.Name
		switch len(f) {
		case 4:
			if !frame.OverlapsBox(f[0], f[1], f[2], f[3]) {
				nb := frame.NearestBox(f[0], f[1], f[2], f[3])
				return Errorf(EArgRange, "%s = [%s, %s, %s, %s] is outside the image's frame %s ([x, y, w, h], doc px).", where, fmtNum(f[0]), fmtNum(f[1]), fmtNum(f[2]), fmtNum(f[3]), frame).
					WithHint("Coordinates are absolute: in the frame of the file the image came from.").
					WithFix(argFix(op, s, fmt.Sprintf("[%s, %s, %s, %s]", fmtNum(nb[0]), fmtNum(nb[1]), fmtNum(nb[2]), fmtNum(nb[3]))))
			}
		case 2:
			if !frame.ContainsPoint(f[0], f[1]) {
				np := frame.NearestPoint(f[0], f[1])
				return Errorf(EArgRange, "%s = [%s, %s] is outside the image's frame %s ([x, y, w, h], doc px).", where, fmtNum(f[0]), fmtNum(f[1]), frame).
					WithHint("Coordinates are absolute: in the frame of the file the image came from.").
					WithFix(argFix(op, s, fmt.Sprintf("[%s, %s]", fmtNum(np[0]), fmtNum(np[1]))))
			}
		}
	}
	return nil
}

// argFix renders a call of op with value for argument s, as a literal fix:
// img.mask.flood([32, 24]) or img.stats({ box: [0, 0, 10, 10] }).
func argFix(op *OpDef, s *ArgSpec, value string) string {
	recv := map[Kind]string{KindImage: "img.", KindMask: "mask.", KindRegions: "regions.", KindValue: "value."}[op.On]
	if s.Positional {
		return fmt.Sprintf("%s%s(%s)", recv, op.Name, value)
	}
	return fmt.Sprintf("%s%s({ %s: %s })", recv, op.Name, s.Name, value)
}

// nodeFrame is the frame of a node: op.DocFrame, or the first input with a
// non-empty frame. It depends only on the hash inputs, so a hash hit has the
// frame a new node would get.
func nodeFrame(op *OpDef, args Args, inputs []*Node) Frame {
	if op.DocFrame != nil {
		return op.DocFrame(args, inputs)
	}
	for _, in := range inputs {
		if !in.Frame.Empty() {
			return in.Frame
		}
	}
	return Frame{}
}

// BuiltArg is the result of a builder lambda in a raw call: the node the
// function returned (Root), built on the placeholder node Hole that stood
// for the call's receiver. Intern substitutes the receiver for the
// placeholder before binding, so the op's input is an ordinary graph over
// the real receiver (hash-consed with the same graph built directly).
type BuiltArg struct{ Root, Hole NodeID }

// PlaceholderOp is the name of the source op of placeholder nodes.
const PlaceholderOp = "placeholder"

// resolveBuilt replaces top-level BuiltArgs in raw by node arguments.
func (b *Builder) resolveBuilt(raw []any, receiver NodeID) ([]any, error) {
	var out []any
	for i, v := range raw {
		ba, ok := v.(BuiltArg)
		if !ok {
			continue
		}
		hole, ok := b.Node(ba.Hole)
		if !ok || hole.Op.Name != PlaceholderOp || hole.Op.On != KindNone {
			return nil, Errorf(EInternal, "builder result #%d does not come with a placeholder", ba.Root)
		}
		if receiver == 0 {
			return nil, Errorf(EInternal, "a builder lambda needs a receiver")
		}
		id, err := b.Substitute(ba.Root, ba.Hole, receiver)
		if err != nil {
			return nil, err
		}
		if out == nil {
			out = append([]any(nil), raw...)
		}
		out[i] = NodeArg{ID: id}
	}
	if out == nil {
		return raw, nil
	}
	return out, nil
}

// Substitute returns the node equal to root with every occurrence of node
// from replaced by node to (re-interning each node on the path; nodes that do
// not depend on from are returned unchanged). The replacement must have the
// same kind as from.
func (b *Builder) Substitute(root, from, to NodeID) (NodeID, error) {
	fn, ok1 := b.Node(from)
	tn, ok2 := b.Node(to)
	if !ok1 || !ok2 {
		return 0, Errorf(EInternal, "substitute: unknown node")
	}
	if fn.Kind != tn.Kind {
		return 0, Errorf(EInternal, "substitute: %s placeholder for %s node", fn.Kind, A(string(tn.Kind)))
	}
	memo := map[NodeID]*Node{from: tn}
	var walk func(id NodeID) (*Node, error)
	walk = func(id NodeID) (*Node, error) {
		if n, ok := memo[id]; ok {
			return n, nil
		}
		n, ok := b.Node(id)
		if !ok {
			return nil, Errorf(EInternal, "substitute: unknown node #%d", id)
		}
		changed := false
		ins := make([]*Node, len(n.Inputs))
		for i, in := range n.Inputs {
			r, err := walk(in)
			if err != nil {
				return nil, err
			}
			ins[i] = r
			changed = changed || r.ID != in
		}
		out := n
		if changed {
			out = b.internNode(n.Op, n.Args, ins, n.NExplicit)
		}
		memo[id] = out
		return out, nil
	}
	r, err := walk(root)
	if err != nil {
		return 0, err
	}
	return r.ID, nil
}

func hashNode(op *OpDef, args Args, inputs []*Node) [32]byte {
	h := sha256.New()
	key := op.Key()
	_ = binary.Write(h, binary.LittleEndian, uint32(len(key))) // #nosec G115 -- an op key is a short identifier
	h.Write([]byte(key))
	js := args.canonicalJSON()
	_ = binary.Write(h, binary.LittleEndian, uint32(len(js))) // #nosec G115 -- canonical args JSON is far below 4 GiB
	h.Write(js)
	for _, in := range inputs {
		h.Write(in.Hash[:])
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}
