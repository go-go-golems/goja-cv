// Package ops registers the cv ops (one file per group) into a graph.Registry.
package ops

import (
	"sort"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// Register adds every op to r.
func Register(r *graph.Registry) {
	r.MustRegister(sourceOps()...)
	r.MustRegister(spaceOps()...)
	r.MustRegister(toneOps()...)
	r.MustRegister(geometryOps()...)
	r.MustRegister(arithImageOps()...)
	r.MustRegister(filterViewOps()...)
	r.MustRegister(viewOp(), levelsAtOp())
	r.MustRegister(channelOps()...)
	r.MustRegister(filterOps()...)
	r.MustRegister(measureOps()...)
	r.MustRegister(measurementOps()...)
	r.MustRegister(maskMakerOps()...)
	r.MustRegister(maskOps()...)
	r.MustRegister(regionOps()...)
	r.MustRegister(graph.ItemsOpDef())
	r.MustRegister(compositeOps()...)
	r.MustRegister(detectOps()...)
	r.MustRegister(healOps()...)
	r.MustRegister(presentOps()...)
	r.MustRegister(annotateOp())
	r.MustRegister(sheetOp())
	r.MustRegisterFunc(funcDefs()...)
}

// NewRegistry returns a registry with every op registered.
func NewRegistry() *graph.Registry {
	r := graph.NewRegistry()
	Register(r)
	return r
}

func image(v graph.Value) *graph.ImageValue { return v.(*graph.ImageValue) }

func data(v any) *graph.DataValue { return &graph.DataValue{Data: v} }

func keysOf(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
