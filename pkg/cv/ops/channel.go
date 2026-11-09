package ops

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

var channelIndex = map[string]int{"r": 0, "g": 1, "b": 2}

// channelOf reads channel("r" | "g" | "b" | 0 | 1 | 2) as an index.
func channelOf(a graph.Args) int {
	switch v := a.Raw("c").(type) {
	case string:
		return channelIndex[v]
	case float64:
		return int(v)
	}
	return 0
}

// encodeEnc: encode's result has the encoding it names; its values are the
// same linear values.
var encodeEnc = &graph.ResultEnc{
	Enc: func(a graph.Args) buffer.Encoding {
		e, _ := buffer.ParseEncoding(a.String("to"))
		return e
	},
	Space: func(graph.Args, buffer.Space) buffer.Space { return buffer.SpaceLinear },
}

// SourceNotes returns the notes of the files a node was computed from.
func SourceNotes(b *graph.Builder, n *graph.Node) []string {
	seen := map[graph.NodeID]bool{}
	var out []string
	var walk func(n *graph.Node)
	walk = func(n *graph.Node) {
		if seen[n.ID] {
			return
		}
		seen[n.ID] = true
		if n.Op.On == graph.KindNone && n.Op.Name == "open" {
			out = append(out, graph.Meta[graph.SourceMeta](n.Args).Info.Notes...)
		}
		for _, id := range n.Inputs {
			if in, ok := b.Node(id); ok {
				walk(in)
			}
		}
	}
	walk(n)
	return out
}

