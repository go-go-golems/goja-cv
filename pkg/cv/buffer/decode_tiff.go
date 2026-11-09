package buffer

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"image"
	"io"
	"math"

	"github.com/go-go-golems/goja-cv/pkg/cv/parallel"
	"github.com/pkg/errors"
	"golang.org/x/image/tiff/lzw"
	"golang.org/x/sync/errgroup"
)

// tiffDecoder is goja-cv's own TIFF reader (DNG is a TIFF): one code path for
// probing, SubIFDs, integer and float samples, LinearRaw, LZW / Deflate /
// PackBits strips with the horizontal predictor, and windowed reads. BigTIFF,
// tiles, planar data and CFA raw data are refused with E_FORMAT.
type tiffDecoder struct{}

func (tiffDecoder) Name() string { return "tiff" }

func (tiffDecoder) Sniff(head []byte) bool {
	return bytes.HasPrefix(head, []byte("II*\x00")) || bytes.HasPrefix(head, []byte("MM\x00*")) ||
		bytes.HasPrefix(head, []byte("II+\x00")) || bytes.HasPrefix(head, []byte("MM\x00+"))
}

// TIFF tags the reader uses.
const (
	tagNewSubfileType    = 254
	tagImageWidth        = 256
	tagImageLength       = 257
	tagBitsPerSample     = 258
	tagCompression       = 259
	tagPhotometric       = 262
	tagImageDescription  = 270
	tagStripOffsets      = 273
	tagSamplesPerPixel   = 277
	tagRowsPerStrip      = 278
	tagStripByteCounts   = 279
	tagPlanarConfig      = 284
	tagPredictor         = 317
	tagTileWidth         = 322
	tagTileOffsets       = 324
	tagSubIFDs           = 330
	tagSampleFormat      = 339
	tagDNGVersion        = 50706
	tagLinearizationTbl  = 50712
	tagBlackLevel        = 50714
	tagWhiteLevel        = 50717
	photometricWhiteZero = 0
	photometricBlackZero = 1
	photometricRGB       = 2
	photometricCFA       = 32803
	photometricLinearRaw = 34892
)

// TIFF field types and their sizes.
var tiffTypeSize = map[uint16]int{1: 1, 2: 1, 3: 2, 4: 4, 5: 8, 6: 1, 7: 1, 8: 2, 9: 4, 10: 8, 11: 4, 12: 8, 13: 4}

type tiffEntry struct {
	typ   uint16
	count uint32
	raw   [4]byte // the value, or the offset of the value
}

type tiffIFD map[uint16]tiffEntry

type tiffFile struct {
	r    io.ReadSeeker
	bo   binary.ByteOrder
	size int64
}

