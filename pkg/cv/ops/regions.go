package ops

import (
	"context"
	"sort"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

func regions(v graph.Value) *graph.RegionsValue { return v.(*graph.RegionsValue) }

// analysisScaleArg is the static argument of Regions-producing ops that
// fixes the scale they analyse at (see OpDef.AnalysisScale).
var analysisScaleArg = graph.ArgSpec{Name: "analysisScale", Type: "number", Default: 1.0, Min: graph.F(0.05), Max: graph.F(1),
	Doc: "scale the input is analysed at, whatever the evaluation scale, so region ids and doc-px coordinates are the same in a look and a measurement"}

func analysisScale(a graph.Args) float64 {
	f, _ := a.Raw("analysisScale").(float64)
	return f
}

// sortByArea orders regions by area (largest first), then id.
func sortByArea(items []*graph.Region) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Area != items[j].Area {
			return items[i].Area > items[j].Area
		}
		return items[i].ID < items[j].ID
	})
}

// itemsCover checks that items (a Regions value) can be used on an image of
// frame f: literal items have no frame; a region set must cover f (regions
// found on the whole image fit a crop of it).
func itemsCover(op string, rv *graph.RegionsValue, f graph.Frame) error {
	if rv.Frame.Empty() {
		return nil
	}
	return containsFrame(op, rv.Frame, f)
}

// itemsArg is the Items argument of an op that takes regions.
func itemsArg(name, typ, doc string) graph.ArgSpec {
	return graph.ArgSpec{Name: name, Type: typ, Positional: true, Doc: doc}
}

func regionOps() []graph.OpDef {
	return []graph.OpDef{
		{
			Name: "components", On: graph.KindImage, Returns: graph.KindRegions, Global: true, OneChannel: true,
			Args: []graph.ArgSpec{
				{Name: "minArea", Type: "number", Unit: graph.DocPx2, Default: 1.0, Min: graph.F(0), Doc: "smallest region kept, in doc px²",
					Redirects: map[string]graph.Redirect{"maxArea": {Hint: "components keeps every region from minArea up; drop large ones in JavaScript after awaiting.",
						Fix: "(await m.components()).filter(r => r.area <= 200)"}}},
				{Name: "threshold", Type: "number", Default: 0.5, Min: graph.F(0), Max: graph.F(1), Doc: "mask values above this are selected"},
				analysisScaleArg,
			},
			AnalysisScale: analysisScale,
			Run: func(ctx context.Context, ec *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				m := image(in[0])
				pl := m.Buf.Plane(0)
				thr := float32(a.Number("threshold"))
				fg := make([]bool, len(pl))
				for i, v := range pl {
					fg[i] = v > thr
				}
				comps, err := kernels.Components(ctx, fg, m.Buf.W, m.Buf.H)
				if err != nil {
					return nil, err
				}
				rv := graph.NewRegions(m.Buf.W, m.Buf.H, ec.Scale, m.Frame)
				minPx := a.Area("minArea", ec.Scale)
				for _, pix := range comps {
					if float64(len(pix)) < minPx {
						continue
					}
					rv.Items = append(rv.Items, rv.NewRegion(len(rv.Items)+1, pix))
				}
				sortByArea(rv.Items)
				return rv, nil
			},
			Doc:    "The connected (8-neighbour) areas of the mask as Regions, largest first; ids follow raster order. Awaited: an array of items.",
			Fields: geometryFields,
			See:    []string{"mask.regions", "measure"},
		},
		{
			Name: "mask.regions", On: graph.KindImage, Returns: graph.KindImage, Result: graph.DerivedEnc, Channels: graph.One,
			Args: []graph.ArgSpec{
				itemsArg("items", graph.ItemsType, "regions: a Regions node or an array of items (region items, boxes { x, y, w, h }, circles { cx, cy, r })"),
			},
			Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
				src := image(in[0])
				rv := regions(a.Value("items"))
				if err := itemsCover("mask.regions", rv, src.Frame); err != nil {
					return nil, err
				}
				m, err := rv.Rasterize(ctx, graph.RasterOf(src))
				if err != nil {
					return nil, err
				}
				return graph.NewImage(m), nil
			},
			Doc: "The items as a mask (a one-channel image in the image's frame): exact area coverage of each region's pixels, box or circle. Grow it with .grow(r).",
			See: []string{"components", "grow", "heal"},
		},
	}
}