func channelOps() []graph.OpDef {
	return []graph.OpDef{
		{
			Name: "channel", On: graph.KindImage, Returns: graph.KindImage,
			Args: []graph.ArgSpec{{Name: "c", Type: "enum:r,g,b|number", Positional: true, Min: graph.F(0), Max: graph.F(2),
				Doc: `the channel: "r", "g", "b" or 0, 1, 2`}},
			Validate: func(a graph.Args) error {
				if f, ok := a.Raw("c").(float64); ok && f != math.Trunc(f) {
					return graph.Errorf(graph.EArgType, "channel(%g): the channel is 0, 1 or 2.", f).WithValid([]string{"r", "g", "b", "0", "1", "2"})
				}
				return nil
			},
			Run: func(_ context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
				c := channelOf(a)
				if c >= src.Buf.C {
					return nil, graph.Errorf(graph.EArgRange, "channel(%v): the image has %d channel(s).", a.Raw("c"), src.Buf.C).
						WithHint("A one-channel image is already grey; its only channel is \"r\" (0).").
						WithValid([]string{"r", "0"}).WithFix("img.toGray()")
				}
				out := buffer.NewLikeC(src.Buf, 1)
				copy(out.Pix, src.Buf.Plane(c))
				return graph.NewImage(out), nil
			},
			Doc: `One channel as a one-channel image: channel("g"). Keeps the encoding and space (plane selection).`,
			See: []string{"split", "toGray", "chroma"},
		},
		{
			Name: "split", On: graph.KindImage, Returns: graph.KindImage,
			Expand: func(x *graph.Expander, recv *graph.Node, _ graph.Args) ([]*graph.Node, error) {
				out := make([]*graph.Node, 3)
				for i, c := range []string{"r", "g", "b"} {
					n, err := x.Call("channel", recv, c)
					if err != nil {
						return nil, err
					}
					out[i] = n
				}
				return out, nil
			},
			Doc: "The three channels as one-channel images: const [r, g, b] = img.split().",
			See: []string{"channel"},
		},
		{
			Name: "toLab", On: graph.KindImage, Returns: graph.KindImage,
			Space:  graph.SpaceRule{Default: buffer.SpaceLinear, Fixed: true},
			Result: graph.DerivedEnc,
			Warn: func(b *graph.Builder, a graph.Args) []graph.Warning {
				for _, note := range SourceNotes(b, a.Receiver()) {
					if strings.Contains(note, "camera RGB primaries") {
						return []graph.Warning{{Code: WPrimaries, Message: "toLab: a*/b* are approximate: the DNG's colour matrices are not applied (camera RGB is treated as sRGB primaries)."}}
					}
				}
				return nil
			},
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, _ graph.Args) (graph.Value, error) {
				out, err := kernels.ToLab(ctx, image(in[0]).Buf)
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out), nil
			},
			Doc: "CIE L*a*b* of linear light (sRGB/Rec. 709 primaries, D65), as L*/100, a*/100, b*/100 in three channels; a derived image (its values are shown and measured as they are).",
			See: []string{"chroma", "split"},
		},
		{
			Name: "chroma", On: graph.KindImage, Returns: graph.KindImage,
			Space:  graph.SpaceRule{Default: buffer.SpaceEncoded},
			Result: graph.DerivedEnc,
			Args: []graph.ArgSpec{{Name: "relative", Type: "bool", Default: false,
				Doc: "divide by the channel mean + 0.001, so equal colour casts score alike in shadows and highlights"}},
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				out, err := kernels.Chroma(ctx, image(in[0]).Buf, a.Bool("relative"))
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out), nil
			},
			Doc: "Per-pixel channel spread max(R, G, B) − min(R, G, B) in the working space (encoded by default): 0 for neutral (grey) pixels. A derived one-channel image (not CIE chroma).",
			See: []string{"colormap", "toLab", "channel"},
		},
		{
			Name: "colormap", On: graph.KindImage, Returns: graph.KindImage,
			Space:  graph.SpaceRule{Default: buffer.SpaceEncoded},
			Result: graph.DisplayEnc,
			Args: []graph.ArgSpec{
				{Name: "name", Type: "enum:" + strings.Join(kernels.ColormapNames, ","), Positional: true, Default: "viridis",
					Doc: "gray, viridis, magma (sequential) or diverging (blue–white–red, for signed maps)"},
				{Name: "range", Type: "number[]", Unit: graph.Level, Len: 2, Optional: true,
					Doc: "[lo, hi] levels mapped to the ends; default: the image's [p01, p99] (diverging: ± the larger magnitude around center)"},
				{Name: "center", Type: "number", Unit: graph.Level, Default: 0.0, Doc: "diverging: the level shown white"},
			},
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
				if src.Buf.C != 1 {
					return nil, graph.Errorf(graph.EWrongKind, "colormap needs a one-channel image; this one has %d channels.", src.Buf.C).
						WithHint("Pick a channel, or reduce the image to grey first.").
						WithValid([]string{`.channel("g")`, `.toGray("average")`}).
						WithFix(fmt.Sprintf(`img.toGray("average").colormap(%q)`, a.String("name")))
				}
				name := a.String("name")
				var lo, hi float64
				if a.Has("range") {
					r := a.Numbers("range")
					lo, hi = r[0], r[1]
				} else {
					s, err := kernels.Stats(ctx, src.Buf, nil)
					if err != nil {
						return nil, err
					}
					lo, hi = s.L.P01, s.L.P99
					if name == "diverging" {
						c := a.Number("center")
						m := math.Max(math.Abs(lo-c), math.Abs(hi-c))
						lo, hi = c-m, c+m
					}
				}
				if !(hi > lo) {
					hi = lo + 1e-6
				}
				out, err := kernels.ApplyColormap(ctx, src.Buf, kernels.Colormaps[name], lo, hi)
				if err != nil {
					return nil, err
				}
				return graph.NewImage(out), nil
			},
			Doc: "False colour for a one-channel image: colormap(\"viridis\" | \"magma\" | \"gray\" | \"diverging\", { range, center }). Without range the image's [p01, p99] at the evaluation scale (a preview and a full-scale look can differ slightly). The result holds display colours.",
			See: []string{"chroma", "channel"},
		},
		{
			Name: "encode", On: graph.KindImage, Returns: graph.KindImage,
			Space:  graph.SpaceRule{Default: buffer.SpaceLinear, Fixed: true},
			Result: encodeEnc,
			Args: []graph.ArgSpec{{Name: "to", Type: "enum:srgb,linear", Static: true, Positional: true, ValueSynonyms: graph.EncodingValueSynonyms,
				Doc: "the encoding the image gets; its linear values do not change"}},
			Run: func(_ context.Context, _ *graph.EvalCtx, in []graph.Value, _ graph.Args) (graph.Value, error) {
				return in[0], nil // the same linear values; the evaluator stamps the new encoding on a copy
			},
			Doc: `Re-encode: the same linear values with another encoding, so looks and encoded-space ops see that curve. img.encode("srgb") makes a linear photograph look normal. To reinterpret a file's values, open it with { encoding } instead.`,
			See: []string{"open"},
		},
	}
}

// WPrimaries warns that Lab of camera RGB is approximate.
const WPrimaries = "PRIMARIES"
