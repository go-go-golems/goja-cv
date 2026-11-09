package kernels

import (
	"context"
	"math"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/parallel"
)

// Colormaps are 256-entry tables of display (sRGB-encoded) colours.
type Colormap [256][3]float32

// polyMap builds a table from a degree-6 polynomial per channel (Matt
// Zucker's CC0 fits of matplotlib's viridis and magma).
func polyMap(c [7][3]float64) *Colormap {
	var m Colormap
	for i := range m {
		t := float64(i) / 255
		for ch := 0; ch < 3; ch++ {
			v := c[6][ch]
			for k := 5; k >= 0; k-- {
				v = c[k][ch] + t*v
			}
			m[i][ch] = float32(math.Max(0, math.Min(1, v)))
		}
	}
	return &m
}

// anchorMap interpolates linearly between evenly spaced anchor colours.
func anchorMap(anchors [][3]float64) *Colormap {
	var m Colormap
	n := len(anchors) - 1
	for i := range m {
		t := float64(i) / 255 * float64(n)
		k := min(int(t), n-1)
		f := t - float64(k)
		for ch := 0; ch < 3; ch++ {
			m[i][ch] = float32(anchors[k][ch] + (anchors[k+1][ch]-anchors[k][ch])*f)
		}
	}
	return &m
}

// Colormaps by name: gray, viridis, magma (sequential) and diverging
// (blue–white–red, ColorBrewer RdBu reversed).
var Colormaps = map[string]*Colormap{
	"gray": anchorMap([][3]float64{{0, 0, 0}, {1, 1, 1}}),
	"viridis": polyMap([7][3]float64{
		{0.2777273272234177, 0.005407344544966578, 0.3340998053353061},
		{0.1050930431085774, 1.404613529898575, 1.384590162594685},
		{-0.3308618287255563, 0.214847559468213, 0.09509516302823659},
		{-4.634230498983486, -5.799100973351585, -19.33244095627987},
		{6.228269936347081, 14.17993336680509, 56.69055260068105},
		{4.776384997670288, -13.74514537774601, -65.35303263337234},
		{-5.435455855934631, 4.645852612178535, 26.3124352495832},
	}),
	"magma": polyMap([7][3]float64{
		{-0.002136485053939582, -0.000749655052795221, -0.005386127855323933},
		{0.2516605407371642, 0.6775232436837668, 2.494026599312351},
		{8.353717279216625, -3.577719514958484, 0.3144679030132573},
		{-27.66873308576866, 14.26473078096533, -13.64921318813922},
		{52.17613981234068, -27.94360607168351, 12.94416944238394},
		{-50.76852536473588, 29.04658282127291, 4.23415299384598},
		{18.65570506591883, -11.48977351997711, -5.601961508734096},
	}),
	"diverging": anchorMap([][3]float64{
		{0.020, 0.188, 0.380}, {0.263, 0.576, 0.765}, {0.969, 0.969, 0.969}, {0.839, 0.376, 0.302}, {0.404, 0.000, 0.122},
	}),
}

// ColormapNames lists the colormaps in display order.
var ColormapNames = []string{"gray", "viridis", "magma", "diverging"}

// ApplyColormap maps the single plane of in through m: lo → entry 0, hi →
// entry 255, clamped. The result has three channels of display values.
func ApplyColormap(ctx context.Context, in *buffer.Buffer, m *Colormap, lo, hi float64) (*buffer.Buffer, error) {
	out := buffer.NewLikeC(in, 3)
	src := in.Plane(0)
	r, g, b := out.Plane(0), out.Plane(1), out.Plane(2)
	scale := 255 / math.Max(hi-lo, 1e-12)
	err := parallel.Rows(ctx, in.H, func(y0, y1 int) {
		for i := y0 * in.W; i < y1*in.W; i++ {
			k := int(math.Round((float64(src[i]) - lo) * scale))
			k = max(0, min(255, k))
			r[i], g[i], b[i] = m[k][0], m[k][1], m[k][2]
		}
	})
	return out, err
}

// Chroma is the per-pixel spread of the channels, max − min, divided by
// (mean + 1e-3) when relative. A 1-channel buffer has chroma 0.
func Chroma(ctx context.Context, in *buffer.Buffer, relative bool) (*buffer.Buffer, error) {
	out := buffer.NewLikeC(in, 1)
	if in.C < 3 {
		return out, nil
	}
	r, g, b, dst := in.Plane(0), in.Plane(1), in.Plane(2), out.Plane(0)
	err := parallel.Rows(ctx, in.H, func(y0, y1 int) {
		for i := y0 * in.W; i < y1*in.W; i++ {
			mx := max(r[i], g[i], b[i])
			mn := min(r[i], g[i], b[i])
			c := mx - mn
			if relative {
				c /= (r[i]+g[i]+b[i])/3 + 1e-3
			}
			dst[i] = c
		}
	})
	return out, err
}

// ToLab converts linear RGB with sRGB/Rec. 709 primaries (D65 white) to
// CIE L*a*b*, returned as L*/100, a*/100, b*/100. A 1-channel buffer is
// treated as grey.
func ToLab(ctx context.Context, in *buffer.Buffer) (*buffer.Buffer, error) {
	out := buffer.NewLikeC(in, 3)
	R, G, B := in.Plane(0), in.Plane(0), in.Plane(0)
	if in.C >= 3 {
		G, B = in.Plane(1), in.Plane(2)
	}
	L, A, Bb := out.Plane(0), out.Plane(1), out.Plane(2)
	const xn, yn, zn = 0.95047, 1.0, 1.08883
	f := func(t float64) float64 {
		const d = 6.0 / 29
		if t > d*d*d {
			return math.Cbrt(t)
		}
		return t/(3*d*d) + 4.0/29
	}
	err := parallel.Rows(ctx, in.H, func(y0, y1 int) {
		for i := y0 * in.W; i < y1*in.W; i++ {
			r, g, b := float64(R[i]), float64(G[i]), float64(B[i])
			x := 0.4124564*r + 0.3575761*g + 0.1804375*b
			y := 0.2126729*r + 0.7151522*g + 0.0721750*b
			z := 0.0193339*r + 0.1191920*g + 0.9503041*b
			fx, fy, fz := f(x/xn), f(y/yn), f(z/zn)
			L[i] = float32((116*fy - 16) / 100)
			A[i] = float32(500 * (fx - fy) / 100)
			Bb[i] = float32(200 * (fy - fz) / 100)
		}
	})
	return out, err
}
