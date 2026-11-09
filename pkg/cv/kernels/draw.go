package kernels

import (
	"context"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
)

// digits is a 3×5 pixel font for 0–9 (each row is 3 bits, MSB left).
var digits = [10][5]uint8{
	{7, 5, 5, 5, 7}, {2, 6, 2, 2, 7}, {7, 1, 7, 4, 7}, {7, 1, 7, 1, 7}, {5, 5, 7, 1, 1},
	{7, 4, 7, 1, 7}, {7, 4, 7, 5, 7}, {7, 1, 1, 1, 1}, {7, 5, 7, 5, 7}, {7, 5, 7, 1, 7},
}

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

// TextWidth is the width in px of a digit string at scale k.
func TextWidth(s string, k int) int { return len(s) * 4 * k }

// DrawDigits draws a string of digits with the 3×5 font, top-left at (x, y),
// each font pixel k×k, over a dark one-pixel shadow for legibility. Other
// characters are skipped.
func DrawDigits(b *buffer.Buffer, x, y int, s string, k int, col [3]float32) {
	shadow := [3]float32{0, 0, 0}
	for pass := 0; pass < 2; pass++ {
		off, c, a := 1, shadow, float32(0.7)
		if pass == 1 {
			off, c, a = 0, col, 1
		}
		cx := x
		for _, r := range s {
			if r < '0' || r > '9' {
				cx += 4 * k
				continue
			}
			g := digits[r-'0']
			for row := 0; row < 5; row++ {
				for colb := 0; colb < 3; colb++ {
					if g[row]&(4>>colb) == 0 {
						continue
					}
					for dy := 0; dy < k; dy++ {
						for dx := 0; dx < k; dx++ {
							blendPixel(b, cx+colb*k+dx+off, y+row*k+dy+off, c, a)
						}
					}
				}
			}
			cx += 4 * k
		}
	}
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