func (t *tiffFile) readAt(off int64, n int) ([]byte, error) {
	if off < 0 || n < 0 || off+int64(n) > t.size {
		return nil, formatErr("TIFF data at %d (+%d bytes) is outside the %d-byte file", off, n, t.size)
	}
	if _, err := t.r.Seek(off, io.SeekStart); err != nil {
		return nil, errors.Wrap(err, "seek")
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(t.r, b); err != nil {
		return nil, formatErr("truncated TIFF data at %d", off)
	}
	return b, nil
}

// data returns the bytes of an entry's value.
func (t *tiffFile) data(e tiffEntry) ([]byte, error) {
	sz, ok := tiffTypeSize[e.typ]
	if !ok {
		return nil, formatErr("TIFF field type %d", e.typ)
	}
	n := int64(sz) * int64(e.count)
	if n > t.size || n > 1<<26 {
		return nil, formatErr("TIFF field of %d bytes", n)
	}
	if n <= 4 {
		return e.raw[:n], nil
	}
	return t.readAt(int64(t.bo.Uint32(e.raw[:])), int(n))
}

// ints reads an integer-valued entry (BYTE, SHORT, LONG, IFD).
func (t *tiffFile) ints(e tiffEntry) ([]int64, error) {
	b, err := t.data(e)
	if err != nil {
		return nil, err
	}
	out := make([]int64, e.count)
	for i := range out {
		switch e.typ {
		case 1, 7:
			out[i] = int64(b[i])
		case 3:
			out[i] = int64(t.bo.Uint16(b[2*i:]))
		case 4, 13:
			out[i] = int64(t.bo.Uint32(b[4*i:]))
		case 8:
			out[i] = int64(int16(t.bo.Uint16(b[2*i:]))) // #nosec G115 -- SSHORT: the bits are a signed value
		case 9:
			out[i] = int64(int32(t.bo.Uint32(b[4*i:]))) // #nosec G115 -- SLONG: the bits are a signed value
		default:
			return nil, formatErr("TIFF field type %d where integers are expected", e.typ)
		}
	}
	return out, nil
}

// floats reads a numeric entry as float64 (integers, RATIONAL, FLOAT, DOUBLE).
func (t *tiffFile) floats(e tiffEntry) ([]float64, error) {
	switch e.typ {
	case 5, 10, 11, 12:
	default:
		is, err := t.ints(e)
		if err != nil {
			return nil, err
		}
		out := make([]float64, len(is))
		for i, v := range is {
			out[i] = float64(v)
		}
		return out, nil
	}
	b, err := t.data(e)
	if err != nil {
		return nil, err
	}
	out := make([]float64, e.count)
	for i := range out {
		switch e.typ {
		case 5:
			num, den := t.bo.Uint32(b[8*i:]), t.bo.Uint32(b[8*i+4:])
			if den != 0 {
				out[i] = float64(num) / float64(den)
			}
		case 10:
			num, den := int32(t.bo.Uint32(b[8*i:])), int32(t.bo.Uint32(b[8*i+4:])) // #nosec G115 -- SRATIONAL: signed values
			if den != 0 {
				out[i] = float64(num) / float64(den)
			}
		case 11:
			out[i] = float64(math.Float32frombits(t.bo.Uint32(b[4*i:])))
		case 12:
			out[i] = math.Float64frombits(t.bo.Uint64(b[8*i:]))
		}
	}
	return out, nil
}

func (t *tiffFile) int1(ifd tiffIFD, tag uint16, def int64) (int64, error) {
	e, ok := ifd[tag]
	if !ok {
		return def, nil
	}
	v, err := t.ints(e)
	if err != nil {
		return 0, err
	}
	if len(v) == 0 {
		return def, nil
	}
	return v[0], nil
}

func (t *tiffFile) ascii(ifd tiffIFD, tag uint16) string {
	e, ok := ifd[tag]
	if !ok || e.typ != 2 {
		return ""
	}
	b, err := t.data(e)
	if err != nil {
		return ""
	}
	return string(bytes.TrimRight(b, "\x00"))
}

// readIFD reads the directory at off and returns it with the offset of the
// next one.
func (t *tiffFile) readIFD(off int64) (tiffIFD, int64, error) {
	b, err := t.readAt(off, 2)
	if err != nil {
		return nil, 0, err
	}
	n := int(t.bo.Uint16(b))
	if n == 0 || n > 4096 {
		return nil, 0, formatErr("TIFF directory with %d entries", n)
	}
	b, err = t.readAt(off+2, 12*n+4)
	if err != nil {
		return nil, 0, err
	}
	ifd := tiffIFD{}
	for i := 0; i < n; i++ {
		e := b[12*i:]
		var raw [4]byte
		copy(raw[:], e[8:12])
		ifd[t.bo.Uint16(e)] = tiffEntry{typ: t.bo.Uint16(e[2:]), count: t.bo.Uint32(e[4:]), raw: raw}
	}
	return ifd, int64(t.bo.Uint32(b[12*n:])), nil
}

// tiffImage is the parsed description of the image a TIFF file is read as.
type tiffImage struct {
	info         Info
	spp, bits    int
	float        bool
	compression  int64
	predictor    int64
	rowsPerStrip int
	offsets      []int64
	counts       []int64
	black        []float64 // DNG BlackLevel per channel (nil: none)
	white        []float64 // DNG WhiteLevel per channel (nil: none)
}

func openTIFF(r io.ReadSeeker) (*tiffFile, int64, error) {
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, 0, errors.Wrap(err, "seek")
	}
	t := &tiffFile{r: r, size: size}
	head, err := t.readAt(0, 8)
	if err != nil {
		return nil, 0, formatErr("truncated TIFF header")
	}
	switch string(head[:2]) {
	case "II":
		t.bo = binary.LittleEndian
	case "MM":
		t.bo = binary.BigEndian
	default:
		return nil, 0, formatErr("not a TIFF file")
	}
	switch t.bo.Uint16(head[2:]) {
	case 42:
	case 43:
		return nil, 0, formatErr("BigTIFF is not supported")
	default:
		return nil, 0, formatErr("not a TIFF file (magic %d)", t.bo.Uint16(head[2:]))
	}
	return t, int64(t.bo.Uint32(head[4:])), nil
}

