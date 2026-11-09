package buffer

import (
	"bufio"
	"context"
	"encoding/binary"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/parallel"
	"github.com/pkg/errors"
	"golang.org/x/image/tiff"
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

// EncodePNG writes b (encoded values) as an 8- or 16-bit PNG.
func EncodePNG(ctx context.Context, w io.Writer, b *Buffer, bits int) error {
	m, err := Quantize(ctx, b, bits)
	if err != nil {
		return err
	}
	return errors.Wrap(png.Encode(w, m), "encode png")
}

// EncodeJPEG writes b (encoded values) as an 8-bit JPEG.
func EncodeJPEG(ctx context.Context, w io.Writer, b *Buffer, quality int) error {
	m, err := Quantize(ctx, b, 8)
	if err != nil {
		return err
	}
	return errors.Wrap(jpeg.Encode(w, m, &jpeg.Options{Quality: quality}), "encode jpeg")
}

// EncodeTIFF16 writes b (encoded values) as a 16-bit TIFF (Deflate).
// x/image/tiff stores RGB images with an (opaque) alpha sample.
func EncodeTIFF16(ctx context.Context, w io.Writer, b *Buffer) error {
	m, err := Quantize(ctx, b, 16)
	if err != nil {
		return err
	}
	return errors.Wrap(tiff.Encode(w, m, &tiff.Options{Compression: tiff.Deflate}), "encode tiff")
}

// WriteFile writes b, which holds encoded values (convert linear values with
// Convert first), by format: "png8", "png16", "jpeg", "tiff16". An empty
// format is inferred from the extension (.png → png16, .jpg/.jpeg → jpeg,
// .tif/.tiff → tiff16).
func WriteFile(ctx context.Context, path string, b *Buffer, format string) error {
	if format == "" {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".png":
			format = "png16"
		case ".jpg", ".jpeg":
			format = "jpeg"
		case ".tif", ".tiff":
			format = "tiff16"
		default:
			return errors.Errorf("cannot infer image format from %q", path)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return errors.Wrap(err, "create image file")
	}
	bw := bufio.NewWriter(f)
	switch format {
	case "png8":
		err = EncodePNG(ctx, bw, b, 8)
	case "png16":
		err = EncodePNG(ctx, bw, b, 16)
	case "jpeg":
		err = EncodeJPEG(ctx, bw, b, 92)
	case "tiff16":
		err = EncodeTIFF16(ctx, bw, b)
	default:
		err = errors.Errorf("unknown image format %q", format)
	}
	if err == nil {
		err = bw.Flush()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}
