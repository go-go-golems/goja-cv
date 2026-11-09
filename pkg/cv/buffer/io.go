package buffer

import (
	"bufio"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/parallel"
	"github.com/pkg/errors"
	"golang.org/x/image/tiff"
)

// Info describes a decoded image file.
type Info struct {
	Format   string `json:"format"` // "png", "jpeg", "tiff"
	Bits     int    `json:"bits"`   // 8 or 16 bits per sample in the file
	W        int    `json:"w"`
	H        int    `json:"h"`
	Channels int    `json:"channels"` // 1 or 3 in the decoded buffer
}

// DecodeConfigFile reads only the header of an image file.
func DecodeConfigFile(path string) (Info, error) {
	f, err := os.Open(path)
	if err != nil {
		return Info{}, errors.Wrap(err, "open image")
	}
	defer func() { _ = f.Close() }() // read-only: a close error cannot lose data
	cfg, format, err := image.DecodeConfig(bufio.NewReader(f))
	if err != nil {
		return Info{}, errors.Wrapf(err, "decode header of %s", path)
	}
	info := Info{Format: format, W: cfg.Width, H: cfg.Height, Bits: 8, Channels: 3}
	switch cfg.ColorModel {
	case color.Gray16Model:
		info.Bits, info.Channels = 16, 1
	case color.GrayModel:
		info.Channels = 1
	case color.RGBA64Model, color.NRGBA64Model:
		info.Bits = 16
	}
	return info, nil
}

// DecodeFile decodes a PNG, JPEG or TIFF file into a linear-light buffer at scale 1.
func DecodeFile(ctx context.Context, path string) (*Buffer, Info, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, Info{}, errors.Wrap(err, "open image")
	}
	defer func() { _ = f.Close() }() // read-only: a close error cannot lose data
	b, info, err := Decode(ctx, bufio.NewReader(f))
	if err != nil {
		return nil, Info{}, errors.Wrapf(err, "decode %s", path)
	}
	return b, info, nil
}

// Decode decodes an image stream. 8- and 16-bit gray and RGB images keep their
// precision through the sRGB lookup tables; alpha is ignored (images are
// assumed opaque). EXIF orientation is not applied.
func Decode(ctx context.Context, r io.Reader) (*Buffer, Info, error) {
	img, format, err := image.Decode(r)
	if err != nil {
		return nil, Info{}, err
	}
	rect := img.Bounds()
	w, h := rect.Dx(), rect.Dy()
	info := Info{Format: format, W: w, H: h, Bits: 8, Channels: 3}

	var buf *Buffer
	switch m := img.(type) {
	case *image.Gray:
		info.Channels = 1
		buf = New(w, h, 1, 1)
		p := buf.Plane(0)
		err = parallel.Rows(ctx, h, func(y0, y1 int) {
			for y := y0; y < y1; y++ {
				row := m.Pix[y*m.Stride : y*m.Stride+w]
				for x, v := range row {
					p[y*w+x] = Linear8(v)
				}
			}
		})
	case *image.Gray16:
		info.Channels, info.Bits = 1, 16
		buf = New(w, h, 1, 1)
		p := buf.Plane(0)
		err = parallel.Rows(ctx, h, func(y0, y1 int) {
			for y := y0; y < y1; y++ {
				off := y * m.Stride
				for x := 0; x < w; x++ {
					p[y*w+x] = Linear16(uint16(m.Pix[off+2*x])<<8 | uint16(m.Pix[off+2*x+1]))
				}
			}
		})
	case *image.RGBA:
		buf, err = decodeInterleaved8(ctx, m.Pix, m.Stride, w, h)
	case *image.NRGBA:
		buf, err = decodeInterleaved8(ctx, m.Pix, m.Stride, w, h)
	case *image.RGBA64:
		info.Bits = 16
		buf, err = decodeInterleaved16(ctx, m.Pix, m.Stride, w, h)
	case *image.NRGBA64:
		info.Bits = 16
		buf, err = decodeInterleaved16(ctx, m.Pix, m.Stride, w, h)
	default:
		// YCbCr (JPEG), paletted, CMYK, ...: go through the generic color model.
		buf = New(w, h, 3, 1)
		rp, gp, bp := buf.Plane(0), buf.Plane(1), buf.Plane(2)
		err = parallel.Rows(ctx, h, func(y0, y1 int) {
			for y := y0; y < y1; y++ {
				for x := 0; x < w; x++ {
					c := color.NRGBA64Model.Convert(img.At(rect.Min.X+x, rect.Min.Y+y)).(color.NRGBA64)
					i := y*w + x
					rp[i], gp[i], bp[i] = Linear16(c.R), Linear16(c.G), Linear16(c.B)
				}
			}
		})
	}
	if err != nil {
		return nil, Info{}, err
	}
	return buf, info, nil
}