// parseTIFF reads the directories and describes the image to decode: the
// first full-resolution image (NewSubfileType 0), preferring LinearRaw data
// (the raw image of a DNG, top-level or in IFD0's SubIFDs).
func parseTIFF(r io.ReadSeeker) (*tiffFile, *tiffImage, error) {
	t, first, err := openTIFF(r)
	if err != nil {
		return nil, nil, err
	}
	var all []tiffIFD
	seen := map[int64]bool{}
	var walk func(off int64, depth int) error
	walk = func(off int64, depth int) error {
		for off != 0 && len(all) < 64 {
			if seen[off] {
				return nil
			}
			seen[off] = true
			ifd, next, err := t.readIFD(off)
			if err != nil {
				return err
			}
			all = append(all, ifd)
			if e, ok := ifd[tagSubIFDs]; ok && depth < 2 {
				subs, err := t.ints(e)
				if err != nil {
					return err
				}
				for _, s := range subs {
					if err := walk(s, depth+1); err != nil {
						return err
					}
				}
			}
			off = next
		}
		return nil
	}
	if err := walk(first, 0); err != nil {
		return nil, nil, err
	}
	if len(all) == 0 {
		return nil, nil, formatErr("TIFF without directories")
	}
	var pick tiffIFD
	for _, ifd := range all {
		sub, err := t.int1(ifd, tagNewSubfileType, 0)
		if err != nil {
			return nil, nil, err
		}
		if _, ok := ifd[tagImageWidth]; !ok || sub != 0 {
			continue
		}
		ph, err := t.int1(ifd, tagPhotometric, -1)
		if err != nil {
			return nil, nil, err
		}
		if ph == photometricLinearRaw || ph == photometricCFA {
			pick = ifd
			break
		}
		if pick == nil {
			pick = ifd
		}
	}
	if pick == nil {
		return nil, nil, formatErr("TIFF without a full-resolution image")
	}
	img, err := t.describe(pick)
	if err != nil {
		return nil, nil, err
	}
	if _, ok := all[0][tagDNGVersion]; ok {
		img.info.Format = "dng"
	}
	if d := t.ascii(all[0], tagImageDescription); d != "" {
		applyMetadata(&img.info, d)
	}
	if d := t.ascii(pick, tagImageDescription); d != "" {
		applyMetadata(&img.info, d)
	}
	return t, img, nil
}

