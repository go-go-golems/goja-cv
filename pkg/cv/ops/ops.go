// Package ops registers the cv ops (one file per group) into a graph.Registry.
package ops

import "github.com/go-go-golems/goja-cv/pkg/cv/graph"

// Register adds every op to r.
func Register(r *graph.Registry) {
	r.MustRegister(sourceOps()...)
	r.MustRegister(spaceOps()...)
	r.MustRegister(toneOps()...)
	r.MustRegister(geometryOps()...)
	r.MustRegister(arithImageOps()...)
	r.MustRegister(filterViewOps()...)
	r.MustRegister(viewOp())
	r.MustRegister(channelOps()...)
	r.MustRegister(filterOps()...)
	r.MustRegister(measureOps()...)
	r.MustRegister(measurementOps()...)
	r.MustRegister(valueOps()...)
	r.MustRegister(maskMakerOps()...)
	r.MustRegister(maskOps()...)
	r.MustRegister(regionOps()...)
	r.MustRegister(compositeOps()...)
	r.MustRegister(detectOps()...)
	r.MustRegister(healOps()...)
	r.MustRegister(presentOps()...)
	r.MustRegister(annotateOp())
	r.MustRegister(sheetOp())
	r.MustRegister(metaOps()...)
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

// frameArg reads a hidden [x, y, w, h] argument as a frame.
func frameArg(a graph.Args, name string) graph.Frame {
	l := a.Numbers(name)
	if len(l) != 4 {
		return graph.Frame{}
	}
	return graph.Frame{X: int(l[0]), Y: int(l[1]), W: int(l[2]), H: int(l[3])}
}
