package buffer

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image/png"
	"io"
	"math"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// Writers of saved images (GCV-003 §7.10). They store encoded values (the
// graph converts to the image's encoding first) and record what the values
// mean, where the format can hold it: TIFF ImageDescription and PNG tEXt
// carry FormatMetadata ("goja-cv encoding=linear origin=1536,1536"), PNG
// also gAMA 1.0 (linear) or sRGB; NPY and JPEG hold values only. GCV-001's
// decoders read the metadata back.

// Meta is what a saved file records about its values.
type Meta struct {
	Encoding Encoding // EncNone: nothing recorded
	Origin   *[2]int  // frame origin in doc px (nil: (0, 0), not recorded)
}

func (m Meta) text() string {
	if m.Encoding == EncNone {
		return ""
	}
	return FormatMetadata(m.Encoding, m.Origin)
}

// Format is a file format the writers produce.
type Format string

const (
	FormatTIFF Format = "tiff"
	FormatPNG  Format = "png"
	FormatNPY  Format = "npy"
	FormatJPEG Format = "jpeg"
)

// FormatOf is the format of a file name by its extension.
func FormatOf(path string) (Format, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".tif", ".tiff":
		return FormatTIFF, true
	case ".png":
		return FormatPNG, true
	case ".npy":
		return FormatNPY, true
	case ".jpg", ".jpeg":
		return FormatJPEG, true
	}
	return "", false
}

// Depths are the sample depths each format writes, the default first.
var Depths = map[Format][]int{FormatTIFF: {16, 8, 32}, FormatPNG: {16, 8}, FormatNPY: {32, 16}, FormatJPEG: {8}}

// ClippedFraction is the fraction of samples outside [0, 1] (what an
// integer format clamps).
func ClippedFraction(b *Buffer) float64 {
	if len(b.Pix) == 0 {
		return 0
	}
	n := 0
	for _, v := range b.Pix {
		if v < 0 || v > 1 || v != v {
			n++
		}
	}
	return float64(n) / float64(len(b.Pix))
}

func code(v float32, top float64) float64 {
	return math.Round(clamp01(float64(v)) * top)
}