func (t *tiffFile) describe(ifd tiffIFD) (*tiffImage, error) {
	if _, ok := ifd[tagTileWidth]; ok {
		return nil, formatErr("tiled TIFF is not supported (only strips)")
	}
	if _, ok := ifd[tagTileOffsets]; ok {
		return nil, formatErr("tiled TIFF is not supported (only strips)")
	}
	if _, ok := ifd[tagLinearizationTbl]; ok {
		return nil, formatErr("DNG with a LinearizationTable is not supported")
	}
	get := func(tag uint16, def int64) int64 {
		v, err := t.int1(ifd, tag, def)
		if err != nil {
			return -1
		}
		return v
	}
	w, h := get(tagImageWidth, 0), get(tagImageLength, 0)
	img := &tiffImage{info: Info{Format: "tiff", W: int(w), H: int(h), Sample: "uint"}}
	if err := checkSize(img.info); err != nil {
		return nil, err
	}
	img.spp = int(get(tagSamplesPerPixel, 1))
	if img.spp < 1 || img.spp > 4 {
		return nil, formatErr("TIFF with %d samples per pixel", img.spp)
	}
	bits := []int64{1}
	if e, ok := ifd[tagBitsPerSample]; ok {
		b, err := t.ints(e)
		if err != nil {
			return nil, err
		}
		bits = b
	}
	if len(bits) == 0 {
		return nil, formatErr("TIFF with an empty BitsPerSample field")
	}
	img.bits = int(bits[0])
	for _, b := range bits {
		if int(b) != img.bits {
			return nil, formatErr("TIFF with mixed bits per sample %v", bits)
		}
	}
	switch get(tagSampleFormat, 1) {
	case 1:
	case 3:
		img.float = true
		img.info.Sample = "float"
	case 2:
		return nil, formatErr("signed-integer TIFF samples are not supported")
	default:
		return nil, formatErr("TIFF sample format %d is not supported", get(tagSampleFormat, 1))
	}
	switch {
	case img.float && img.bits == 32:
	case img.float:
		return nil, formatErr("%d-bit float TIFF samples are not supported (only 32-bit)", img.bits)
	case img.bits == 8 || img.bits == 16:
	default:
		return nil, formatErr("%d-bit TIFF samples are not supported (8, 16 or 32-bit float)", img.bits)
	}
	img.info.Bits = img.bits
	ph := get(tagPhotometric, -1)
	switch ph {
	case photometricBlackZero:
		if img.spp > 2 {
			return nil, formatErr("greyscale TIFF with %d samples per pixel", img.spp)
		}
		img.info.Channels = 1
	case photometricRGB, photometricLinearRaw:
		switch {
		case img.spp >= 3:
			img.info.Channels = 3
		case ph == photometricLinearRaw:
			img.info.Channels = 1
		default:
			return nil, formatErr("RGB TIFF with %d samples per pixel", img.spp)
		}
	case photometricCFA:
		return nil, formatErr("CFA (mosaiced) raw data is not supported; goja-cv reads LinearRaw DNGs")
	case photometricWhiteZero:
		return nil, formatErr("WhiteIsZero TIFF is not supported")
	default:
		return nil, formatErr("TIFF photometric interpretation %d is not supported (greyscale, RGB or LinearRaw)", ph)
	}
	img.info.Encoding = EncSRGB
	if img.float || ph == photometricLinearRaw {
		img.info.Encoding = EncLinear
	}
	if ph == photometricLinearRaw {
		img.info.Notes = append(img.info.Notes, "camera RGB primaries (DNG)")
	}
	if pc := get(tagPlanarConfig, 1); pc != 1 {
		return nil, formatErr("planar TIFF (PlanarConfiguration %d) is not supported", pc)
	}
	img.compression = get(tagCompression, 1)
	switch img.compression {
	case 1:
	case 5:
		img.info.Notes = append(img.info.Notes, "LZW")
	case 8, 32946:
		img.info.Notes = append(img.info.Notes, "Deflate")
	case 32773:
		img.info.Notes = append(img.info.Notes, "PackBits")
	default:
		return nil, formatErr("TIFF compression %d is not supported (none, LZW, Deflate, PackBits)", img.compression)
	}
	img.predictor = get(tagPredictor, 1)
	switch img.predictor {
	case 1:
	case 2:
		if img.float {
			return nil, formatErr("the horizontal predictor on float samples is not supported")
		}
	default:
		return nil, formatErr("TIFF predictor %d is not supported", img.predictor)
	}
	img.rowsPerStrip = int(get(tagRowsPerStrip, h))
	if img.rowsPerStrip <= 0 || img.rowsPerStrip > int(h) {
		img.rowsPerStrip = int(h)
	}
	oe, ok1 := ifd[tagStripOffsets]
	ce, ok2 := ifd[tagStripByteCounts]
	if !ok1 || !ok2 {
		return nil, formatErr("TIFF without strip offsets")
	}
	var err error
	if img.offsets, err = t.ints(oe); err != nil {
		return nil, err
	}
	if img.counts, err = t.ints(ce); err != nil {
		return nil, err
	}
	strips := (int(h) + img.rowsPerStrip - 1) / img.rowsPerStrip
	if len(img.offsets) < strips || len(img.counts) < strips {
		return nil, formatErr("TIFF has %d strips for %d rows of %d", len(img.offsets), h, img.rowsPerStrip)
	}
	if e, ok := ifd[tagBlackLevel]; ok {
		if img.black, err = t.floats(e); err != nil {
			return nil, err
		}
	}
	if e, ok := ifd[tagWhiteLevel]; ok {
		if img.white, err = t.floats(e); err != nil {
			return nil, err
		}
	}
	if img.black != nil || img.white != nil {
		img.info.scaled = true
		img.info.Notes = append(img.info.Notes, "BlackLevel/WhiteLevel applied")
	}
	return img, nil
}

