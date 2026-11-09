package kernels

import (
	"context"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/text"
)

// blendPixel mixes colour col into RGB buffer b at (x, y) with opacity a.
func blendPixel(b *buffer.Buffer, x, y int, col [3]float32, a float32) {
	if x < 0 || y < 0 || x >= b.W || y >= b.H {
		return
	}
	i := y*b.W + x
	for c := 0; c < 3; c++ {
		p := b.Plane(c)
		p[i] = p[i]*(1-a) + col[c]*a
	}
}

// DrawRect draws the outline of [x0, x1] × [y0, y1] (inclusive, buffer px)
// with line width lw.
func DrawRect(b *buffer.Buffer, x0, y0, x1, y1, lw int, col [3]float32, a float32) {
	for t := 0; t < lw; t++ {
		for x := x0 - t; x <= x1+t; x++ {
			blendPixel(b, x, y0-t, col, a)
			blendPixel(b, x, y1+t, col, a)
		}
		for y := y0 - t + 1; y < y1+t; y++ {
			blendPixel(b, x0-t, y, col, a)
			blendPixel(b, x1+t, y, col, a)
		}
	}
}

// DrawText draws s with the pkg/cv/text font, top-left at (x, y), each font
// pixel px × px, over a dark one-pixel shadow for legibility.
func DrawText(b *buffer.Buffer, x, y int, s string, px int, col [3]float32) {
	text.Walk(x+1, y+1, s, px, func(x, y int) { blendPixel(b, x, y, [3]float32{}, 0.7) })
	text.Walk(x, y, s, px, func(x, y int) { blendPixel(b, x, y, col, 1) })
}

// Paste copies src into dst with its top-left at (x, y), clipped; channel
// counts must match (use ToRGB).
func Paste(dst, src *buffer.Buffer, x, y int) {
	for c := 0; c < min(dst.C, src.C); c++ {
		dp, sp := dst.Plane(c), src.Plane(c)
		for sy := 0; sy < src.H; sy++ {
			dy := sy + y
			if dy < 0 || dy >= dst.H {
				continue
			}
			for sx := 0; sx < src.W; sx++ {
				dx := sx + x
				if dx < 0 || dx >= dst.W {
					continue
				}
				dp[dy*dst.W+dx] = sp[sy*src.W+sx]
			}
		}
	}
}

// AbsDiff is |a − b|·gain per channel, clamped to [0, 1], for looking at
// what changed.
func AbsDiff(ctx context.Context, a, b *buffer.Buffer, gain float32) (*buffer.Buffer, error) {
	a, b = matchChannels(a, b)
	out := buffer.NewLike(a)
	for c := 0; c < a.C; c++ {
		pa, pb, po := a.Plane(c), b.Plane(c), out.Plane(c)
		for i := range po {
			if i%(1<<16) == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			d := pa[i] - pb[i]
			if d < 0 {
				d = -d
			}
			po[i] = clamp01f(d * gain)
		}
	}
	return out, nil
}
