// Package graph holds the cv op registry, the node graph and the evaluator.
package graph

import (
	"sort"
	"sync"
)

// Kind is the kind of value a node evaluates to.
type Kind string

const (
	Image   Kind = "Image"
	Mask    Kind = "Mask"
	Regions Kind = "Regions"
	Value   Kind = "Value"
	Param   Kind = "Param"
)

// Kinds lists every kind in display order.
var Kinds = []Kind{Image, Mask, Regions, Value, Param}

// OpDef describes one registered op.
type OpDef struct {
	Name    string
	On      Kind
	Returns Kind
	Doc     string
}

// Registry is the single source of truth for every op the agent can call.
type Registry struct {
	mu  sync.RWMutex
	ops map[string]*OpDef
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{ops: map[string]*OpDef{}}
}

// Ops returns all ops sorted by receiver kind and name.
func (r *Registry) Ops() []*OpDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*OpDef, 0, len(r.ops))
	for _, op := range r.ops {
		out = append(out, op)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].On != out[j].On {
			return out[i].On < out[j].On
		}
		return out[i].Name < out[j].Name
	})
	return out
}
