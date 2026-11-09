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
	Base      int      // document step the node was built on (E_STALE); -1 if none
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
			return nil, nil, Errorf(EWrongKind, "%s is a %s method; the receiver is a %s.", op.Name, op.On, recv.Kind)
		}
	} else if op.On != KindNone {
		return nil, nil, Errorf(EInternal, "%s needs a %s receiver", op.Name, op.On)
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
	h := hashNode(op, args, inputs)

	b.mu.Lock()
	defer b.mu.Unlock()
	if n, ok := b.byKey[h]; ok {
		return n, bd.warnings, nil
	}
	b.next++
	n := &Node{ID: b.next, Op: op, Inputs: all, NExplicit: nExplicit, Args: args, Kind: op.Returns, Hash: h, Base: -1}
	for _, in := range inputs {
		n.Base = max(n.Base, in.Base)
	}
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
	return n, bd.warnings, nil
}

func hashNode(op *OpDef, args Args, inputs []*Node) [32]byte {
	h := sha256.New()
	key := op.Key()
	_ = binary.Write(h, binary.LittleEndian, uint32(len(key)))
	h.Write([]byte(key))
	js := args.canonicalJSON()
	_ = binary.Write(h, binary.LittleEndian, uint32(len(js)))
	h.Write(js)
	for _, in := range inputs {
		h.Write(in.Hash[:])
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}
