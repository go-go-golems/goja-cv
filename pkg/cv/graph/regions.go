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
	Items      []*Region
	W, H       int     // source raster size in buffer px
	Scale      float64 // scale of the source raster (buffer px per doc px)
	DocW, DocH int
}

var _ Value = &RegionsValue{}

// NewRegions returns an empty Regions value over a w×h raster at scale.
func NewRegions(w, h int, scale float64, docW, docH int) *RegionsValue {
	return &RegionsValue{W: w, H: h, Scale: scale, DocW: docW, DocH: docH}
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
	return float64(v.W) / float64(max(1, v.DocW)), float64(v.H) / float64(max(1, v.DocH))
}

// NewRegion builds a region from pixel indices of the source raster and
// fills its doc-px geometry (box, area, centroid) and the aspect measure
// (ratio of the principal axes, ≥ 1).
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
	r.Box = [4]float64{float64(minX) / sx, float64(minY) / sy, float64(maxX+1-minX) / sx, float64(maxY+1-minY) / sy}
	r.Area = n / (sx * sy)
	r.Centroid = [2]float64{(mx + 0.5) / sx, (my + 0.5) / sy}
	r.Measures["aspect"] = math.Sqrt(l1 / l2)
	return r
}

// RegionFields are the fields every region has; ops add measures such as
// meanL, contrast, localNoise, score (detect.specks) or meanMask
// (components).
var RegionFields = []string{"id", "area", "x", "y", "w", "h", "cx", "cy", "aspect"}

// KnownRegionFields are all field names region predicates may use.
var KnownRegionFields = []string{"id", "area", "x", "y", "w", "h", "cx", "cy", "aspect",
	"meanL", "contrast", "localNoise", "score", "sharpness", "meanMask", "polarity"}

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

// FieldNames lists the region's fields.
func (r *Region) FieldNames() []string {
	out := []string{"id", "area", "x", "y", "w", "h", "cx", "cy"}
	for k := range r.Measures {
		out = append(out, k)
	}
	for k := range r.Tags {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func round(v float64, digits int) float64 {
	p := math.Pow(10, float64(digits))
	return math.Round(v*p) / p
}

// Data is the plain-data form JavaScript sees: { id, box, area, centroid,
// …measures, …tags }. Geometry is rounded to 0.01 doc px, measures to 4
// decimals.
func (r *Region) Data() map[string]any {
	m := map[string]any{
		"id":       float64(r.ID),
		"box":      []any{round(r.Box[0], 2), round(r.Box[1], 2), round(r.Box[2], 2), round(r.Box[3], 2)},
		"area":     round(r.Area, 2),
		"centroid": []any{round(r.Centroid[0], 2), round(r.Centroid[1], 2)},
	}
	for k, v := range r.Measures {
		m[k] = round(v, 4)
	}
	for k, v := range r.Tags {
		m[k] = v
	}
	return m
}

// SummaryLimit is the number of items an awaited Regions value includes.
const SummaryLimit = 100

// Summary is what JavaScript receives for an awaited Regions node: the count
// and the first limit items in list order.
func (v *RegionsValue) Summary(limit int) map[string]any {
	n := min(limit, len(v.Items))
	items := make([]any, n)
	for i := 0; i < n; i++ {
		items[i] = v.Items[i].Data()
	}
	return map[string]any{
		"kind": string(KindRegions), "count": float64(len(v.Items)), "items": items,
		"truncated": len(v.Items) > n, "scale": v.Scale, "docW": v.DocW, "docH": v.DocH,
	}
}

// Rasterize draws the regions into a 1-channel w×h mask covering the doc
// (normally buffer.ScaledSize(DocW, DocH, scale)). Each source pixel adds
// its exact area overlap with each target pixel, so a region keeps its doc
// area at any scale and is soft-edged when the target is coarser than the
// source. Values are clamped to [0, 1].
func (v *RegionsValue) Rasterize(ctx context.Context, w, h int, scale float64) (*buffer.Buffer, error) {
	out := buffer.New(w, h, 1, scale)
	pl := out.Plane(0)
	sx, sy := v.axisScales()
	rx, ry := float64(w)/float64(max(1, v.DocW))/sx, float64(h)/float64(max(1, v.DocH))/sy
	same := w == v.W && h == v.H
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
			x0, x1 := float64(x)*rx, float64(x+1)*rx
			y0, y1 := float64(y)*ry, float64(y+1)*ry
			for ty := int(math.Floor(y0)); ty < int(math.Ceil(y1)) && ty < h; ty++ {
				oy := math.Min(y1, float64(ty+1)) - math.Max(y0, float64(ty))
				if ty < 0 || oy <= 0 {
					continue
				}
				for tx := int(math.Floor(x0)); tx < int(math.Ceil(x1)) && tx < w; tx++ {
					ox := math.Min(x1, float64(tx+1)) - math.Max(x0, float64(tx))
					if tx < 0 || ox <= 0 {
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
