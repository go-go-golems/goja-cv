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
	Enc: func(a graph.Args, _ []*graph.Node) buffer.Encoding {
		name := a.String("fileEncoding")
		if a.Has("encoding") {
			name = a.String("encoding")
		}
		e, ok := buffer.ParseEncoding(name)
		if !ok {
			return buffer.EncSRGB
		}
		return e
	},
	Space: func(graph.Args, buffer.Space, []*graph.Node) buffer.Space { return buffer.SpaceLinear },
}

// openFrame is the frame of an opened file: a window keeps its file
// coordinates (box), a file that is itself a window has its declared
// (origin) or recorded origin.
func openFrame(a graph.Args, _ []*graph.Node) graph.Frame {
	f := graph.Frame{W: a.Int("w"), H: a.Int("h")}
	if a.Has("recordedOrigin") {
		o := a.Numbers("recordedOrigin")
		f.X, f.Y = int(o[0]), int(o[1])
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
		w, h := a.Int("w"), a.Int("h")
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
	Enc: func(a graph.Args, _ []*graph.Node) buffer.Encoding {
		e, _ := buffer.ParseEncoding(a.String("enc"))
		return e
	},
	Space: func(a graph.Args, _ buffer.Space, _ []*graph.Node) buffer.Space {
		s, _ := buffer.ParseSpace(a.String("space"))
		return s
	},
}

func sourceOps() []graph.OpDef {
	return []graph.OpDef{
		{
			Name: "open", On: graph.KindNone, Returns: graph.KindImage, Result: sourceEnc,
			Args: []graph.ArgSpec{
				{Name: "path", Type: "string", Positional: true, Doc: "image file: " + buffer.SupportedFormats},
				{Name: "encoding", Type: "enum:srgb,linear", Static: true, Optional: true, ValueSynonyms: graph.EncodingValueSynonyms,
					Doc: "how to read the file's values: srgb (photographs) or linear (scientific data, raw scans, density maps); omit it to use what the file says"},
				{Name: "box", Type: "box", Static: true, Optional: true,
					Doc: "read only this window [x, y, w, h] of the file (file px); the image keeps the file's coordinates"},
				{Name: "origin", Type: "point", Static: true, Optional: true,
					Doc: "declare where a file that is itself a window starts [x, y] (doc px); omit it for whole files"},
				{Name: "stamp", Type: "string", Hidden: true},
				{Name: "w", Type: "number", Hidden: true},
				{Name: "h", Type: "number", Hidden: true},
				{Name: "format", Type: "string", Hidden: true, Optional: true},
				{Name: "bits", Type: "number", Hidden: true, Optional: true},
				{Name: "channels", Type: "number", Hidden: true, Optional: true},
				{Name: "sample", Type: "string", Hidden: true, Optional: true},
				{Name: "fileEncoding", Type: "string", Hidden: true, Optional: true},
				{Name: "recordedOrigin", Type: "number[]", Hidden: true, Optional: true},
				{Name: "notes", Type: "string[]", Hidden: true, Optional: true},
			},
			DocFrame: openFrame,
			Validate: validateOpen,
			Run: func(ctx context.Context, ec *graph.EvalCtx, _ []graph.Value, a graph.Args) (graph.Value, error) {
				k := graph.SourceKey{Path: a.String("path"), Stamp: a.String("stamp"), Enc: ec.Node.Enc}
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
			Name: "param", On: graph.KindNone, Returns: graph.KindParam,
			Args: []graph.ArgSpec{{Name: "name", Type: "string", Positional: true}},
			Run: func(_ context.Context, ec *graph.EvalCtx, _ []graph.Value, a graph.Args) (graph.Value, error) {
				v, ok := ec.Params[a.String("name")]
				if !ok {
					return nil, graph.Errorf(graph.EArgType, "param %q is not declared.", a.String("name"))
				}
				return data(v), nil
			},
			Doc: "A named value a person can move (declare with P.slider / P.toggle / P.choice).",
		},
		{
			Name: graph.PlaceholderOp, On: graph.KindNone, Returns: graph.KindImage, Result: placeholderEnc,
			Args: []graph.ArgSpec{
				{Name: "for", Type: "number", Hidden: true},
				{Name: "frame", Type: "number[]", Hidden: true, Doc: "the receiver's frame [x, y, w, h]"},
				{Name: "enc", Type: "string", Hidden: true, Optional: true, Doc: "the receiver's encoding"},
				{Name: "space", Type: "string", Hidden: true, Optional: true, Doc: "the receiver's space"},
			},
			DocFrame: func(a graph.Args, _ []*graph.Node) graph.Frame { return frameArg(a, "frame") },
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
