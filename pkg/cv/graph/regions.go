package graph

import (
	"context"
	"math"
	"sort"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
)

// Region is one connected area of a Regions value. Its geometry is in doc
// px; its pixels are kept in the raster it was found in (RegionsValue W×H at
// RegionsValue.Scale), so it can be rasterized back at any evaluation scale.
type Region struct {
	ID       int
	Box      [4]float64         // x, y, w, h in doc px
	Area     float64            // doc px²
	Centroid [2]float64         // x, y in doc px
	Measures map[string]float64 // aspect, meanL, contrast, localNoise, score, …
	Tags     map[string]string  // polarity, …
	Pix      []int32            // pixel indices into the source raster, row-major
}

// RegionsValue is an evaluated Regions node: an ordered list of regions over
// one source raster. The list order is meaningful (sortBy changes it); items
// are shared between values and must not be modified.
type RegionsValue struct {
	Items []*Region
	W, H  int     // source raster size in buffer px
	Scale float64 // scale of the source raster (buffer px per doc px)
	Frame Frame   // the frame the raster covers; region geometry is absolute in it
	// Fields is the schema of the items (stamped from the node by the
	// evaluator; nil for values built outside the graph, see Schema).
	Fields []FieldSpec
}

var _ Value = &RegionsValue{}

// NewRegions returns an empty Regions value over a w×h raster at scale
// covering frame (the frame of the image or mask the regions are found in).
func NewRegions(w, h int, scale float64, frame Frame) *RegionsValue {
	return &RegionsValue{W: w, H: h, Scale: scale, Frame: frame}
}

func (v *RegionsValue) Kind() Kind { return KindRegions }

func (v *RegionsValue) Bytes() int64 {
	n := int64(64)
	for _, r := range v.Items {
		n += 160 + 4*int64(len(r.Pix)) + 48*int64(len(r.Measures)+len(r.Tags))
	}
	return n
}

// With returns a Regions value over the same raster with items.
func (v *RegionsValue) With(items []*Region) *RegionsValue {
	out := *v
	out.Items = items
	return &out
}

// axisScales are the effective per-axis scales of the source raster (buffer
// sizes are rounded, so they differ slightly from Scale).
func (v *RegionsValue) axisScales() (float64, float64) {
	return float64(v.W) / float64(max(1, v.Frame.W)), float64(v.H) / float64(max(1, v.Frame.H))
}

// NewRegion builds a region from pixel indices of the source raster and
// fills its doc-px geometry (box, area, centroid; absolute: the frame origin
// is added) and the aspect measure (ratio of the principal axes, ≥ 1).
func (v *RegionsValue) NewRegion(id int, pix []int32) *Region {
	sx, sy := v.axisScales()
	r := &Region{ID: id, Pix: pix, Measures: map[string]float64{}, Tags: map[string]string{}}
	if len(pix) == 0 {
		return r
	}
	minX, minY, maxX, maxY := v.W, v.H, -1, -1
	var mx, my float64
	for _, p := range pix {
		x, y := int(p)%v.W, int(p)/v.W
		minX, maxX = min(minX, x), max(maxX, x)
		minY, maxY = min(minY, y), max(maxY, y)
		mx += float64(x)
		my += float64(y)
	}
	n := float64(len(pix))
	mx, my = mx/n, my/n
	var cxx, cyy, cxy float64
	for _, p := range pix {
		dx, dy := (float64(int(p)%v.W)-mx)/sx, (float64(int(p)/v.W)-my)/sy
		cxx += dx * dx
		cyy += dy * dy
		cxy += dx * dy
	}
	cxx, cyy, cxy = cxx/n+1/12.0, cyy/n+1/12.0, cxy/n // + pixel extent
	tr, det := cxx+cyy, cxx*cyy-cxy*cxy
	disc := math.Sqrt(math.Max(0, tr*tr/4-det))
	l1, l2 := tr/2+disc, math.Max(tr/2-disc, 1e-9)
	ox, oy := float64(v.Frame.X), float64(v.Frame.Y)
	r.Box = [4]float64{ox + float64(minX)/sx, oy + float64(minY)/sy, float64(maxX+1-minX) / sx, float64(maxY+1-minY) / sy}
	r.Area = n / (sx * sy)
	r.Centroid = [2]float64{ox + (mx+0.5)/sx, oy + (my+0.5)/sy}
	r.Measures["aspect"] = math.Sqrt(l1 / l2)
	return r
}

// Field returns a field of the region: a number, or a string for tags.
func (r *Region) Field(name string) (any, bool) {
	switch name {
	case "id":
		return float64(r.ID), true
	case "area":
		return r.Area, true
	case "x":
		return r.Box[0], true
	case "y":
		return r.Box[1], true
	case "w":
		return r.Box[2], true
	case "h":
		return r.Box[3], true
	case "cx":
		return r.Centroid[0], true
	case "cy":
		return r.Centroid[1], true
	}
	if m, ok := r.Measures[name]; ok {
		return m, true
	}
	if t, ok := r.Tags[name]; ok {
		return t, true
	}
	return nil, false
}

func round(v float64, digits int) float64 {
	p := math.Pow(10, float64(digits))
	return math.Round(v*p) / p
}

// Schema is the value's field list: Fields (stamped from its node), or for
// a value built without a node the geometry and the first item's measures
// and tags in name order.
func (v *RegionsValue) Schema() []FieldSpec {
	if v.Fields != nil {
		return v.Fields
	}
	out := WithFields(GeometryFields)
	if len(v.Items) == 0 {
		return out
	}
	r := v.Items[0]
	var names []string
	for k := range r.Measures {
		if _, geo := FindField(GeometryFields, k); !geo {
			names = append(names, k)
		}
	}
	for k := range r.Tags {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		t := "number"
		if _, ok := r.Tags[k]; ok {
			t = "string"
		}
		out = append(out, FieldSpec{Name: k, Type: t})
	}
	return out
}

