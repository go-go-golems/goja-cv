package buffer

import (
	"context"
	"math"

	"github.com/go-go-golems/goja-cv/pkg/cv/parallel"
	"github.com/pkg/errors"
)

// Pyramid holds a full-resolution buffer and area-averaged halvings of it.
// Levels[0] is the original at scale 1; each following level halves the
// previous one. Previews resample from the coarsest level that is still at
// least as large as the target, so they never read the full buffer.
type Pyramid struct {
	Levels []*Buffer
}

// NewPyramid builds a pyramid from full (which becomes Levels[0]) down to a
// level whose long side is at most minLong pixels.
func NewPyramid(ctx context.Context, full *Buffer, minLong int) (*Pyramid, error) {
	if minLong < 1 {
		minLong = 1
	}
	p := &Pyramid{Levels: []*Buffer{full}}
	cur := full
	for max(cur.W, cur.H) > minLong && cur.W > 1 && cur.H > 1 {
		next, err := Resize(ctx, cur, (cur.W+1)/2, (cur.H+1)/2)
		if err != nil {
			return nil, err
		}
		next.Scale = cur.Scale * float64(next.W) / float64(cur.W)
		p.Levels = append(p.Levels, next)
		cur = next
	}
	return p, nil
}

// FullW is the full-resolution width.
func (p *Pyramid) FullW() int { return p.Levels[0].W }

// FullH is the full-resolution height.
func (p *Pyramid) FullH() int { return p.Levels[0].H }

// Bytes is the memory held by all levels.
func (p *Pyramid) Bytes() int64 {
	var n int64
	for _, l := range p.Levels {
		n += l.Bytes()
	}
	return n
}

// ScaledSize is the buffer size of a doc-sized image evaluated at scale.
func ScaledSize(docW, docH int, scale float64) (int, int) {
	w := int(math.Round(float64(docW) * scale))
	h := int(math.Round(float64(docH) * scale))
	return max(w, 1), max(h, 1)
}

// AtScale returns the image resampled to scale (0 < scale <= 1). At scale 1
// it returns Levels[0] itself, which must not be modified.
func (p *Pyramid) AtScale(ctx context.Context, scale float64) (*Buffer, error) {
	if scale <= 0 || scale > 1 {
		return nil, errors.Errorf("scale %g out of range (0, 1]", scale)
	}
	w, h := ScaledSize(p.FullW(), p.FullH(), scale)
	src := p.Levels[0]
	for _, l := range p.Levels[1:] {
		if l.W >= w && l.H >= h {
			src = l
		}
	}
	if src.W == w && src.H == h {
		if src.Scale == scale {
			return src, nil
		}
		out := *src
		out.Scale = scale
		return &out, nil
	}
	out, err := Resize(ctx, src, w, h)
	if err != nil {
		return nil, err
	}
	out.Scale = scale
	return out, nil
}

// axisWeights describes, for each output sample, the contributing input
// range and weights of an area-averaging resample.
type axisWeights struct {
	start []int
	w     [][]float32
}

func makeAxisWeights(src, dst int) axisWeights {
	aw := axisWeights{start: make([]int, dst), w: make([][]float32, dst)}
	ratio := float64(src) / float64(dst)
	for o := 0; o < dst; o++ {
		a, b := float64(o)*ratio, float64(o+1)*ratio
		if ratio < 1 { // upsampling: nearest input pixel
			c := int(math.Min(math.Floor((a+b)/2), float64(src-1)))
			aw.start[o], aw.w[o] = c, []float32{1}
			continue
		}
		i0 := int(math.Floor(a))
		i1 := int(math.Ceil(b))
		if i1 > src {
			i1 = src
		}
		ws := make([]float32, i1-i0)
		for i := i0; i < i1; i++ {
			lo, hi := math.Max(a, float64(i)), math.Min(b, float64(i+1))
			ws[i-i0] = float32((hi - lo) / ratio)
		}
		aw.start[o], aw.w[o] = i0, ws
	}
	return aw
}

// Resize resamples b to w×h by area averaging (box filter with fractional
// coverage) when shrinking, and nearest neighbour when enlarging. The scale of
// the result is b.Scale times the width ratio.
func Resize(ctx context.Context, b *Buffer, w, h int) (*Buffer, error) {
	if w < 1 || h < 1 {
		return nil, errors.Errorf("invalid resize target %dx%d", w, h)
	}
	xw := makeAxisWeights(b.W, w)
	yw := makeAxisWeights(b.H, h)
	tmp := New(w, b.H, b.C, b.Scale) // horizontal pass
	for c := 0; c < b.C; c++ {
		src, dst := b.Plane(c), tmp.Plane(c)
		if err := parallel.Rows(ctx, b.H, func(y0, y1 int) {
			for y := y0; y < y1; y++ {
				row := src[y*b.W : (y+1)*b.W]
				for o := 0; o < w; o++ {
					var s float32
					st := xw.start[o]
					for k, wt := range xw.w[o] {
						s += row[st+k] * wt
					}
					dst[y*w+o] = s
				}
			}
		}); err != nil {
			return nil, err
		}
	}
	out := New(w, h, b.C, b.Scale*float64(w)/float64(b.W))
	for c := 0; c < b.C; c++ {
		src, dst := tmp.Plane(c), out.Plane(c)
		if err := parallel.Rows(ctx, h, func(y0, y1 int) {
			for o := y0; o < y1; o++ {
				drow := dst[o*w : (o+1)*w]
				st := yw.start[o]
				for k, wt := range yw.w[o] {
					srow := src[(st+k)*w : (st+k+1)*w]
					for x := range drow {
						drow[x] += srow[x] * wt
					}
				}
			}
		}); err != nil {
			return nil, err
		}
	}
	return out, nil
}
