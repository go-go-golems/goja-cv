package testimage

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/pkg/errors"
)

// Fixture writers for the decoder tests (GCV-001 Phase 4): the variants
// goja-cv reads but its own writers (buffer.Encode, WriteFile) never produce
// - LZW and PackBits strips, predictors, big-endian TIFF, |u1, >u2, >f4 and
// <f8 NPY - and the minimal DNG. They store the values of a buffer as they
// are (code = round(v · max) for integers, the float itself for float
// samples); what those values mean is the encoding the file declares.

// TIFFOptions select how WriteTIFF stores a buffer.
type TIFFOptions struct {
	Bits         int    // 8, 16, or 32 (float)
	Compression  string // "", "lzw", "deflate", "packbits"
	Predictor    bool   // horizontal differencing (integer samples)
	RowsPerStrip int    // 0 = 16
	Description  string // ImageDescription (e.g. buffer.FormatMetadata)
	BigEndian    bool
}

type tiffTag struct {
	tag   uint16
	typ   uint16 // 2 ASCII, 3 SHORT, 4 LONG, 5 RATIONAL
	vals  []uint32
	ascii string
}

// tiffWriter lays out one TIFF file: the header, image data, then the IFDs.
type tiffWriter struct {
	bo  binary.ByteOrder
	buf bytes.Buffer
}

func newTIFFWriter(bigEndian bool) *tiffWriter {
	w := &tiffWriter{bo: binary.LittleEndian}
	if bigEndian {
		w.bo = binary.BigEndian
		w.buf.WriteString("MM\x00*")
	} else {
		w.buf.WriteString("II*\x00")
	}
	w.buf.Write([]byte{0, 0, 0, 0}) // first IFD offset, patched later
	return w
}

func (w *tiffWriter) align() {
	if w.buf.Len()%2 == 1 {
		w.buf.WriteByte(0)
	}
}

// data appends b and returns its offset.
func (w *tiffWriter) data(b []byte) uint32 {
	w.align()
	off := uint32(w.buf.Len()) // #nosec G115 -- fixtures are small
	w.buf.Write(b)
	return off
}

// ifd appends a directory with tags (sorted) and returns its offset; next is
// the offset of the next directory (0 = none).
func (w *tiffWriter) ifd(tags []tiffTag, next uint32) uint32 {
	sort.Slice(tags, func(i, j int) bool { return tags[i].tag < tags[j].tag })
	// out-of-line values first
	type pending struct {
		raw [4]byte
	}
	entries := make([]pending, len(tags))
	for i, t := range tags {
		var payload []byte
		count := uint32(len(t.vals)) // #nosec G115 -- fixtures are small
		switch t.typ {
		case 2:
			payload = append([]byte(t.ascii), 0)
			count = uint32(len(payload)) // #nosec G115 -- fixtures are small
		case 3:
			payload = make([]byte, 2*len(t.vals))
			for k, v := range t.vals {
				w.bo.PutUint16(payload[2*k:], uint16(v)) // #nosec G115 -- SHORT values
			}
		case 4:
			payload = make([]byte, 4*len(t.vals))
			for k, v := range t.vals {
				w.bo.PutUint32(payload[4*k:], v)
			}
		case 5:
			payload = make([]byte, 4*len(t.vals)) // pairs num, den
			for k, v := range t.vals {
				w.bo.PutUint32(payload[4*k:], v)
			}
			count /= 2
		}
		if len(payload) <= 4 {
			copy(entries[i].raw[:], payload)
		} else {
			w.bo.PutUint32(entries[i].raw[:], w.data(payload))
		}
		tags[i].vals = []uint32{count}
	}
	w.align()
	off := uint32(w.buf.Len()) // #nosec G115 -- fixtures are small
	var b [12]byte
	w.bo.PutUint16(b[:2], uint16(len(tags))) // #nosec G115 -- few tags
	w.buf.Write(b[:2])
	for i, t := range tags {
		w.bo.PutUint16(b[0:], t.tag)
		w.bo.PutUint16(b[2:], t.typ)
		w.bo.PutUint32(b[4:], t.vals[0])
		copy(b[8:], entries[i].raw[:])
		w.buf.Write(b[:12])
	}
	w.bo.PutUint32(b[:4], next)
	w.buf.Write(b[:4])
	return off
}

