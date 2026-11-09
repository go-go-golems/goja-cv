package ops

import (
	"context"
	"fmt"
	"math"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// sourceEnc: an opened file has the encoding the call gives, else the one
// its decoder reports; the node holds linear values (the encoding undone).
var sourceEnc = &graph.ResultEnc{
	Enc: func(a graph.Args) buffer.Encoding {
		if a.Has("encoding") {
			e, _ := buffer.ParseEncoding(a.String("encoding"))
			return e
		}
		if e := graph.Meta[graph.SourceMeta](a).FileEncoding; e != buffer.EncNone {
			return e
		}
		return buffer.EncSRGB
	},
	Space: func(graph.Args, buffer.Space) buffer.Space { return buffer.SpaceLinear },
}

// openFrame is the frame of an opened file: a window keeps its file
// coordinates (box), a file that is itself a window has its declared
// (origin) or recorded origin.
func openFrame(a graph.Args) graph.Frame {
	info := graph.Meta[graph.SourceMeta](a).Info
	f := graph.Frame{W: info.W, H: info.H}
	if info.Origin != nil {
		f.X, f.Y = info.Origin[0], info.Origin[1]
	}
	switch {
	case a.Has("box"):
		b := a.Numbers("box")
		f.X, f.Y, f.W, f.H = f.X+int(b[0]), f.Y+int(b[1]), int(b[2]), int(b[3])
	case a.Has("origin"):
		o := a.Numbers("origin")
		f.X, f.Y = int(o[0]), int(o[1])
	}
	return f
}

func validateOpen(a graph.Args) error {
	if a.Has("box") && a.Has("origin") {
		return graph.Errorf(graph.EArgType, "cv.open: box already sets the origin; pass box or origin, not both.").
			WithHint("box reads a window in file px and the frame starts at its corner; origin declares where a file that is itself a window starts.").
			WithFix(`cv.open(path, { box: [x, y, w, h] })`)
	}
	isInt := func(l []float64) bool {
		for _, v := range l {
			if v != math.Trunc(v) {
				return false
			}
		}
		return true
	}
	if a.Has("origin") && !isInt(a.Numbers("origin")) {
		return graph.Errorf(graph.EArgType, "cv.open.origin must be whole pixels, got %v.", a.Numbers("origin")).WithFix(`cv.open(path, { origin: [1536, 1536] })`)
	}
	if a.Has("box") {
		b := a.Numbers("box")
		info := graph.Meta[graph.SourceMeta](a).Info
		w, h := info.W, info.H
		if !isInt(b) {
			return graph.Errorf(graph.EArgType, "cv.open.box must be whole file pixels, got %v.", b).
				WithFix(fmt.Sprintf("cv.open(path, { box: [%d, %d, %d, %d] })", int(b[0]), int(b[1]), int(math.Ceil(b[2])), int(math.Ceil(b[3]))))
		}
		if b[2] < 1 || b[3] < 1 || b[0] < 0 || b[1] < 0 || b[0]+b[2] > float64(w) || b[1]+b[3] > float64(h) {
			nb := graph.Frame{W: w, H: h}.NearestBox(b[0], b[1], math.Max(1, math.Min(b[2], float64(w))), math.Max(1, math.Min(b[3], float64(h))))
			return graph.Errorf(graph.EArgRange, "cv.open.box = [%g, %g, %g, %g] is not inside the file, which is %d×%d px ([0, 0, %d, %d]).", b[0], b[1], b[2], b[3], w, h, w, h).
				WithHint("box is a window in file px: [x, y, w, h] with x + w ≤ width and y + h ≤ height.").
				WithFix(fmt.Sprintf("cv.open(path, { box: [%g, %g, %g, %g] })", nb[0], nb[1], nb[2], nb[3]))
		}
	}
	return nil
}

// placeholderEnc gives a builder lambda's placeholder the statics of the
// receiver it stands for, so the lambda's graph resolves as on the receiver.
var placeholderEnc = &graph.ResultEnc{
	Enc: func(a graph.Args) buffer.Encoding { return graph.Meta[graph.PlaceholderMeta](a).Enc },
	Space: func(a graph.Args, _ buffer.Space) buffer.Space {
		return graph.Meta[graph.PlaceholderMeta](a).Space
	},
}

func sourceOps() []graph.OpDef {
	return []graph.OpDef{
		{
			Name: "open", On: graph.KindNone, Returns: graph.KindImage, Result: sourceEnc, Bespoke: true,
			Args: []graph.ArgSpec{
				{Name: "path", Type: "string", Positional: true, Doc: "image file: " + buffer.SupportedFormats},
				{Name: "encoding", Type: "enum:srgb,linear", Optional: true, ValueSynonyms: graph.EncodingValueSynonyms,
					Doc: "how to read the file's values: srgb (photographs) or linear (scientific data, raw scans, density maps); omit it to use what the file says"},
				{Name: "box", Type: "box", Optional: true,
					Doc: "read only this window [x, y, w, h] of the file (file px); the image keeps the file's coordinates"},
				{Name: "origin", Type: "point", Optional: true,
					Doc: "declare where a file that is itself a window starts [x, y] (doc px); omit it for whole files"},
			},
			DocFrame: openFrame,
			Validate: validateOpen,
			Run: func(ctx context.Context, ec *graph.EvalCtx, _ []graph.Value, a graph.Args) (graph.Value, error) {
				m := graph.Meta[graph.SourceMeta](a)
				k := graph.SourceKey{Path: m.Path, Stamp: m.Stamp, Enc: ec.Node.Enc}
				if a.Has("box") {
					b := a.Numbers("box")
					k.Box = &[4]int{int(b[0]), int(b[1]), int(b[2]), int(b[3])}
				}
				src, err := ec.Eval.Sources().Load(ctx, k)
				if err != nil {
					return nil, err
				}
				buf, err := src.Pyr.AtScale(ctx, ec.Scale)
				if err != nil {
					return nil, err
				}
				return graph.NewImage(buf), nil
			},
			Doc: "Open an image file as a source node: cv.open(path, { encoding, box, origin }). Decoded on first evaluation; the node holds linear values (the file's encoding undone) in the file's coordinates.",
			See: []string{"stats", "explain"},
		},
		{
			Name: graph.PlaceholderOp, On: graph.KindNone, Returns: graph.KindImage, Result: placeholderEnc, Bespoke: true,
			DocFrame: func(a graph.Args) graph.Frame { return graph.Meta[graph.PlaceholderMeta](a).Frame },
			Run: func(context.Context, *graph.EvalCtx, []graph.Value, graph.Args) (graph.Value, error) {
				return nil, graph.Errorf(graph.EPlaceholder, "This is the placeholder image a within() function receives; it only stands for the masked image while the function builds its edit.").
					WithHint("Use the function's return value. Do not keep the placeholder in a variable outside the function.").
					WithFix("img.within(mask, i => i.exposure(-0.4))")
			},
			Doc: "The placeholder Image a builder lambda (within) receives; replaced by the real receiver when the lambda returns.",
			See: []string{"within"},
		},
	}
}
