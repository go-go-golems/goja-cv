package buffer

import (
	"bufio"
	"context"
	"fmt"
	"image"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/go-go-golems/goja-cv/pkg/cv/parallel"
	"github.com/pkg/errors"
)

// Info describes an image file: what its decoder found in the header.
type Info struct {
	Format   string   `json:"format"`   // "png", "jpeg", "tiff", "dng", "npy"
	W        int      `json:"w"`        // width in file px
	H        int      `json:"h"`        // height in file px
	Channels int      `json:"channels"` // 1 or 3 after decoding (alpha dropped)
	Bits     int      `json:"bits"`     // 8, 16, 32 or 64 per sample in the file
	Sample   string   `json:"sample"`   // "uint" | "float"
	Encoding Encoding `json:"-"`        // what the file implies (cv.open can override it)
	Origin   *[2]int  `json:"origin,omitempty"`
	Notes    []string `json:"notes,omitempty"` // e.g. "camera RGB primaries (DNG)", "LZW"

	// scaled is set when decoded values are not code/max (DNG black and
	// white levels), so the exact integer tables do not apply.
	scaled bool
}

// DecodeOptions select what Decode reads.
type DecodeOptions struct {
	Box *image.Rectangle // read only this window (file px); nil = the whole image
}

// Decoder reads one family of image files. Decode returns encoded values:
// integers divided by their largest code, floats as stored.
type Decoder interface {
	Name() string
	Sniff(head []byte) bool                                                              // magic bytes
	Probe(r io.ReadSeeker) (Info, error)                                                 // header only
	Decode(ctx context.Context, r io.ReadSeeker, o DecodeOptions) (*Buffer, Info, error) // encoded values
}

var (
	decodersMu sync.RWMutex
	decoders   []Decoder
)

// Register adds a decoder; later registrations are tried after earlier ones.
func Register(d Decoder) {
	decodersMu.Lock()
	defer decodersMu.Unlock()
	decoders = append(decoders, d)
}

func init() {
	Register(pngDecoder{})
	Register(jpegDecoder{})
	Register(tiffDecoder{})
	Register(npyDecoder{})
}

// FormatError is a file feature goja-cv does not read (E_FORMAT): tiles,
// planar configuration, CFA raw data, BigTIFF, Fortran order, …
type FormatError struct{ Feature string }

func (e *FormatError) Error() string { return e.Feature }

func formatErr(format string, args ...any) error {
	return &FormatError{Feature: fmt.Sprintf(format, args...)}
}

// IsFormatError reports whether err is a FormatError, and returns it.
func IsFormatError(err error) (*FormatError, bool) {
	var fe *FormatError
	if errors.As(err, &fe) {
		return fe, true
	}
	return nil, false
}

// SupportedFormats names what the decoders read, for error hints.
const SupportedFormats = "PNG (8/16-bit), JPEG, TIFF (8/16-bit integer or 32-bit float; uncompressed, LZW, Deflate, PackBits), LinearRaw DNG, NumPy .npy"

func decoderFor(r io.ReadSeeker) (Decoder, error) {
	head := make([]byte, 16)
	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, errors.Wrap(err, "read header")
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, errors.Wrap(err, "seek")
	}
	decodersMu.RLock()
	defer decodersMu.RUnlock()
	for _, d := range decoders {
		if d.Sniff(head[:n]) {
			return d, nil
		}
	}
	return nil, formatErr("not a supported image file (supported: %s)", SupportedFormats)
}

// ProbeFile reads only the header of an open image file (opened by the
// caller, through the session's workspace).
func ProbeFile(f *os.File) (Info, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return Info{}, errors.Wrap(err, "seek")
	}
	d, err := decoderFor(f)
	if err != nil {
		return Info{}, err
	}
	info, err := d.Probe(f)
	if err != nil {
		return Info{}, err
	}
	return info, checkSize(info)
}

// DecodeFile decodes an open image file (or the window o.Box of it) into
// encoded values at scale 1.
func DecodeFile(ctx context.Context, f *os.File, o DecodeOptions) (*Buffer, Info, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, Info{}, errors.Wrap(err, "seek")
	}
	d, err := decoderFor(f)
	if err != nil {
		return nil, Info{}, err
	}
	return d.Decode(ctx, f, o)
}