func (w *tiffWriter) bytes(first uint32) []byte {
	out := w.buf.Bytes()
	w.bo.PutUint32(out[4:], first)
	return out
}

// samples serializes b's values interleaved (chunky) with bits per sample.
func samples(b *buffer.Buffer, bits int, bo binary.ByteOrder) []byte {
	n := b.N()
	bps := bits / 8
	out := make([]byte, n*b.C*bps)
	for i := 0; i < n; i++ {
		for c := 0; c < b.C; c++ {
			v := float64(b.Pix[c*n+i])
			s := out[(i*b.C+c)*bps:]
			switch bits {
			case 8:
				s[0] = uint8(math.Round(math.Max(0, math.Min(1, v)) * 255))
			case 16:
				bo.PutUint16(s, uint16(math.Round(math.Max(0, math.Min(1, v))*65535)))
			case 32:
				bo.PutUint32(s, math.Float32bits(float32(v)))
			}
		}
	}
	return out
}

// imageStrips writes b's strips (compressed per o) and returns the tags that
// describe them.
func (w *tiffWriter) imageStrips(b *buffer.Buffer, o TIFFOptions, photometric uint32) ([]tiffTag, error) {
	bits := o.Bits
	if bits == 0 {
		bits = 16
	}
	rps := o.RowsPerStrip
	if rps <= 0 {
		rps = 16
	}
	raw := samples(b, bits, w.bo)
	rowBytes := b.W * b.C * bits / 8
	var offsets, counts []uint32
	for y0 := 0; y0 < b.H; y0 += rps {
		y1 := min(b.H, y0+rps)
		strip := append([]byte(nil), raw[y0*rowBytes:y1*rowBytes]...)
		if o.Predictor {
			for y := 0; y < y1-y0; y++ {
				predict(strip[y*rowBytes:(y+1)*rowBytes], b.C, bits/8, w.bo)
			}
		}
		var err error
		switch o.Compression {
		case "lzw":
			strip = lzwEncode(strip)
		case "deflate":
			var zb bytes.Buffer
			zw := zlib.NewWriter(&zb)
			if _, err = zw.Write(strip); err == nil {
				err = zw.Close()
			}
			strip = zb.Bytes()
		case "packbits":
			strip = packBits(strip)
		case "":
		default:
			err = errors.Errorf("unknown compression %q", o.Compression)
		}
		if err != nil {
			return nil, err
		}
		offsets = append(offsets, w.data(strip))
		counts = append(counts, uint32(len(strip))) // #nosec G115 -- fixtures are small
	}
	comp := map[string]uint32{"": 1, "lzw": 5, "deflate": 8, "packbits": 32773}[o.Compression]
	bps := make([]uint32, b.C)
	for i := range bps {
		bps[i] = uint32(bits) // #nosec G115 -- 8, 16 or 32
	}
	format := uint32(1)
	if bits == 32 {
		format = 3
	}
	tags := []tiffTag{
		{tag: 256, typ: 4, vals: []uint32{uint32(b.W)}}, // #nosec G115 -- fixtures are small
		{tag: 257, typ: 4, vals: []uint32{uint32(b.H)}}, // #nosec G115 -- fixtures are small
		{tag: 258, typ: 3, vals: bps},
		{tag: 259, typ: 3, vals: []uint32{comp}},
		{tag: 262, typ: 3, vals: []uint32{photometric}},
		{tag: 273, typ: 4, vals: offsets},
		{tag: 277, typ: 3, vals: []uint32{uint32(b.C)}}, // #nosec G115 -- 1 or 3
		{tag: 278, typ: 4, vals: []uint32{uint32(rps)}}, // #nosec G115 -- fixtures are small
		{tag: 279, typ: 4, vals: counts},
		{tag: 284, typ: 3, vals: []uint32{1}},
		{tag: 339, typ: 3, vals: []uint32{format}},
	}
	if o.Predictor {
		tags = append(tags, tiffTag{tag: 317, typ: 3, vals: []uint32{2}})
	}
	if o.Description != "" {
		tags = append(tags, tiffTag{tag: 270, typ: 2, ascii: o.Description})
	}
	return tags, nil
}

// TIFFBytes encodes b (1 or 3 channels) as a TIFF.
func TIFFBytes(b *buffer.Buffer, o TIFFOptions) ([]byte, error) {
	w := newTIFFWriter(o.BigEndian)
	photometric := uint32(1)
	if b.C == 3 {
		photometric = 2
	}
	tags, err := w.imageStrips(b, o, photometric)
	if err != nil {
		return nil, err
	}
	return w.bytes(w.ifd(tags, 0)), nil
}