// EncodeTIFF writes b (1 or 3 channels, encoded values) as a little-endian
// TIFF with Deflate strips: depth 8 or 16 (unsigned codes, values clamped
// to [0, 1]) or 32 (IEEE float, values as they are), no alpha, and meta in
// ImageDescription.
func EncodeTIFF(ctx context.Context, w io.Writer, b *Buffer, depth int, meta Meta) error {
	if b.C != 1 && b.C != 3 {
		return errors.Errorf("tiff: %d channels (1 or 3)", b.C)
	}
	if depth != 8 && depth != 16 && depth != 32 {
		return errors.Errorf("tiff: depth %d (8, 16 or 32)", depth)
	}
	bps := depth / 8
	rowBytes := b.W * b.C * bps
	rowsPerStrip := max(1, min(b.H, (64<<10)/max(1, rowBytes)))
	n := b.N()
	var body bytes.Buffer
	body.Write([]byte{'I', 'I', 42, 0, 0, 0, 0, 0}) // IFD offset patched below
	var offsets, counts []uint32
	row := make([]byte, rowBytes)
	for y0 := 0; y0 < b.H; y0 += rowsPerStrip {
		if err := ctx.Err(); err != nil {
			return err
		}
		var strip bytes.Buffer
		zw := zlib.NewWriter(&strip)
		for y := y0; y < min(b.H, y0+rowsPerStrip); y++ {
			for x := 0; x < b.W; x++ {
				for c := 0; c < b.C; c++ {
					v := b.Pix[c*n+y*b.W+x]
					o := ((x * b.C) + c) * bps
					switch depth {
					case 8:
						row[o] = uint8(code(v, 255))
					case 16:
						binary.LittleEndian.PutUint16(row[o:], uint16(code(v, 65535)))
					default:
						binary.LittleEndian.PutUint32(row[o:], math.Float32bits(v))
					}
				}
			}
			if _, err := zw.Write(row); err != nil {
				return errors.Wrap(err, "tiff: deflate")
			}
		}
		if err := zw.Close(); err != nil {
			return errors.Wrap(err, "tiff: deflate")
		}
		offsets = append(offsets, uint32(body.Len())) // #nosec G115 -- files below 4 GiB (classic TIFF)
		counts = append(counts, uint32(strip.Len()))  // #nosec G115 -- one strip is below 4 GiB
		body.Write(strip.Bytes())
		if body.Len()%2 == 1 {
			body.WriteByte(0)
		}
	}
	type tag struct {
		id, typ uint16
		vals    []uint32
		ascii   string
	}
	short, long := uint16(3), uint16(4)
	sampleFormat := uint32(1)
	if depth == 32 {
		sampleFormat = 3
	}
	photometric := uint32(1)
	if b.C == 3 {
		photometric = 2
	}
	rep := func(v uint32) []uint32 {
		out := make([]uint32, b.C)
		for i := range out {
			out[i] = v
		}
		return out
	}
	tags := []tag{
		{id: 256, typ: long, vals: []uint32{uint32(b.W)}},          // #nosec G115 -- image sizes are small
		{id: 257, typ: long, vals: []uint32{uint32(b.H)}},          // #nosec G115
		{id: 258, typ: short, vals: rep(uint32(depth))},            // #nosec G115
		{id: 259, typ: short, vals: []uint32{8}},                   // Deflate
		{id: 262, typ: short, vals: []uint32{photometric}},         //
		{id: 273, typ: long, vals: offsets},                        //
		{id: 277, typ: short, vals: []uint32{uint32(b.C)}},         // #nosec G115
		{id: 278, typ: long, vals: []uint32{uint32(rowsPerStrip)}}, // #nosec G115
		{id: 279, typ: long, vals: counts},                         //
		{id: 284, typ: short, vals: []uint32{1}},                   // chunky
		{id: 339, typ: short, vals: rep(sampleFormat)},             //
	}
	if s := meta.text(); s != "" {
		tags = append(tags, tag{})
		copy(tags[6:], tags[5:])
		tags[5] = tag{id: 270, typ: 2, ascii: s} // ImageDescription, in tag order after 262
	}
	// out-of-line values first, then the IFD
	ext := map[int]uint32{}
	for i, t := range tags {
		var data []byte
		switch {
		case t.typ == 2:
			data = append([]byte(t.ascii), 0)
		case t.typ == short && len(t.vals) > 2:
			for _, v := range t.vals {
				data = binary.LittleEndian.AppendUint16(data, uint16(v)) // #nosec G115 -- SHORT values
			}
		case t.typ == long && len(t.vals) > 1:
			for _, v := range t.vals {
				data = binary.LittleEndian.AppendUint32(data, v)
			}
		}
		if len(data) > 4 {
			ext[i] = uint32(body.Len()) // #nosec G115 -- classic TIFF
			body.Write(data)
			if body.Len()%2 == 1 {
				body.WriteByte(0)
			}
		}
	}
	ifd := uint32(body.Len()) // #nosec G115 -- classic TIFF
	var e [12]byte
	body.Write(binary.LittleEndian.AppendUint16(nil, uint16(len(tags)))) // #nosec G115 -- a dozen tags
	for i, t := range tags {
		count := uint32(len(t.vals)) // #nosec G115
		if t.typ == 2 {
			count = uint32(len(t.ascii) + 1) // #nosec G115
		}
		binary.LittleEndian.PutUint16(e[0:], t.id)
		binary.LittleEndian.PutUint16(e[2:], t.typ)
		binary.LittleEndian.PutUint32(e[4:], count)
		binary.LittleEndian.PutUint32(e[8:], 0)
		if off, ok := ext[i]; ok {
			binary.LittleEndian.PutUint32(e[8:], off)
		} else if t.typ == 2 {
			copy(e[8:], append([]byte(t.ascii), 0))
		} else if t.typ == short {
			for k, v := range t.vals {
				binary.LittleEndian.PutUint16(e[8+2*k:], uint16(v)) // #nosec G115
			}
		} else {
			binary.LittleEndian.PutUint32(e[8:], t.vals[0])
		}
		body.Write(e[:])
	}
	body.Write([]byte{0, 0, 0, 0})
	out := body.Bytes()
	binary.LittleEndian.PutUint32(out[4:], ifd)
	_, err := w.Write(out)
	return errors.Wrap(err, "tiff: write")
}

