package kernels

import (
	"context"
	"math"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/parallel"
)

// Masks are 1-channel buffers with values in [0, 1]. All coordinates and
// lengths here are in buffer px; the op layer converts from doc px.

func clamp01f(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func smoothstep(e0, e1, x float64) float64 {
	if e1 <= e0 {
		if x >= e0 {
			return 1
		}
		return 0
	}
	t := (x - e0) / (e1 - e0)
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	return t * t * (3 - 2*t)
}

// LuminanceMask selects pixels by luminance: above and/or below a threshold,
// with a smoothstep ramp of total width softness centred on it. A nil bound
// is not applied. b is normally one channel (the graph feeds luminance in
// the receiver's space); for three channels the Rec. 709 weighted sum of
// the given values is used.
func LuminanceMask(ctx context.Context, b *buffer.Buffer, above, below *float64, softness float64) (*buffer.Buffer, error) {
	L := b.Luminance()
	out := buffer.New(b.W, b.H, 1, b.Scale)
	m := out.Plane(0)
	h := softness / 2
	err := parallel.Rows(ctx, b.H, func(y0, y1 int) {
		for i := y0 * b.W; i < y1*b.W; i++ {
			v := 1.0
			l := float64(L[i])
			if above != nil {
				if h == 0 {
					v *= b2f(l > *above)
				} else {
					v *= smoothstep(*above-h, *above+h, l)
				}
			}
			if below != nil {
				if h == 0 {
					v *= b2f(l < *below)
				} else {
					v *= 1 - smoothstep(*below-h, *below+h, l)
				}
			}
			m[i] = float32(v)
		}
	})
	return out, err
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// HueSat returns hue (degrees, [0, 360)) and HSV saturation of an RGB
// triple.
func HueSat(r, g, b float64) (float64, float64) {
	mx := math.Max(r, math.Max(g, b))
	mn := math.Min(r, math.Min(g, b))
	d := mx - mn
	if mx <= 0 || d <= 0 {
		return 0, 0
	}
	var h float64
	switch mx {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h, d / mx
}

// ColorMask selects pixels whose hue is within tol degrees of hue
// (soft over a further tol/4) and whose saturation is at least minSat (soft
// over [minSat/2, minSat]). A 1-channel image selects nothing.
func ColorMask(ctx context.Context, b *buffer.Buffer, hue, tol, minSat float64) (*buffer.Buffer, error) {
	out := buffer.New(b.W, b.H, 1, b.Scale)
	if b.C < 3 {
		return out, nil
	}
	m := out.Plane(0)
	R, G, B := b.Plane(0), b.Plane(1), b.Plane(2)
	soft := math.Max(tol/4, 1)
	err := parallel.Rows(ctx, b.H, func(y0, y1 int) {
		for i := y0 * b.W; i < y1*b.W; i++ {
			h, s := HueSat(float64(R[i]), float64(G[i]), float64(B[i]))
			d := math.Abs(math.Mod(h-hue+540, 360) - 180)
			wh := math.Max(0, math.Min(1, (tol+soft-d)/soft))
			ws := 1.0
			if minSat > 0 {
				ws = math.Max(0, math.Min(1, (s-minSat/2)/(minSat/2)))
			}
			m[i] = float32(wh * ws)
		}
	})
	return out, err
}

// overlap1 is the length of [a0, a1) ∩ [i, i+1).
func overlap1(a0, a1 float64, i int) float64 {
	return math.Max(0, math.Min(a1, float64(i+1))-math.Max(a0, float64(i)))
}

// BoxMask is a w×h mask with the box [x, y, bw, bh] (buffer px), each pixel
// weighted by its exact area overlap with the box.
func BoxMask(w, h int, scale, x, y, bw, bh float64) *buffer.Buffer {
	out := buffer.New(w, h, 1, scale)
	m := out.Plane(0)
	cx := make([]float64, w)
	for i := range cx {
		cx[i] = overlap1(x, x+bw, i)
	}
	for j := 0; j < h; j++ {
		oy := overlap1(y, y+bh, j)
		if oy == 0 {
			continue
		}
		for i := 0; i < w; i++ {
			m[j*w+i] = float32(cx[i] * oy)
		}
	}
	return out
}

// EllipseMask is a w×h mask with the ellipse inscribed in [x, y, bw, bh],
// anti-aliased with 4×4 supersampling.
func EllipseMask(ctx context.Context, w, h int, scale, x, y, bw, bh float64) (*buffer.Buffer, error) {
	out := buffer.New(w, h, 1, scale)
	m := out.Plane(0)
	cx, cy, rx, ry := x+bw/2, y+bh/2, bw/2, bh/2
	if rx <= 0 || ry <= 0 {
		return out, nil
	}
	err := parallel.Rows(ctx, h, func(y0, y1 int) {
		for j := y0; j < y1; j++ {
			for i := 0; i < w; i++ {
				m[j*w+i] = EllipseCoverage(i, j, cx, cy, rx, ry)
			}
		}
	})
	return out, err
}

// EllipseCoverage is the fraction of buffer pixel (i, j) (which covers
// [i, i+1) × [j, j+1)) inside the ellipse centred at (cx, cy) with radii rx,
// ry (buffer px), by 4×4 supersampling: a multiple of 1/16. It is the one
// coverage rule of mask.ellipse and of every measurement (sample, measure,
// profile, measure), so their numbers agree.
func EllipseCoverage(i, j int, cx, cy, rx, ry float64) float32 {
	if rx <= 0 || ry <= 0 {
		return 0
	}
	n := 0
	for sy := 0; sy < 4; sy++ {
		for sx := 0; sx < 4; sx++ {
			dx := (float64(i) + (float64(sx)+0.5)/4 - cx) / rx
			dy := (float64(j) + (float64(sy)+0.5)/4 - cy) / ry
			if dx*dx+dy*dy <= 1 {
				n++
			}
		}
	}
	return float32(n) / 16
}

// EdgeGradientMask is 1 at an image edge ("top", "bottom", "left",
// "right") falling linearly to 0 at fraction to of the height/width.
func EdgeGradientMask(ctx context.Context, w, h int, scale float64, from string, to float64) (*buffer.Buffer, error) {
	out := buffer.New(w, h, 1, scale)
	m := out.Plane(0)
	err := parallel.Rows(ctx, h, func(y0, y1 int) {
		for j := y0; j < y1; j++ {
			for i := 0; i < w; i++ {
				var t float64
				switch from {
				case "top":
					t = (float64(j) + 0.5) / float64(h)
				case "bottom":
					t = 1 - (float64(j)+0.5)/float64(h)
				case "left":
					t = (float64(i) + 0.5) / float64(w)
				default: // right
					t = 1 - (float64(i)+0.5)/float64(w)
				}
				m[j*w+i] = float32(math.Max(0, math.Min(1, 1-t/to)))
			}
		}
	})
	return out, err
}

// PointGradientMask is 1 at point (x0, y0) falling linearly to 0 at (x1, y1)
// (buffer px), constant across the direction of the gradient.
func PointGradientMask(ctx context.Context, w, h int, scale, x0, y0, x1, y1 float64) (*buffer.Buffer, error) {
	out := buffer.New(w, h, 1, scale)
	m := out.Plane(0)
	dx, dy := x1-x0, y1-y0
	l2 := dx*dx + dy*dy
	err := parallel.Rows(ctx, h, func(ya, yb int) {
		for j := ya; j < yb; j++ {
			for i := 0; i < w; i++ {
				t := 0.0
				if l2 > 0 {
					t = ((float64(i)+0.5-x0)*dx + (float64(j)+0.5-y0)*dy) / l2
				}
				m[j*w+i] = float32(math.Max(0, math.Min(1, 1-t)))
			}
		}
	})
	return out, err
}

// FloodMask selects the 4-connected area around (x, y) (buffer px) whose
// colour differs from the seed's by at most tol in every channel.
func FloodMask(ctx context.Context, b *buffer.Buffer, x, y int, tol float64) (*buffer.Buffer, error) {
	out := buffer.New(b.W, b.H, 1, b.Scale)
	m := out.Plane(0)
	W, H := b.W, b.H
	planes := make([][]float32, b.C)
	for c := range planes {
		planes[c] = b.Plane(c)
	}
	seed := make([]float64, b.C)
	si := y*W + x
	for c := range seed {
		seed[c] = float64(planes[c][si])
	}
	ok := func(i int) bool {
		for c := range planes {
			if math.Abs(float64(planes[c][i])-seed[c]) > tol {
				return false
			}
		}
		return true
	}
	stack := []int{si}
	m[si] = 1
	for n := 0; len(stack) > 0; n++ {
		if n%65536 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		i := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		px, py := i%W, i/W
		for _, nb := range [4][2]int{{px - 1, py}, {px + 1, py}, {px, py - 1}, {px, py + 1}} {
			if nb[0] < 0 || nb[1] < 0 || nb[0] >= W || nb[1] >= H {
				continue
			}
			j := nb[1]*W + nb[0]
			if m[j] == 0 && ok(j) {
				m[j] = 1
				stack = append(stack, j)
			}
		}
	}
	return out, nil
}

// Combine applies f pointwise to two masks of the same size.
func Combine(ctx context.Context, a, b *buffer.Buffer, f func(x, y float32) float32) (*buffer.Buffer, error) {
	out := buffer.NewLike(a)
	pa, pb, po := a.Plane(0), b.Plane(0), out.Plane(0)
	err := parallel.Rows(ctx, a.H, func(y0, y1 int) {
		for i := y0 * a.W; i < y1*a.W; i++ {
			po[i] = clamp01f(f(pa[i], pb[i]))
		}
	})
	return out, err
}

// MapMask applies f pointwise to a mask.
func MapMask(ctx context.Context, a *buffer.Buffer, f func(x float32) float32) (*buffer.Buffer, error) {
	out := buffer.NewLike(a)
	pa, po := a.Plane(0), out.Plane(0)
	err := parallel.Rows(ctx, a.H, func(y0, y1 int) {
		for i := y0 * a.W; i < y1*a.W; i++ {
			po[i] = clamp01f(f(pa[i]))
		}
	})
	return out, err
}

// Feather blurs a mask with a Gaussian of standard deviation r/2, so the
// edge ramp is about r wide on each side, and keeps it in [0, 1].
func Feather(ctx context.Context, m *buffer.Buffer, r float64) (*buffer.Buffer, error) {
	g, err := Gaussian(ctx, m, r/2)
	if err != nil {
		return nil, err
	}
	for i, v := range g.Pix {
		g.Pix[i] = clamp01f(v)
	}
	return g, nil
}

const edtInf = 1e20

// dt1d is the 1-D squared Euclidean distance transform of f (Felzenszwalb
// and Huttenlocher): d[q] = min_p (q-p)² + f[p]. v and z are scratch of
// len(f) and len(f)+1. Empty positions hold edtInf.
func dt1d(f, d []float64, v []int, z []float64) {
	n := len(f)
	parab := func(q, p int) float64 {
		return ((f[q] + float64(q*q)) - (f[p] + float64(p*p))) / float64(2*q-2*p)
	}
	k := 0
	v[0] = 0
	z[0], z[1] = math.Inf(-1), math.Inf(1)
	for q := 1; q < n; q++ {
		s := parab(q, v[k])
		for s <= z[k] {
			k--
			s = parab(q, v[k])
		}
		k++
		v[k] = q
		z[k], z[k+1] = s, math.Inf(1)
	}
	k = 0
	for q := 0; q < n; q++ {
		for z[k+1] < float64(q) {
			k++
		}
		p := v[k]
		d[q] = float64((q-p)*(q-p)) + f[p]
	}
}

// DistanceToSet returns, for every pixel, the exact Euclidean distance (in
// px, between pixel centres) to the nearest pixel where in[i] is true; edtInf
// (as a large number) if the set is empty.
func DistanceToSet(ctx context.Context, in []bool, w, h int) ([]float32, error) {
	g := make([]float64, w*h)
	for i, b := range in {
		if b {
			g[i] = 0
		} else {
			g[i] = edtInf
		}
	}
	// columns
	err := parallel.Rows(ctx, w, func(x0, x1 int) {
		f, d := make([]float64, h), make([]float64, h)
		v, z := make([]int, h), make([]float64, h+1)
		for x := x0; x < x1; x++ {
			for y := 0; y < h; y++ {
				f[y] = g[y*w+x]
			}
			dt1d(f, d, v, z)
			for y := 0; y < h; y++ {
				g[y*w+x] = d[y]
			}
		}
	})
	if err != nil {
		return nil, err
	}
	out := make([]float32, w*h)
	err = parallel.Rows(ctx, h, func(y0, y1 int) {
		f, d := make([]float64, w), make([]float64, w)
		v, z := make([]int, w), make([]float64, w+1)
		for y := y0; y < y1; y++ {
			copy(f, g[y*w:(y+1)*w])
			dt1d(f, d, v, z)
			for x := 0; x < w; x++ {
				out[y*w+x] = float32(math.Sqrt(d[x]))
			}
		}
	})
	return out, err
}

// Grow dilates a mask by r px: pixels within r of the selection (m ≥ 0.5)
// are selected, with a one-pixel anti-aliased edge.
func Grow(ctx context.Context, m *buffer.Buffer, r float64) (*buffer.Buffer, error) {
	pl := m.Plane(0)
	set := make([]bool, len(pl))
	for i, v := range pl {
		set[i] = v >= 0.5
	}
	d, err := DistanceToSet(ctx, set, m.W, m.H)
	if err != nil {
		return nil, err
	}
	out := buffer.NewLike(m)
	po := out.Plane(0)
	for i := range po {
		po[i] = max(pl[i], clamp01f(float32(r)+1-d[i]))
	}
	return out, nil
}

// Shrink erodes a mask by r px: pixels closer than r to the unselected part
// (m < 0.5) are removed, with a one-pixel anti-aliased edge.
func Shrink(ctx context.Context, m *buffer.Buffer, r float64) (*buffer.Buffer, error) {
	pl := m.Plane(0)
	set := make([]bool, len(pl))
	for i, v := range pl {
		set[i] = v < 0.5
	}
	d, err := DistanceToSet(ctx, set, m.W, m.H)
	if err != nil {
		return nil, err
	}
	out := buffer.NewLike(m)
	po := out.Plane(0)
	for i := range po {
		po[i] = min(pl[i], clamp01f(d[i]-float32(r)))
	}
	return out, nil
}

// BoxMean is the mean of p over a (2r+1)² window clipped to the image,
// computed with an integral image.
func BoxMean(ctx context.Context, p []float32, w, h, r int) ([]float32, error) {
	ii := make([]float64, (w+1)*(h+1))
	for y := 0; y < h; y++ {
		var row float64
		for x := 0; x < w; x++ {
			row += float64(p[y*w+x])
			ii[(y+1)*(w+1)+x+1] = ii[y*(w+1)+x+1] + row
		}
	}
	out := make([]float32, w*h)
	err := parallel.Rows(ctx, h, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			ya, yb := max(0, y-r), min(h, y+r+1)
			for x := 0; x < w; x++ {
				xa, xb := max(0, x-r), min(w, x+r+1)
				s := ii[yb*(w+1)+xb] - ii[ya*(w+1)+xb] - ii[yb*(w+1)+xa] + ii[ya*(w+1)+xa]
				out[y*w+x] = float32(s / float64((yb-ya)*(xb-xa)))
			}
		}
	})
	return out, err
}

// GuidedFilter refines mask p with the gray guide I (He et al.): an
// edge-aware smoothing that snaps soft mask edges to edges of the guide. r
// is the window radius in px, eps the regularization (in guide units²).
func GuidedFilter(ctx context.Context, I, p []float32, w, h, r int, eps float64) ([]float32, error) {
	n := w * h
	mul := func(a, b []float32) []float32 {
		o := make([]float32, n)
		for i := range o {
			o[i] = a[i] * b[i]
		}
		return o
	}
	mI, err := BoxMean(ctx, I, w, h, r)
	if err != nil {
		return nil, err
	}
	mP, err := BoxMean(ctx, p, w, h, r)
	if err != nil {
		return nil, err
	}
	cII, err := BoxMean(ctx, mul(I, I), w, h, r)
	if err != nil {
		return nil, err
	}
	cIP, err := BoxMean(ctx, mul(I, p), w, h, r)
	if err != nil {
		return nil, err
	}
	a, b := make([]float32, n), make([]float32, n)
	for i := 0; i < n; i++ {
		varI := float64(cII[i]) - float64(mI[i])*float64(mI[i])
		cov := float64(cIP[i]) - float64(mI[i])*float64(mP[i])
		ai := cov / (varI + eps)
		a[i] = float32(ai)
		b[i] = float32(float64(mP[i]) - ai*float64(mI[i]))
	}
	mA, err := BoxMean(ctx, a, w, h, r)
	if err != nil {
		return nil, err
	}
	mB, err := BoxMean(ctx, b, w, h, r)
	if err != nil {
		return nil, err
	}
	q := make([]float32, n)
	for i := range q {
		q[i] = clamp01f(mA[i]*I[i] + mB[i])
	}
	return q, nil
}

// ToRGB returns b as a 3-channel buffer (b itself if it already is).
func ToRGB(b *buffer.Buffer) *buffer.Buffer {
	if b.C >= 3 {
		return b
	}
	out := buffer.NewLikeC(b, 3)
	for c := 0; c < 3; c++ {
		copy(out.Plane(c), b.Plane(0))
	}
	return out
}

// TintOverlay draws mask m over img in colour col (linear RGB) with opacity
// alpha·m, for looking at a selection. The result is RGB.
func TintOverlay(ctx context.Context, img, m *buffer.Buffer, col [3]float32, alpha float32) (*buffer.Buffer, error) {
	src := ToRGB(img)
	out := buffer.NewLike(src)
	mp := m.Plane(0)
	err := parallel.Rows(ctx, src.H, func(y0, y1 int) {
		for c := 0; c < 3; c++ {
			s, d := src.Plane(c), out.Plane(c)
			for i := y0 * src.W; i < y1*src.W; i++ {
				a := alpha * mp[i]
				d[i] = s[i]*(1-a) + col[c]*a
			}
		}
	})
	return out, err
}