func decodeInterleaved8(ctx context.Context, pix []uint8, stride, w, h int) (*Buffer, error) {
	buf := New(w, h, 3, 1)
	rp, gp, bp := buf.Plane(0), buf.Plane(1), buf.Plane(2)
	err := parallel.Rows(ctx, h, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			off := y * stride
			for x := 0; x < w; x++ {
				s := pix[off+4*x:]
				i := y*w + x
				rp[i], gp[i], bp[i] = Linear8(s[0]), Linear8(s[1]), Linear8(s[2])
			}
		}
	})
	return buf, err
}

func decodeInterleaved16(ctx context.Context, pix []uint8, stride, w, h int) (*Buffer, error) {
	buf := New(w, h, 3, 1)
	rp, gp, bp := buf.Plane(0), buf.Plane(1), buf.Plane(2)
	err := parallel.Rows(ctx, h, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			off := y * stride
			for x := 0; x < w; x++ {
				s := pix[off+8*x:]
				i := y*w + x
				rp[i] = Linear16(uint16(s[0])<<8 | uint16(s[1]))
				gp[i] = Linear16(uint16(s[2])<<8 | uint16(s[3]))
				bp[i] = Linear16(uint16(s[4])<<8 | uint16(s[5]))
			}
		}
	})
	return buf, err
}

// ToImage converts a buffer to an sRGB-encoded Go image with 8 or 16 bits per
// sample (Gray/Gray16 for C=1, opaque NRGBA/NRGBA64 for C=3). Values are
// clamped to [0, 1].
func ToImage(ctx context.Context, b *Buffer, bits int) (image.Image, error) {
	if bits != 8 && bits != 16 {
		return nil, errors.Errorf("unsupported bit depth %d", bits)
	}
	if b.C != 1 && b.C != 3 {
		return nil, errors.Errorf("unsupported channel count %d", b.C)
	}
	w, h := b.W, b.H
	rect := image.Rect(0, 0, w, h)
	switch {
	case b.C == 1 && bits == 8:
		m := image.NewGray(rect)
		p := b.Plane(0)
		err := parallel.Rows(ctx, h, func(y0, y1 int) {
			for i := y0 * w; i < y1*w; i++ {
				m.Pix[i] = EncodeSRGB8(p[i])
			}
		})
		return m, err
	case b.C == 1:
		m := image.NewGray16(rect)
		p := b.Plane(0)
		err := parallel.Rows(ctx, h, func(y0, y1 int) {
			for i := y0 * w; i < y1*w; i++ {
				binary.BigEndian.PutUint16(m.Pix[2*i:], EncodeSRGB16(p[i]))
			}
		})
		return m, err
	case bits == 8:
		m := image.NewNRGBA(rect)
		rp, gp, bp := b.Plane(0), b.Plane(1), b.Plane(2)
		err := parallel.Rows(ctx, h, func(y0, y1 int) {
			for i := y0 * w; i < y1*w; i++ {
				s := m.Pix[4*i : 4*i+4]
				s[0], s[1], s[2], s[3] = EncodeSRGB8(rp[i]), EncodeSRGB8(gp[i]), EncodeSRGB8(bp[i]), 255
			}
		})
		return m, err
	default:
		m := image.NewNRGBA64(rect)
		rp, gp, bp := b.Plane(0), b.Plane(1), b.Plane(2)
		err := parallel.Rows(ctx, h, func(y0, y1 int) {
			for i := y0 * w; i < y1*w; i++ {
				s := m.Pix[8*i : 8*i+8]
				binary.BigEndian.PutUint16(s[0:], EncodeSRGB16(rp[i]))
				binary.BigEndian.PutUint16(s[2:], EncodeSRGB16(gp[i]))
				binary.BigEndian.PutUint16(s[4:], EncodeSRGB16(bp[i]))
				s[6], s[7] = 255, 255
			}
		})
		return m, err
	}
}

// EncodePNG writes b as an 8- or 16-bit sRGB PNG.
func EncodePNG(ctx context.Context, w io.Writer, b *Buffer, bits int) error {
	m, err := ToImage(ctx, b, bits)
	if err != nil {
		return err
	}
	return errors.Wrap(png.Encode(w, m), "encode png")
}

// EncodeJPEG writes b as an 8-bit sRGB JPEG.
func EncodeJPEG(ctx context.Context, w io.Writer, b *Buffer, quality int) error {
	m, err := ToImage(ctx, b, 8)
	if err != nil {
		return err
	}
	return errors.Wrap(jpeg.Encode(w, m, &jpeg.Options{Quality: quality}), "encode jpeg")
}

// EncodeTIFF16 writes b as a 16-bit sRGB TIFF (Deflate). x/image/tiff stores
// RGB images with an (opaque) alpha sample.
func EncodeTIFF16(ctx context.Context, w io.Writer, b *Buffer) error {
	m, err := ToImage(ctx, b, 16)
	if err != nil {
		return err
	}
	return errors.Wrap(tiff.Encode(w, m, &tiff.Options{Compression: tiff.Deflate}), "encode tiff")
}

// WriteFile encodes b by format: "png8", "png16", "jpeg", "tiff16". An empty
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