// EncodePNG writes b (encoded values) as an 8- or 16-bit PNG and records
// meta after IHDR: gAMA 1.0 for linear values or an sRGB chunk for sRGB
// ones, and a tEXt chunk with keyword goja-cv.
func EncodePNG(ctx context.Context, w io.Writer, b *Buffer, bits int, meta Meta) error {
	m, err := Quantize(ctx, b, bits)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, m); err != nil {
		return errors.Wrap(err, "encode png")
	}
	var chunks [][2]string
	switch meta.Encoding {
	case EncLinear:
		chunks = append(chunks, [2]string{"gAMA", string(binary.BigEndian.AppendUint32(nil, 100000))})
	case EncSRGB:
		chunks = append(chunks, [2]string{"sRGB", "\x00"})
	case EncNone:
	}
	if s := meta.text(); s != "" {
		chunks = append(chunks, [2]string{"tEXt", MetadataKey + "\x00" + s})
	}
	_, err = w.Write(insertPNGChunks(buf.Bytes(), chunks))
	return errors.Wrap(err, "write png")
}

// insertPNGChunks inserts chunks (type, data) right after IHDR.
func insertPNGChunks(p []byte, chunks [][2]string) []byte {
	const ihdrEnd = 8 + 8 + 13 + 4
	if len(chunks) == 0 || len(p) < ihdrEnd {
		return p
	}
	out := append([]byte(nil), p[:ihdrEnd]...)
	for _, c := range chunks {
		out = binary.BigEndian.AppendUint32(out, uint32(len(c[1]))) // #nosec G115 -- short chunks
		body := []byte(c[0] + c[1])
		out = append(out, body...)
		out = binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(body))
	}
	return append(out, p[ihdrEnd:]...)
}

// EncodeNPY writes b as a NumPy array of shape (H, W) or (H, W, 3): depth
// 32 is "<f4" (values as they are), 16 is "<u2" (codes of [0, 1], clamped).
func EncodeNPY(ctx context.Context, w io.Writer, b *Buffer, depth int) error {
	dtype := "<f4"
	switch depth {
	case 32:
	case 16:
		dtype = "<u2"
	default:
		return errors.Errorf("npy: depth %d (32 or 16)", depth)
	}
	shape := fmt.Sprintf("(%d, %d)", b.H, b.W)
	if b.C > 1 {
		shape = fmt.Sprintf("(%d, %d, %d)", b.H, b.W, b.C)
	}
	header := fmt.Sprintf("{'descr': '%s', 'fortran_order': False, 'shape': %s, }", dtype, shape)
	header += strings.Repeat(" ", (64-(10+len(header)+1)%64)%64) + "\n"
	var out bytes.Buffer
	out.WriteString("\x93NUMPY\x01\x00")
	out.Write(binary.LittleEndian.AppendUint16(nil, uint16(len(header)))) // #nosec G115 -- a short header
	out.WriteString(header)
	n := b.N()
	for i := 0; i < n; i++ {
		if i%(1<<16) == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		for c := 0; c < b.C; c++ {
			v := b.Pix[c*n+i]
			if depth == 32 {
				out.Write(binary.LittleEndian.AppendUint32(nil, math.Float32bits(v)))
			} else {
				out.Write(binary.LittleEndian.AppendUint16(nil, uint16(code(v, 65535))))
			}
		}
	}
	_, err := w.Write(out.Bytes())
	return errors.Wrap(err, "npy: write")
}

// Encode writes b in format f at depth (0 = the format's default) with
// meta where the format holds it.
func Encode(ctx context.Context, w io.Writer, b *Buffer, f Format, depth int, meta Meta) error {
	if depth == 0 {
		depth = Depths[f][0]
	}
	switch f {
	case FormatTIFF:
		return EncodeTIFF(ctx, w, b, depth, meta)
	case FormatPNG:
		return EncodePNG(ctx, w, b, depth, meta)
	case FormatNPY:
		return EncodeNPY(ctx, w, b, depth)
	case FormatJPEG:
		return EncodeJPEG(ctx, w, b, 92)
	}
	return errors.Errorf("unknown format %q", f)
}
