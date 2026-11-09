// Package ops registers the cv ops (one file per group) into a graph.Registry.
package ops

import "github.com/go-go-golems/goja-cv/pkg/cv/graph"

// Register adds every op to r.
func Register(r *graph.Registry) {
	r.MustRegister(sourceOps()...)
	r.MustRegister(toneOps()...)
	r.MustRegister(filterOps()...)
	r.MustRegister(measureOps()...)
	r.MustRegister(valueOps()...)
	r.MustRegister(maskMakerOps()...)
	r.MustRegister(maskOps()...)
	r.MustRegister(regionOps()...)
	r.MustRegister(compositeOps()...)
	r.MustRegister(detectOps()...)
	r.MustRegister(healOps()...)
	r.MustRegister(metaOps()...)
}

// NewRegistry returns a registry with every op registered.
func NewRegistry() *graph.Registry {
	r := graph.NewRegistry()
	Register(r)
	return r
}

func image(v graph.Value) *graph.ImageValue { return v.(*graph.ImageValue) }

func data(v any) *graph.DataValue { return &graph.DataValue{Data: v} }
