package graph

import (
	"crypto/sha256"
	"encoding/binary"
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
	DocW      int      // doc size of the result (0 for pure data)
	DocH      int
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
	if op.DocSize != nil {
		n.DocW, n.DocH = op.DocSize(args, inputs)
	} else {
		for _, in := range inputs {
			if in.DocW > 0 {
				n.DocW, n.DocH = in.DocW, in.DocH
				break
			}
		}
	}
	b.byKey[h], b.nodes[n.ID] = n, n
	return n
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
