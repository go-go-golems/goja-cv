package buffer

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// npyDecoder reads NumPy .npy arrays (format v1-v3, C order) of shape
// (H, W), (H, W, 1) or (H, W, 3) with dtype |u1, <u2, >u2, <f4, >f4, <f8 or
// >f8. Values are linear: integers divided by their largest code, floats as
// stored (float64 narrowed to float32). Windows are read row by row.
type npyDecoder struct{}

var npyMagic = []byte("\x93NUMPY")

func (npyDecoder) Name() string           { return "npy" }
func (npyDecoder) Sniff(head []byte) bool { return bytes.HasPrefix(head, npyMagic) }

type npyHeader struct {
	info     Info
	order    binary.ByteOrder
	itemSize int
	float    bool
	data     int64 // offset of the array data
	cDim     int   // samples per pixel in the file (1 or 3)
}

var (
	npyDescr   = regexp.MustCompile(`'descr'\s*:\s*'([^']*)'`)
	npyFortran = regexp.MustCompile(`'fortran_order'\s*:\s*(True|False)`)
	npyShape   = regexp.MustCompile(`'shape'\s*:\s*\(([^)]*)\)`)
)

func readNPYHeader(r io.ReadSeeker) (*npyHeader, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, errors.Wrap(err, "seek")
	}
	pre := make([]byte, 8)
	if _, err := io.ReadFull(r, pre); err != nil || !bytes.HasPrefix(pre, npyMagic) {
		return nil, formatErr("not a .npy file")
	}
	var hlen, start int
	switch pre[6] {
	case 1:
		b := make([]byte, 2)
		if _, err := io.ReadFull(r, b); err != nil {
			return nil, formatErr("truncated .npy header")
		}
		hlen, start = int(binary.LittleEndian.Uint16(b)), 10
	case 2, 3:
		b := make([]byte, 4)
		if _, err := io.ReadFull(r, b); err != nil {
			return nil, formatErr("truncated .npy header")
		}
		hlen, start = int(binary.LittleEndian.Uint32(b)), 12
	default:
		return nil, formatErr(".npy format version %d is not supported", pre[6])
	}
	if hlen <= 0 || hlen > 1<<20 {
		return nil, formatErr(".npy header of %d bytes", hlen)
	}
	hb := make([]byte, hlen)
	if _, err := io.ReadFull(r, hb); err != nil {
		return nil, formatErr("truncated .npy header")
	}
	hs := string(hb)
	h := &npyHeader{data: int64(start + hlen), info: Info{Format: "npy", Encoding: EncLinear}}
	if m := npyFortran.FindStringSubmatch(hs); m == nil {
		return nil, formatErr(".npy header without fortran_order")
	} else if m[1] == "True" {
		return nil, formatErr("Fortran-order .npy arrays are not supported (save with C order: np.ascontiguousarray)")
	}
	m := npyDescr.FindStringSubmatch(hs)
	if m == nil {
		return nil, formatErr(".npy header without descr")
	}
	switch m[1] {
	case "|u1", "<u1", ">u1", "u1":
		h.order, h.itemSize, h.info.Bits, h.info.Sample = binary.LittleEndian, 1, 8, "uint"
	case "<u2", ">u2":
		h.itemSize, h.info.Bits, h.info.Sample = 2, 16, "uint"
	case "<f4", ">f4":
		h.itemSize, h.info.Bits, h.info.Sample, h.float = 4, 32, "float", true
	case "<f8", ">f8":
		h.itemSize, h.info.Bits, h.info.Sample, h.float = 8, 64, "float", true
	default:
		return nil, formatErr(".npy dtype %q is not supported (|u1, <u2, >u2, <f4, >f4, <f8, >f8)", m[1])
	}
	if h.order == nil {
		h.order = binary.LittleEndian
		if strings.HasPrefix(m[1], ">") {
			h.order = binary.BigEndian
		}
	}
	sm := npyShape.FindStringSubmatch(hs)
	if sm == nil {
		return nil, formatErr(".npy header without shape")
	}
	var dims []int
	for _, f := range strings.Split(sm[1], ",") {
		if f = strings.TrimSpace(f); f == "" {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSuffix(f, "L"))
		if err != nil || n < 0 {
			return nil, formatErr(".npy shape (%s)", sm[1])
		}
		dims = append(dims, n)
	}
	switch {
	case len(dims) == 2:
		h.cDim = 1
	case len(dims) == 3 && (dims[2] == 1 || dims[2] == 3):
		h.cDim = dims[2]
	default:
		return nil, formatErr(".npy shape (%s) is not (H, W), (H, W, 1) or (H, W, 3)", sm[1])
	}
	h.info.H, h.info.W, h.info.Channels = dims[0], dims[1], h.cDim
	if err := checkSize(h.info); err != nil {
		return nil, err
	}
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, errors.Wrap(err, "seek")
	}
	if need := h.data + int64(h.info.W)*int64(h.info.H)*int64(h.cDim*h.itemSize); size < need {
		return nil, formatErr("truncated .npy file: %d bytes, the shape needs %d", size, need)
	}
	return h, nil
}

func (npyDecoder) Probe(r io.ReadSeeker) (Info, error) {
	h, err := readNPYHeader(r)
	if err != nil {
		return Info{}, err
	}
	return h.info, nil
}

func (npyDecoder) Decode(ctx context.Context, r io.ReadSeeker, o DecodeOptions) (*Buffer, Info, error) {
	h, err := readNPYHeader(r)
	if err != nil {
		return nil, Info{}, err
	}
	win, err := window(o, h.info.W, h.info.H)
	if err != nil {
		return nil, Info{}, err
	}
	w, ht, C := win.Dx(), win.Dy(), h.cDim
	out := New(w, ht, C, 1)
	n := w * ht
	pix := C * h.itemSize
	row := make([]byte, w*pix)
	for y := 0; y < ht; y++ {
		if y%256 == 0 && ctx.Err() != nil {
			return nil, Info{}, ctx.Err()
		}
		off := h.data + (int64(win.Min.Y+y)*int64(h.info.W)+int64(win.Min.X))*int64(pix)
		if _, err := r.Seek(off, io.SeekStart); err != nil {
			return nil, Info{}, errors.Wrap(err, "seek")
		}
		if _, err := io.ReadFull(r, row); err != nil {
			return nil, Info{}, formatErr("truncated .npy data at row %d", win.Min.Y+y)
		}
		for x := 0; x < w; x++ {
			for c := 0; c < C; c++ {
				s := row[x*pix+c*h.itemSize:]
				var v float32
				switch h.itemSize {
				case 1:
					v = float32(s[0]) / 255
				case 2:
					v = float32(h.order.Uint16(s)) / 65535
				case 4:
					v = math.Float32frombits(h.order.Uint32(s))
				default:
					v = float32(math.Float64frombits(h.order.Uint64(s)))
				}
				out.Pix[c*n+y*w+x] = v
			}
		}
	}
	return out, h.info, nil
}
