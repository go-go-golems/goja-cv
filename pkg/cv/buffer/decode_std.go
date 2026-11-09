package buffer

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/go-go-golems/goja-cv/pkg/cv/parallel"
	"github.com/pkg/errors"
)

// pngDecoder reads PNG through image/png; a chunk scan reads what the stdlib
// ignores: IHDR depth, gAMA, sRGB and goja-cv's tEXt metadata.
type pngDecoder struct{}

var pngMagic = []byte("\x89PNG\r\n\x1a\n")

func (pngDecoder) Name() string                          { return "png" }
func (pngDecoder) Sniff(head []byte) bool                { return bytes.HasPrefix(head, pngMagic) }
func (d pngDecoder) Probe(r io.ReadSeeker) (Info, error) { return pngChunks(r) }

// pngChunks scans the chunk list: IHDR (size, depth, colour type), gAMA
// (100000 = gamma 1.0 = linear), sRGB, and tEXt with keyword goja-cv.
func pngChunks(r io.ReadSeeker) (Info, error) {
	if _, err := r.Seek(int64(len(pngMagic)), io.SeekStart); err != nil {
		return Info{}, errors.Wrap(err, "seek")
	}
	info := Info{Format: "png", Sample: "uint", Encoding: EncSRGB}
	var hdr [8]byte
	sawIHDR, sawSRGB, linearGamma := false, false, false
	var meta string
	for i := 0; i < 1<<16; i++ {
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			if sawIHDR {
				break // tolerate a missing IEND
			}
			return Info{}, formatErr("truncated PNG header")
		}
		n := binary.BigEndian.Uint32(hdr[:4])
		typ := string(hdr[4:8])
		if n > 1<<30 {
			return Info{}, formatErr("PNG chunk %q of %d bytes", typ, n)
		}
		switch typ {
		case "IHDR", "gAMA", "tEXt":
			if n > 1<<20 {
				return Info{}, formatErr("PNG %s chunk of %d bytes", typ, n)
			}
			data := make([]byte, n)
			if _, err := io.ReadFull(r, data); err != nil {
				return Info{}, formatErr("truncated PNG %s chunk", typ)
			}
			if _, err := r.Seek(4, io.SeekCurrent); err != nil { // CRC
				return Info{}, errors.Wrap(err, "seek")
			}
			switch typ {
			case "IHDR":
				if len(data) < 13 {
					return Info{}, formatErr("short PNG IHDR")
				}
				info.W, info.H = int(binary.BigEndian.Uint32(data[0:4])), int(binary.BigEndian.Uint32(data[4:8]))
				depth, ct := int(data[8]), data[9]
				info.Bits = 8
				if depth == 16 {
					info.Bits = 16
				}
				info.Channels = 3
				if ct == 0 || ct == 4 {
					info.Channels = 1
				}
				sawIHDR = true
			case "gAMA":
				if len(data) == 4 && binary.BigEndian.Uint32(data) == 100000 {
					linearGamma = true
				}
			case "tEXt":
				if k, v, ok := bytes.Cut(data, []byte{0}); ok && string(k) == MetadataKey {
					meta = string(v)
				}
			}
		case "sRGB":
			sawSRGB = true
			if _, err := r.Seek(int64(n)+4, io.SeekCurrent); err != nil {
				return Info{}, errors.Wrap(err, "seek")
			}
		case "IEND":
			i = 1 << 16
		default:
			if _, err := r.Seek(int64(n)+4, io.SeekCurrent); err != nil {
				return Info{}, errors.Wrap(err, "seek")
			}
		}
	}
	if !sawIHDR {
		return Info{}, formatErr("PNG without IHDR")
	}
	if linearGamma && !sawSRGB {
		info.Encoding = EncLinear
	}
	applyMetadata(&info, meta)
	return info, nil
}

func (d pngDecoder) Decode(ctx context.Context, r io.ReadSeeker, o DecodeOptions) (*Buffer, Info, error) {
	info, err := pngChunks(r)
	if err != nil {
		return nil, Info{}, err
	}
	br, err := bufReader(r)
	if err != nil {
		return nil, Info{}, err
	}
	m, err := png.Decode(br)
	if err != nil {
		return nil, Info{}, formatErr("decode PNG: %v", err)
	}
	b, err := fromImage(ctx, m, o)
	return b, info, err
}

// jpegDecoder reads baseline and progressive JPEG through image/jpeg. EXIF
// orientation is not applied.
type jpegDecoder struct{}

func (jpegDecoder) Name() string           { return "jpeg" }
func (jpegDecoder) Sniff(head []byte) bool { return bytes.HasPrefix(head, []byte{0xff, 0xd8, 0xff}) }

