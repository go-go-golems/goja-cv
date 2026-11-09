package buffer

import (
	"context"
	"encoding/binary"
	"image"
	"image/jpeg"
	"io"
	"math"

	"github.com/go-go-golems/goja-cv/pkg/cv/parallel"
	"github.com/pkg/errors"
)

// Quantize converts a buffer of encoded values to a Go image with 8 or 16
// bits per sample (Gray/Gray16 for C=1, opaque NRGBA/NRGBA64 for C=3): values
// are clamped to [0, 1] and scaled to the largest code. No transfer curve is
// applied: the graph converts to encoded values first (looks, files).
func Quantize(ctx context.Context, b *Buffer, bits int) (image.Image, error) {
	if bits != 8 && bits != 16 {
		return nil, errors.Errorf("unsupported bit depth %d", bits)
	}
	if b.C != 1 && b.C != 3 {
		return nil, errors.Errorf("unsupported channel count %d", b.C)
	}
	w, h := b.W, b.H
	rect := image.Rect(0, 0, w, h)
	q8 := func(v float32) uint8 { return uint8(math.Round(clamp01(float64(v)) * 255)) }
	q16 := func(v float32) uint16 { return uint16(math.Round(clamp01(float64(v)) * 65535)) }
	switch {
	case b.C == 1 && bits == 8:
		m := image.NewGray(rect)
		p := b.Plane(0)
		err := parallel.Rows(ctx, h, func(y0, y1 int) {
			for i := y0 * w; i < y1*w; i++ {
				m.Pix[i] = q8(p[i])
			}
		})
		return m, err
	case b.C == 1:
		m := image.NewGray16(rect)
		p := b.Plane(0)
		err := parallel.Rows(ctx, h, func(y0, y1 int) {
			for i := y0 * w; i < y1*w; i++ {
				binary.BigEndian.PutUint16(m.Pix[2*i:], q16(p[i]))
			}
		})
		return m, err
	case bits == 8:
		m := image.NewNRGBA(rect)
		rp, gp, bp := b.Plane(0), b.Plane(1), b.Plane(2)
		err := parallel.Rows(ctx, h, func(y0, y1 int) {
			for i := y0 * w; i < y1*w; i++ {
				s := m.Pix[4*i : 4*i+4]
				s[0], s[1], s[2], s[3] = q8(rp[i]), q8(gp[i]), q8(bp[i]), 255
			}
		})
		return m, err
	default:
		m := image.NewNRGBA64(rect)
		rp, gp, bp := b.Plane(0), b.Plane(1), b.Plane(2)
		err := parallel.Rows(ctx, h, func(y0, y1 int) {
			for i := y0 * w; i < y1*w; i++ {
				s := m.Pix[8*i : 8*i+8]
				binary.BigEndian.PutUint16(s[0:], q16(rp[i]))
				binary.BigEndian.PutUint16(s[2:], q16(gp[i]))
				binary.BigEndian.PutUint16(s[4:], q16(bp[i]))
				s[6], s[7] = 255, 255
			}
		})
		return m, err
	}
}

// EncodeJPEG writes b (encoded values) as an 8-bit JPEG.
func EncodeJPEG(ctx context.Context, w io.Writer, b *Buffer, quality int) error {
	m, err := Quantize(ctx, b, 8)
	if err != nil {
		return err
	}
	return errors.Wrap(jpeg.Encode(w, m, &jpeg.Options{Quality: quality}), "encode jpeg")
}