// Data is the plain-data form JavaScript and data out see: the fields of
// the schema, flat and in schema order ({ id, x, y, w, h, cx, cy, area,
// aspect, …measures, …tags }). Geometry is rounded to 0.01 doc px, measures
// to 4 decimals. A field the region lacks is omitted.
func (r *Region) Data(fields []FieldSpec) Record {
	rec := Record{Keys: make([]string, 0, len(fields)), Vals: make(map[string]any, len(fields))}
	for _, f := range fields {
		v, ok := r.Field(f.Name)
		if !ok {
			continue
		}
		switch x := v.(type) {
		case float64:
			switch f.Name {
			case "id":
			case "x", "y", "w", "h", "cx", "cy", "area":
				v = round(x, 2)
			default:
				v = round(x, 4)
			}
		}
		rec.Keys = append(rec.Keys, f.Name)
		rec.Vals[f.Name] = v
	}
	return rec
}

// MaxRegionItems is a safety limit on the items of an awaited Regions value:
// a pathological mask (every grain a region) would otherwise become a
// multi-gigabyte JavaScript array.
const MaxRegionItems = 100000

// Summary is what JavaScript receives for an awaited Regions node: the count
// and every item in list order. More than MaxRegionItems items is
// E_TOO_MANY.
func (v *RegionsValue) Summary() (map[string]any, error) {
	if len(v.Items) > MaxRegionItems {
		return nil, Errorf(ETooMany, "%d regions; await at most %d.", len(v.Items), MaxRegionItems).
			WithHint("Keep the regions you need before awaiting: rank and cut, or count them without awaiting the items.").
			WithFix(`regions.sortBy("area").top(1000)   // or await regions.count()`)
	}
	fields := v.Schema()
	items := make([]any, len(v.Items))
	for i, r := range v.Items {
		items[i] = r.Data(fields)
	}
	return map[string]any{
		"kind": string(KindRegions), "count": float64(len(v.Items)), "items": items,
		"scale": v.Scale, "frame": v.Frame.Data(),
	}, nil
}

// Raster is where a buffer lies in doc px: the doc position of its
// top-left corner, its per-axis scale (buffer px per doc px) and its size.
type Raster struct {
	OX, OY float64
	SX, SY float64
	W, H   int
}

// RasterOf is the raster of an image value: its buffer origin, axis scales
// and size.
func RasterOf(v *ImageValue) Raster {
	ox, oy := v.BufferOrigin()
	sx, sy := AxisScales(v)
	return Raster{OX: ox, OY: oy, SX: sx, SY: sy, W: v.Buf.W, H: v.Buf.H}
}

// FrameRaster is the raster of a frame evaluated at scale.
func FrameRaster(f Frame, scale float64) Raster {
	w, h := buffer.ScaledSize(f.W, f.H, scale)
	return Raster{OX: float64(f.X), OY: float64(f.Y), SX: float64(w) / float64(max(1, f.W)), SY: float64(h) / float64(max(1, f.H)), W: w, H: h}
}

// Rasterize draws the regions into a 1-channel mask on target (any window
// and scale: a render window at zoom 8, a crop, the regions' own frame at a
// preview scale). Each source pixel adds its exact area overlap with each
// target pixel, so a region keeps its doc area at any scale and is
// soft-edged when the target is coarser than the source; pixels outside the
// target are skipped. Values are clamped to [0, 1].
func (v *RegionsValue) Rasterize(ctx context.Context, t Raster) (*buffer.Buffer, error) {
	w, h := t.W, t.H
	out := buffer.New(w, h, 1, t.SX)
	pl := out.Plane(0)
	sx, sy := v.axisScales()
	same := w == v.W && h == v.H && t.OX == float64(v.Frame.X) && t.OY == float64(v.Frame.Y) && t.SX == sx && t.SY == sy
	// source pixel x covers doc [Frame.X + x/sx, Frame.X + (x+1)/sx): in target px, (doc − OX)·SX
	ax, bx := t.SX/sx, (float64(v.Frame.X)-t.OX)*t.SX
	ay, by := t.SY/sy, (float64(v.Frame.Y)-t.OY)*t.SY
	for k, r := range v.Items {
		if k%64 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		for _, p := range r.Pix {
			x, y := int(p)%v.W, int(p)/v.W
			if same {
				pl[int(p)] = 1
				continue
			}
			x0, x1 := float64(x)*ax+bx, float64(x+1)*ax+bx
			y0, y1 := float64(y)*ay+by, float64(y+1)*ay+by
			if x1 <= 0 || y1 <= 0 || x0 >= float64(w) || y0 >= float64(h) {
				continue
			}
			for ty := max(0, int(math.Floor(y0))); ty < int(math.Ceil(y1)) && ty < h; ty++ {
				oy := math.Min(y1, float64(ty+1)) - math.Max(y0, float64(ty))
				if oy <= 0 {
					continue
				}
				for tx := max(0, int(math.Floor(x0))); tx < int(math.Ceil(x1)) && tx < w; tx++ {
					ox := math.Min(x1, float64(tx+1)) - math.Max(x0, float64(tx))
					if ox <= 0 {
						continue
					}
					pl[ty*w+tx] += float32(ox * oy)
				}
			}
		}
	}
	for i, p := range pl {
		if p > 1 {
			pl[i] = 1
		}
	}
	return out, nil
}