func (jpegDecoder) Probe(r io.ReadSeeker) (Info, error) {
	br, err := bufReader(r)
	if err != nil {
		return Info{}, err
	}
	cfg, err := jpeg.DecodeConfig(br)
	if err != nil {
		return Info{}, formatErr("JPEG header: %v", err)
	}
	info := Info{Format: "jpeg", W: cfg.Width, H: cfg.Height, Bits: 8, Sample: "uint", Channels: 3, Encoding: EncSRGB}
	if cfg.ColorModel == color.GrayModel {
		info.Channels = 1
	}
	return info, nil
}

func (d jpegDecoder) Decode(ctx context.Context, r io.ReadSeeker, o DecodeOptions) (*Buffer, Info, error) {
	info, err := d.Probe(r)
	if err != nil {
		return nil, Info{}, err
	}
	br, err := bufReader(r)
	if err != nil {
		return nil, Info{}, err
	}
	m, err := jpeg.Decode(br)
	if err != nil {
		return nil, Info{}, formatErr("decode JPEG: %v", err)
	}
	b, err := fromImage(ctx, m, o)
	return b, info, err
}

// fromImage converts a decoded Go image (or the window o.Box of it) to
// planar encoded values: codes divided by 255 or 65535. Alpha is ignored
// (images are assumed opaque).
func fromImage(ctx context.Context, img image.Image, o DecodeOptions) (*Buffer, error) {
	rect := img.Bounds()
	win, err := window(o, rect.Dx(), rect.Dy())
	if err != nil {
		return nil, err
	}
	win = win.Add(rect.Min)
	w, h := win.Dx(), win.Dy()
	switch m := img.(type) {
	case *image.Gray:
		buf := New(w, h, 1, 1)
		p := buf.Plane(0)
		err := parallel.Rows(ctx, h, func(y0, y1 int) {
			for y := y0; y < y1; y++ {
				off := m.PixOffset(win.Min.X, win.Min.Y+y)
				for x := 0; x < w; x++ {
					p[y*w+x] = float32(m.Pix[off+x]) / 255
				}
			}
		})
		return buf, err
	case *image.Gray16:
		buf := New(w, h, 1, 1)
		p := buf.Plane(0)
		err := parallel.Rows(ctx, h, func(y0, y1 int) {
			for y := y0; y < y1; y++ {
				off := m.PixOffset(win.Min.X, win.Min.Y+y)
				for x := 0; x < w; x++ {
					p[y*w+x] = float32(uint16(m.Pix[off+2*x])<<8|uint16(m.Pix[off+2*x+1])) / 65535
				}
			}
		})
		return buf, err
	case *image.RGBA:
		return interleaved8(ctx, m.Pix, m.PixOffset(win.Min.X, win.Min.Y), m.Stride, w, h)
	case *image.NRGBA:
		return interleaved8(ctx, m.Pix, m.PixOffset(win.Min.X, win.Min.Y), m.Stride, w, h)
	case *image.RGBA64:
		return interleaved16(ctx, m.Pix, m.PixOffset(win.Min.X, win.Min.Y), m.Stride, w, h)
	case *image.NRGBA64:
		return interleaved16(ctx, m.Pix, m.PixOffset(win.Min.X, win.Min.Y), m.Stride, w, h)
	}
	// YCbCr (JPEG), paletted, CMYK, …: through the generic colour model
	buf := New(w, h, 3, 1)
	rp, gp, bp := buf.Plane(0), buf.Plane(1), buf.Plane(2)
	err = parallel.Rows(ctx, h, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < w; x++ {
				c := color.NRGBA64Model.Convert(img.At(win.Min.X+x, win.Min.Y+y)).(color.NRGBA64)
				i := y*w + x
				rp[i], gp[i], bp[i] = float32(c.R)/65535, float32(c.G)/65535, float32(c.B)/65535
			}
		}
	})
	return buf, err
}

func interleaved8(ctx context.Context, pix []uint8, start, stride, w, h int) (*Buffer, error) {
	buf := New(w, h, 3, 1)
	rp, gp, bp := buf.Plane(0), buf.Plane(1), buf.Plane(2)
	err := parallel.Rows(ctx, h, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			off := start + y*stride
			for x := 0; x < w; x++ {
				s := pix[off+4*x:]
				i := y*w + x
				rp[i], gp[i], bp[i] = float32(s[0])/255, float32(s[1])/255, float32(s[2])/255
			}
		}
	})
	return buf, err
}

func interleaved16(ctx context.Context, pix []uint8, start, stride, w, h int) (*Buffer, error) {
	buf := New(w, h, 3, 1)
	rp, gp, bp := buf.Plane(0), buf.Plane(1), buf.Plane(2)
	err := parallel.Rows(ctx, h, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			off := start + y*stride
			for x := 0; x < w; x++ {
				s := pix[off+8*x:]
				i := y*w + x
				rp[i] = float32(uint16(s[0])<<8|uint16(s[1])) / 65535
				gp[i] = float32(uint16(s[2])<<8|uint16(s[3])) / 65535
				bp[i] = float32(uint16(s[4])<<8|uint16(s[5])) / 65535
			}
		}
	})
	return buf, err
}
