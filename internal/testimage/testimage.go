// Package testimage generates the small synthetic images under testdata/.
package testimage

import (
	"context"
	"path/filepath"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
)

// Size of the gradient test image.
const (
	W = 64
	H = 48
)

// Files written by Write, relative to the output directory.
const (
	TIFF16 = "gradient-64x48-16bit.tif"
	PNG8   = "gradient-64x48-8bit.png"
)

// Gradient returns the test image in linear light: in sRGB (perceptual)
// terms, R ramps 0→1 left to right, G ramps 0→1 top to bottom and B is 0.25.
func Gradient() *buffer.Buffer {
	b := buffer.New(W, H, 3, 1)
	r, g, bl := b.Plane(0), b.Plane(1), b.Plane(2)
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			i := y*W + x
			r[i] = buffer.ToLinear(float32(x) / (W - 1))
			g[i] = buffer.ToLinear(float32(y) / (H - 1))
			bl[i] = buffer.ToLinear(0.25)
		}
	}
	return b
}

// Write writes all test images into dir.
func Write(ctx context.Context, dir string) error {
	b := Gradient()
	if err := writeSRGB(ctx, filepath.Join(dir, TIFF16), b, "tiff16"); err != nil {
		return err
	}
	return writeSRGB(ctx, filepath.Join(dir, PNG8), b, "png8")
}

// writeSRGB writes linear-light b as an sRGB-encoded file.
func writeSRGB(ctx context.Context, path string, b *buffer.Buffer, format string) error {
	enc, err := buffer.Convert(ctx, b, buffer.EncSRGB, buffer.SpaceLinear, buffer.SpaceEncoded)
	if err != nil {
		return err
	}
	return buffer.WriteFile(ctx, path, enc, format)
}