// WriteTIFF writes b as a TIFF file.
func WriteTIFF(path string, b *buffer.Buffer, o TIFFOptions) error {
	data, err := TIFFBytes(b, o)
	if err != nil {
		return err
	}
	return errors.Wrap(os.WriteFile(path, data, 0o644), "write tiff") // #nosec G306 -- test fixtures
}

// DNGOptions select optional DNG tags.
type DNGOptions struct {
	BlackLevel, WhiteLevel uint32 // 0 = absent
	CFA                    bool   // write CFA photometric (refused by the reader)
}

// DNGBytes encodes b (3 channels, linear values in [0, 1]) as a minimal
// VueScan-like DNG: IFD0 is an 8-bit RGB thumbnail (NewSubfileType 1) whose
// SubIFDs hold the full-resolution LinearRaw image (16-bit, 3 samples,
// uncompressed). With BlackLevel/WhiteLevel the stored codes are
// black + v·(white − black).
func DNGBytes(b *buffer.Buffer, o DNGOptions) ([]byte, error) {
	if b.C != 3 {
		return nil, errors.New("a DNG fixture needs 3 channels")
	}
	w := newTIFFWriter(false)
	raw := b
	if o.WhiteLevel != 0 {
		raw = b.Clone()
		lo, hi := float32(o.BlackLevel)/65535, float32(o.WhiteLevel)/65535
		for i, v := range raw.Pix {
			raw.Pix[i] = lo + v*(hi-lo)
		}
	}
	photometric := uint32(34892)
	if o.CFA {
		photometric = 32803
	}
	rawTags, err := w.imageStrips(raw, TIFFOptions{Bits: 16, RowsPerStrip: b.H}, photometric)
	if err != nil {
		return nil, err
	}
	rawTags = append(rawTags, tiffTag{tag: 254, typ: 4, vals: []uint32{0}})
	if o.WhiteLevel != 0 {
		rawTags = append(rawTags,
			tiffTag{tag: 50714, typ: 4, vals: []uint32{o.BlackLevel}},
			tiffTag{tag: 50717, typ: 4, vals: []uint32{o.WhiteLevel}})
	}
	sub := w.ifd(rawTags, 0)
	thumb := buffer.New(4, 4, 3, 1)
	thumbTags, err := w.imageStrips(thumb, TIFFOptions{Bits: 8, RowsPerStrip: 4}, 2)
	if err != nil {
		return nil, err
	}
	thumbTags = append(thumbTags,
		tiffTag{tag: 254, typ: 4, vals: []uint32{1}},
		tiffTag{tag: 330, typ: 4, vals: []uint32{sub}},
		tiffTag{tag: 50706, typ: 4, vals: []uint32{0x00000401}}, // DNGVersion (BYTE[4] in real files; the reader checks presence)
	)
	return w.bytes(w.ifd(thumbTags, 0)), nil
}

// WriteDNG writes b as a minimal LinearRaw DNG.
func WriteDNG(path string, b *buffer.Buffer, o DNGOptions) error {
	data, err := DNGBytes(b, o)
	if err != nil {
		return err
	}
	return errors.Wrap(os.WriteFile(path, data, 0o644), "write dng") // #nosec G306 -- test fixtures
}