func (tiffDecoder) Probe(r io.ReadSeeker) (Info, error) {
	_, img, err := parseTIFF(r)
	if err != nil {
		return Info{}, err
	}
	return img.info, nil
}

// level returns the DNG black and white levels of channel c, or (0, max).
func (img *tiffImage) level(c int) (float64, float64) {
	pick := func(l []float64, def float64) float64 {
		switch {
		case len(l) == 0:
			return def
		case c < len(l):
			return l[c]
		}
		return l[0]
	}
	maxCode := float64(uint64(1)<<img.bits - 1)
	return pick(img.black, 0), pick(img.white, maxCode)
}

func (tiffDecoder) Decode(ctx context.Context, r io.ReadSeeker, o DecodeOptions) (*Buffer, Info, error) {
	t, img, err := parseTIFF(r)
	if err != nil {
		return nil, Info{}, err
	}
	win, err := window(o, img.info.W, img.info.H)
	if err != nil {
		return nil, Info{}, err
	}
	w, h, C := win.Dx(), win.Dy(), img.info.Channels
	bps := img.bits / 8
	pixBytes := img.spp * bps
	rowBytes := img.info.W * pixBytes
	// the strips must be able to hold the rows before anything is allocated
	// (a malformed header must not ask for gigabytes)
	ratio := int64(1)
	if img.compression != 1 {
		ratio = 4096 // more than any LZW, Deflate or PackBits ratio
	}
	for s := win.Min.Y / img.rowsPerStrip; s <= (win.Max.Y-1)/img.rowsPerStrip; s++ {
		rows := min(img.rowsPerStrip, img.info.H-s*img.rowsPerStrip)
		if img.counts[s] < 0 || img.counts[s]*ratio < int64(rows)*int64(rowBytes) || img.offsets[s] < 0 || img.offsets[s]+img.counts[s] > t.size {
			return nil, Info{}, formatErr("TIFF strip %d (%d bytes at %d) cannot hold %d rows of %d bytes", s, img.counts[s], img.offsets[s], rows, rowBytes)
		}
	}
	out := New(w, h, C, 1)
	conv := img.sampleConverter(t.bo)
	put := func(y int, row []byte) { // row holds samples from x = win.Min.X
		n := w * h
		for x := 0; x < w; x++ {
			s := row[x*pixBytes:]
			for c := 0; c < C; c++ {
				out.Pix[c*n+y*w+x] = conv(s[c*bps:], c)
			}
		}
	}
	if img.compression == 1 && img.predictor == 1 {
		// uncompressed: read only the window of each row
		for y := win.Min.Y; y < win.Max.Y; y++ {
			if y%256 == 0 && ctx.Err() != nil {
				return nil, Info{}, ctx.Err()
			}
			s := y / img.rowsPerStrip
			off := img.offsets[s] + int64(y-s*img.rowsPerStrip)*int64(rowBytes) + int64(win.Min.X*pixBytes)
			row, err := t.readAt(off, w*pixBytes)
			if err != nil {
				return nil, Info{}, err
			}
			put(y-win.Min.Y, row)
		}
		return out, img.info, nil
	}
	// compressed: read the strips that intersect the window, decompress in parallel
	s0, s1 := win.Min.Y/img.rowsPerStrip, (win.Max.Y-1)/img.rowsPerStrip
	raw := make([][]byte, s1-s0+1)
	for s := s0; s <= s1; s++ {
		if raw[s-s0], err = t.readAt(img.offsets[s], int(img.counts[s])); err != nil {
			return nil, Info{}, err
		}
	}
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(8)
	for s := s0; s <= s1; s++ {
		g.Go(func() (err error) { //nolint:nonamedreturns // recover needs named results
			defer parallel.Recover(&err)
			if err := gctx.Err(); err != nil {
				return err
			}
			r0 := s * img.rowsPerStrip
			rows := min(img.rowsPerStrip, img.info.H-r0)
			data, err := decompress(raw[s-s0], img.compression, rows*rowBytes)
			if err != nil {
				return err
			}
			for k := 0; k < rows; k++ {
				y := r0 + k
				if y < win.Min.Y || y >= win.Max.Y {
					continue
				}
				row := data[k*rowBytes : (k+1)*rowBytes]
				if img.predictor == 2 {
					undoPredictor(row, img.spp, bps, t.bo)
				}
				put(y-win.Min.Y, row[win.Min.X*pixBytes:])
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, Info{}, err
	}
	return out, img.info, nil
}

// sampleConverter maps one stored sample of channel c to its encoded value.
func (img *tiffImage) sampleConverter(bo binary.ByteOrder) func(b []byte, c int) float32 {
	switch {
	case img.float:
		return func(b []byte, _ int) float32 { return math.Float32frombits(bo.Uint32(b)) }
	case img.info.scaled:
		var lo, scale [3]float64
		for c := 0; c < 3; c++ {
			black, white := img.level(c)
			lo[c], scale[c] = black, 1/math.Max(white-black, 1e-9)
		}
		if img.bits == 8 {
			return func(b []byte, c int) float32 { return float32((float64(b[0]) - lo[c]) * scale[c]) }
		}
		return func(b []byte, c int) float32 { return float32((float64(bo.Uint16(b)) - lo[c]) * scale[c]) }
	case img.bits == 8:
		return func(b []byte, _ int) float32 { return float32(b[0]) / 255 }
	}
	return func(b []byte, _ int) float32 { return float32(bo.Uint16(b)) / 65535 }
}

// decompress returns at least want bytes of the decompressed strip.
func decompress(src []byte, compression int64, want int) ([]byte, error) {
	var out []byte
	switch compression {
	case 5:
		rc := lzw.NewReader(bytes.NewReader(src), lzw.MSB, 8)
		defer func() { _ = rc.Close() }()
		out = make([]byte, want)
		n, err := io.ReadFull(rc, out)
		if err != nil && n < want {
			return nil, formatErr("LZW strip: %v (%d of %d bytes)", err, n, want)
		}
	case 8, 32946:
		zr, err := zlib.NewReader(bytes.NewReader(src))
		if err != nil {
			return nil, formatErr("Deflate strip: %v", err)
		}
		defer func() { _ = zr.Close() }()
		out = make([]byte, want)
		if n, err := io.ReadFull(zr, out); err != nil && n < want {
			return nil, formatErr("Deflate strip: %v (%d of %d bytes)", err, n, want)
		}
	case 32773:
		var err error
		if out, err = unpackBits(src, want); err != nil {
			return nil, err
		}
	default:
		out = src
	}
	if len(out) < want {
		return nil, formatErr("strip of %d bytes, want %d", len(out), want)
	}
	return out, nil
}

// unpackBits decodes PackBits (Apple's run-length encoding) up to want bytes.
func unpackBits(src []byte, want int) ([]byte, error) {
	out := make([]byte, 0, want)
	for i := 0; i < len(src) && len(out) < want; {
		n := int(int8(src[i])) // #nosec G115 -- PackBits headers are signed bytes
		i++
		switch {
		case n >= 0: // n+1 literal bytes
			if i+n+1 > len(src) {
				return nil, formatErr("truncated PackBits literal")
			}
			out = append(out, src[i:i+n+1]...)
			i += n + 1
		case n != -128: // repeat the next byte 1-n times
			if i >= len(src) {
				return nil, formatErr("truncated PackBits run")
			}
			for k := 0; k < 1-n; k++ {
				out = append(out, src[i])
			}
			i++
		}
	}
	if len(out) < want {
		return nil, formatErr("PackBits strip of %d bytes, want %d", len(out), want)
	}
	return out[:want], nil
}

// undoPredictor reverses horizontal differencing (Predictor 2) on one row of
// spp samples per pixel of bps bytes each.
func undoPredictor(row []byte, spp, bps int, bo binary.ByteOrder) {
	switch bps {
	case 1:
		for i := spp; i < len(row); i++ {
			row[i] += row[i-spp]
		}
	case 2:
		for i := spp * 2; i+1 < len(row); i += 2 {
			bo.PutUint16(row[i:], bo.Uint16(row[i:])+bo.Uint16(row[i-2*spp:]))
		}
	}
}

// RectXYWH is a window [x, y, w, h] in file px.
func RectXYWH(x, y, w, h int) *image.Rectangle {
	r := image.Rect(x, y, x+w, y+h)
	return &r
}