// maxPixels bounds width × height so that malformed headers cannot ask for
// absurd allocations (a 7256 × 7326 scan is 53 M px).
const maxPixels = 1 << 30

func checkSize(info Info) error {
	if info.W <= 0 || info.H <= 0 {
		return formatErr("image has no pixels (%d×%d)", info.W, info.H)
	}
	if int64(info.W)*int64(info.H) > maxPixels {
		return formatErr("image of %d×%d px is too large", info.W, info.H)
	}
	return nil
}

// window clips o.Box to w×h; nil means the whole image.
func window(o DecodeOptions, w, h int) (image.Rectangle, error) {
	full := image.Rect(0, 0, w, h)
	if o.Box == nil {
		return full, nil
	}
	b := o.Box.Canon()
	if b.Empty() || !b.In(full) {
		return image.Rectangle{}, errors.Errorf("window %v is not inside the %d×%d image", b, w, h)
	}
	return b, nil
}

// Linearize converts decoded (encoded) values to linear values for encoding
// enc, as the source loader stores them. Integer files use the exact 8- and
// 16-bit sRGB tables; floats and scaled values the exact transfer function.
// It returns b itself for the linear encoding.
func Linearize(ctx context.Context, b *Buffer, info Info, enc Encoding) (*Buffer, error) {
	if enc != EncSRGB {
		return b, nil
	}
	maxCode := 0
	if info.Sample == "uint" && !info.scaled {
		switch info.Bits {
		case 8:
			maxCode = 255
		case 16:
			maxCode = 65535
		}
	}
	if maxCode == 0 {
		return Convert(ctx, b, EncSRGB, SpaceEncoded, SpaceLinear)
	}
	out := NewLike(b)
	n := b.N()
	err := parallel.Rows(ctx, b.H, func(y0, y1 int) {
		for c := 0; c < b.C; c++ {
			src, dst := b.Pix[c*n:(c+1)*n], out.Pix[c*n:(c+1)*n]
			for i := y0 * b.W; i < y1*b.W; i++ {
				code := math.Round(float64(src[i]) * float64(maxCode))
				if maxCode == 255 {
					dst[i] = Linear8(uint8(code))
				} else {
					dst[i] = Linear16(uint16(code))
				}
			}
		}
	})
	return out, err
}

// MetadataKey is the PNG tEXt keyword (and TIFF ImageDescription prefix) of
// goja-cv's own metadata: "goja-cv encoding=linear origin=1536,1536".
const MetadataKey = "goja-cv"

// FormatMetadata renders goja-cv's metadata string.
func FormatMetadata(enc Encoding, origin *[2]int) string {
	s := MetadataKey + " encoding=" + enc.String()
	if origin != nil {
		s += fmt.Sprintf(" origin=%d,%d", origin[0], origin[1])
	}
	return s
}

// parseMetadata reads goja-cv's metadata string. Other strings are ignored
// (ok false).
func parseMetadata(s string) (Encoding, *[2]int, bool) {
	s = strings.TrimSpace(strings.TrimRight(s, "\x00"))
	if !strings.HasPrefix(s, MetadataKey+" ") {
		return EncNone, nil, false
	}
	enc := EncNone
	var origin *[2]int
	for _, f := range strings.Fields(s[len(MetadataKey):]) {
		k, v, ok := strings.Cut(f, "=")
		if !ok {
			continue
		}
		switch k {
		case "encoding":
			if e, ok := ParseEncoding(v); ok {
				enc = e
			}
		case "origin":
			xs, ys, ok := strings.Cut(v, ",")
			x, err1 := strconv.Atoi(xs)
			y, err2 := strconv.Atoi(ys)
			if ok && err1 == nil && err2 == nil {
				origin = &[2]int{x, y}
			}
		}
	}
	return enc, origin, true
}

// applyMetadata overrides info's encoding and origin with goja-cv metadata s.
func applyMetadata(info *Info, s string) {
	enc, origin, ok := parseMetadata(s)
	if !ok {
		return
	}
	if enc != EncNone {
		info.Encoding = enc
	}
	if origin != nil {
		info.Origin = origin
	}
}

// bufReader adapts an io.ReadSeeker positioned at 0 for stdlib decoders.
func bufReader(r io.ReadSeeker) (io.Reader, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, errors.Wrap(err, "seek")
	}
	return bufio.NewReader(r), nil
}