// NPYBytes encodes b as a NumPy .npy array of shape (H, W) for one channel
// or (H, W, 3), with dtype "|u1", "<u2", ">u2", "<f4", ">f4" or "<f8".
func NPYBytes(b *buffer.Buffer, dtype string) ([]byte, error) {
	var bo binary.ByteOrder = binary.LittleEndian
	if strings.HasPrefix(dtype, ">") {
		bo = binary.BigEndian
	}
	shape := fmt.Sprintf("(%d, %d)", b.H, b.W)
	if b.C == 3 {
		shape = fmt.Sprintf("(%d, %d, 3)", b.H, b.W)
	}
	header := fmt.Sprintf("{'descr': '%s', 'fortran_order': False, 'shape': %s, }", dtype, shape)
	pad := 64 - (10+len(header)+1)%64
	header += strings.Repeat(" ", pad%64) + "\n"
	var out bytes.Buffer
	out.WriteString("\x93NUMPY\x01\x00")
	var hl [2]byte
	binary.LittleEndian.PutUint16(hl[:], uint16(len(header))) // #nosec G115 -- a short header
	out.Write(hl[:])
	out.WriteString(header)
	n := b.N()
	var s [8]byte
	for i := 0; i < n; i++ {
		for c := 0; c < b.C; c++ {
			v := float64(b.Pix[c*n+i])
			switch dtype {
			case "|u1":
				out.WriteByte(uint8(math.Round(math.Max(0, math.Min(1, v)) * 255)))
			case "<u2", ">u2":
				bo.PutUint16(s[:], uint16(math.Round(math.Max(0, math.Min(1, v))*65535)))
				out.Write(s[:2])
			case "<f4", ">f4":
				bo.PutUint32(s[:], math.Float32bits(float32(v)))
				out.Write(s[:4])
			case "<f8":
				bo.PutUint64(s[:], math.Float64bits(v))
				out.Write(s[:8])
			default:
				return nil, errors.Errorf("unsupported dtype %q", dtype)
			}
		}
	}
	return out.Bytes(), nil
}

// WriteNPY writes b as a .npy file.
func WriteNPY(path string, b *buffer.Buffer, dtype string) error {
	data, err := NPYBytes(b, dtype)
	if err != nil {
		return err
	}
	return errors.Wrap(os.WriteFile(path, data, 0o644), "write npy") // #nosec G306 -- test fixtures
}

// predict applies horizontal differencing to one row (the inverse of the
// reader's Predictor 2).
func predict(row []byte, spp, bps int, bo binary.ByteOrder) {
	switch bps {
	case 1:
		for i := len(row) - 1; i >= spp; i-- {
			row[i] -= row[i-spp]
		}
	case 2:
		for i := len(row) - 2; i >= 2*spp; i -= 2 {
			bo.PutUint16(row[i:], bo.Uint16(row[i:])-bo.Uint16(row[i-2*spp:]))
		}
	}
}

// packBits run-length encodes data (literal runs only, plus repeats of 3+).
func packBits(data []byte) []byte {
	var out []byte
	for i := 0; i < len(data); {
		run := 1
		for i+run < len(data) && run < 128 && data[i+run] == data[i] {
			run++
		}
		if run >= 3 {
			out = append(out, byte(int8(1-run)), data[i]) // #nosec G115 -- run in [3, 128]
			i += run
			continue
		}
		j := i
		for j < len(data) && j-i < 128 && (j+2 >= len(data) || data[j] != data[j+1] || data[j] != data[j+2]) {
			j++
		}
		out = append(out, byte(j-i-1)) // #nosec G115 -- at most 127
		out = append(out, data[i:j]...)
		i = j
	}
	return out
}

// lzwEncode is TIFF LZW (MSB bit order, 9-12 bit codes, "early change": the
// code width grows one code before the table reaches its limit).
func lzwEncode(data []byte) []byte {
	const clearCode, eoiCode = 256, 257
	var out []byte
	var acc uint32
	var nbits uint
	width := uint(9)
	emit := func(code int) {
		acc = acc<<width | uint32(code) // #nosec G115 -- codes < 4096
		nbits += width
		for nbits >= 8 {
			out = append(out, byte(acc>>(nbits-8))) // #nosec G115 -- the next 8 bits of the accumulator
			nbits -= 8
		}
	}
	table := map[int]int{}
	next := 258
	reset := func() {
		table = map[int]int{}
		next = 258
		width = 9
	}
	emit(clearCode)
	if len(data) == 0 {
		emit(eoiCode)
	} else {
		prefix := int(data[0])
		for _, c := range data[1:] {
			key := prefix<<8 | int(c)
			if code, ok := table[key]; ok {
				prefix = code
				continue
			}
			emit(prefix)
			table[key] = next
			next++
			if next >= 1<<width { // early change: the reader switches when its newest code + 1 overflows
				if width == 12 {
					emit(clearCode)
					reset()
				} else {
					width++
				}
			}
			prefix = int(c)
		}
		emit(prefix)
		next++
		if next >= 1<<width && width < 12 {
			width++
		}
		emit(eoiCode)
	}
	if nbits > 0 {
		out = append(out, byte(acc<<(8-nbits))) // #nosec G115 -- the last bits, left-aligned
	}
	return out
}
