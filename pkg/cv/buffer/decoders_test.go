package buffer_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"image"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/internal/testimage"
	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
)

// planted returns the planted scan's linear float buffer: the values every
// fixture stores (as linear-encoded data).
func planted() *buffer.Buffer {
	b, _ := testimage.PlantedScan()
	return b
}

type fixture struct {
	name     string
	file     string
	write    func(path string, b *buffer.Buffer) error
	tol      float64 // per sample, encoded units
	format   string
	bits     int
	sample   string
	enc      buffer.Encoding
	note     string
	channels int
}

func fixtures() []fixture {
	tiff := func(o testimage.TIFFOptions) func(string, *buffer.Buffer) error {
		return func(p string, b *buffer.Buffer) error { return testimage.WriteTIFF(p, b, o) }
	}
	npy := func(dtype string) func(string, *buffer.Buffer) error {
		return func(p string, b *buffer.Buffer) error { return testimage.WriteNPY(p, b, dtype) }
	}
	lin := buffer.FormatMetadata(buffer.EncLinear, nil)
	code16, code8 := 0.5/65535+1e-7, 0.5/255+1e-7
	return []fixture{
		{"tiff16-linear", "a.tif", tiff(testimage.TIFFOptions{Bits: 16, Description: lin}), code16, "tiff", 16, "uint", buffer.EncLinear, "", 3},
		{"tiff16-srgb-default", "b.tif", tiff(testimage.TIFFOptions{Bits: 16}), code16, "tiff", 16, "uint", buffer.EncSRGB, "", 3},
		{"tiff16-bigendian", "c.tif", tiff(testimage.TIFFOptions{Bits: 16, BigEndian: true, RowsPerStrip: 7}), code16, "tiff", 16, "uint", buffer.EncSRGB, "", 3},
		{"tiff-float32", "d.tif", tiff(testimage.TIFFOptions{Bits: 32}), 0, "tiff", 32, "float", buffer.EncLinear, "", 3},
		{"tiff16-lzw-predictor", "e.tif", tiff(testimage.TIFFOptions{Bits: 16, Compression: "lzw", Predictor: true}), code16, "tiff", 16, "uint", buffer.EncSRGB, "LZW", 3},
		{"tiff8-lzw", "f.tif", tiff(testimage.TIFFOptions{Bits: 8, Compression: "lzw", RowsPerStrip: 192}), code8, "tiff", 8, "uint", buffer.EncSRGB, "LZW", 3},
		{"tiff16-deflate-predictor", "g.tif", tiff(testimage.TIFFOptions{Bits: 16, Compression: "deflate", Predictor: true, RowsPerStrip: 5}), code16, "tiff", 16, "uint", buffer.EncSRGB, "Deflate", 3},
		{"tiff8-packbits", "h.tif", tiff(testimage.TIFFOptions{Bits: 8, Compression: "packbits"}), code8, "tiff", 8, "uint", buffer.EncSRGB, "PackBits", 3},
		{"tiff-float32-lzw", "i.tif", tiff(testimage.TIFFOptions{Bits: 32, Compression: "lzw"}), 0, "tiff", 32, "float", buffer.EncLinear, "LZW", 3},
		{"npy-f4", "a.npy", npy("<f4"), 0, "npy", 32, "float", buffer.EncLinear, "", 3},
		{"npy-f4-big", "b.npy", npy(">f4"), 0, "npy", 32, "float", buffer.EncLinear, "", 3},
		{"npy-f8", "c.npy", npy("<f8"), 0, "npy", 64, "float", buffer.EncLinear, "", 3},
		{"npy-u2", "d.npy", npy("<u2"), code16, "npy", 16, "uint", buffer.EncLinear, "", 3},
		{"npy-u1", "e.npy", npy("|u1"), code8, "npy", 8, "uint", buffer.EncLinear, "", 3},
		{"dng", "a.dng", func(p string, b *buffer.Buffer) error { return testimage.WriteDNG(p, b, testimage.DNGOptions{}) },
			code16, "dng", 16, "uint", buffer.EncLinear, "camera RGB primaries (DNG)", 3},
		{"dng-levels", "b.dng", func(p string, b *buffer.Buffer) error {
			return testimage.WriteDNG(p, b, testimage.DNGOptions{BlackLevel: 1024, WhiteLevel: 61440})
		}, 0.5/(61440-1024) + 1e-6, "dng", 16, "uint", buffer.EncLinear, "BlackLevel/WhiteLevel applied", 3},
	}
}

func writeFixture(t *testing.T, dir string, f fixture, b *buffer.Buffer) string {
	t.Helper()
	p := filepath.Join(dir, f.file)
	if err := f.write(p, b); err != nil {
		t.Fatal(err)
	}
	return p
}

func compare(t *testing.T, name string, got, want *buffer.Buffer, box image.Rectangle, tol float64) {
	t.Helper()
	if got.W != box.Dx() || got.H != box.Dy() || got.C != want.C {
		t.Fatalf("%s: got %v, want %d×%d×%d", name, got, box.Dx(), box.Dy(), want.C)
	}
	worst := 0.0
	for c := 0; c < want.C; c++ {
		gp, wp := got.Plane(c), want.Plane(c)
		for y := 0; y < got.H; y++ {
			for x := 0; x < got.W; x++ {
				d := math.Abs(float64(gp[y*got.W+x]) - float64(wp[(box.Min.Y+y)*want.W+box.Min.X+x]))
				worst = math.Max(worst, d)
			}
		}
	}
	if worst > tol {
		t.Fatalf("%s: largest difference %g > %g", name, worst, tol)
	}
}

func TestDecodersMatchTheirGeneratingBuffer(t *testing.T) {
	ctx := context.Background()
	src := planted()
	dir := t.TempDir()
	full := image.Rect(0, 0, src.W, src.H)
	box := image.Rect(37, 21, 37+100, 21+80)
	for _, f := range fixtures() {
		t.Run(f.name, func(t *testing.T) {
			p := writeFixture(t, dir, f, src)
			info, err := buffer.ProbeFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if info.Format != f.format || info.W != src.W || info.H != src.H || info.Bits != f.bits || info.Sample != f.sample ||
				info.Encoding != f.enc || info.Channels != f.channels {
				t.Fatalf("probe %+v (encoding %s)", info, info.Encoding)
			}
			if f.note != "" && !strings.Contains(strings.Join(info.Notes, "; "), f.note) {
				t.Fatalf("notes %v lack %q", info.Notes, f.note)
			}
			got, dinfo, err := buffer.DecodeFile(ctx, p, buffer.DecodeOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if dinfo.Encoding != info.Encoding || dinfo.Format != info.Format {
				t.Fatalf("decode info %+v differs from probe %+v", dinfo, info)
			}
			compare(t, "full", got, src, full, f.tol)
			win, _, err := buffer.DecodeFile(ctx, p, buffer.DecodeOptions{Box: &box})
			if err != nil {
				t.Fatal(err)
			}
			compare(t, "window", win, got, image.Rect(box.Min.X, box.Min.Y, box.Max.X, box.Max.Y), 0)
			compare(t, "window vs source", win, src, box, f.tol)
		})
	}
}

func TestOneChannelFixtures(t *testing.T) {
	ctx := context.Background()
	src := buffer.New(20, 10, 1, 1)
	for i := range src.Pix {
		src.Pix[i] = float32(i) / float32(len(src.Pix))
	}
	dir := t.TempDir()
	for _, f := range []fixture{
		{name: "tiff16", file: "g.tif", write: func(p string, b *buffer.Buffer) error {
			return testimage.WriteTIFF(p, b, testimage.TIFFOptions{Bits: 16, Compression: "lzw", Predictor: true})
		}, tol: 0.5/65535 + 1e-7},
		{name: "npy", file: "g.npy", write: func(p string, b *buffer.Buffer) error { return testimage.WriteNPY(p, b, "<f4") }},
	} {
		p := writeFixture(t, dir, f, src)
		got, info, err := buffer.DecodeFile(ctx, p, buffer.DecodeOptions{})
		if err != nil || info.Channels != 1 {
			t.Fatalf("%s: %v %+v", f.name, err, info)
		}
		compare(t, f.name, got, src, image.Rect(0, 0, 20, 10), f.tol)
	}
}

func TestRecordedMetadata(t *testing.T) {
	src := buffer.New(8, 4, 3, 1)
	dir := t.TempDir()
	p := filepath.Join(dir, "w.tif")
	if err := testimage.WriteTIFF(p, src, testimage.TIFFOptions{Bits: 16, Description: "goja-cv encoding=linear origin=1536,1536"}); err != nil {
		t.Fatal(err)
	}
	info, err := buffer.ProbeFile(p)
	if err != nil || info.Encoding != buffer.EncLinear || info.Origin == nil || *info.Origin != [2]int{1536, 1536} {
		t.Fatalf("ImageDescription metadata: %+v %v", info, err)
	}
	if got := buffer.FormatMetadata(buffer.EncLinear, &[2]int{1536, 1536}); got != "goja-cv encoding=linear origin=1536,1536" {
		t.Fatalf("metadata string %q", got)
	}
	// other descriptions are ignored
	if err := testimage.WriteTIFF(p, src, testimage.TIFFOptions{Bits: 16, Description: "VueScan 9"}); err != nil {
		t.Fatal(err)
	}
	if info, _ := buffer.ProbeFile(p); info.Encoding != buffer.EncSRGB || info.Origin != nil {
		t.Fatalf("foreign description: %+v", info)
	}

	// PNG: gAMA 1.0 means linear (unless an sRGB chunk says otherwise); tEXt goja-cv carries both
	var pb bytes.Buffer
	if err := buffer.EncodePNG(context.Background(), &pb, src, 16); err != nil {
		t.Fatal(err)
	}
	gama := make([]byte, 4)
	binary.BigEndian.PutUint32(gama, 100000)
	for _, tc := range []struct {
		name   string
		chunks [][2]string
		enc    buffer.Encoding
		origin *[2]int
	}{
		{"plain", nil, buffer.EncSRGB, nil},
		{"gAMA 1.0", [][2]string{{"gAMA", string(gama)}}, buffer.EncLinear, nil},
		{"gAMA with sRGB", [][2]string{{"sRGB", "\x00"}, {"gAMA", string(gama)}}, buffer.EncSRGB, nil},
		{"tEXt", [][2]string{{"tEXt", "goja-cv\x00goja-cv encoding=linear origin=10,20"}}, buffer.EncLinear, &[2]int{10, 20}},
		{"other tEXt", [][2]string{{"tEXt", "Comment\x00goja-cv encoding=linear"}}, buffer.EncSRGB, nil},
	} {
		p := filepath.Join(dir, "m.png")
		if err := os.WriteFile(p, withChunks(pb.Bytes(), tc.chunks), 0o644); err != nil { // #nosec G306 -- test data
			t.Fatal(err)
		}
		info, err := buffer.ProbeFile(p)
		if err != nil || info.Encoding != tc.enc || (tc.origin == nil) != (info.Origin == nil) || (tc.origin != nil && *tc.origin != *info.Origin) {
			t.Fatalf("%s: %+v %v", tc.name, info, err)
		}
		if _, _, err := buffer.DecodeFile(context.Background(), p, buffer.DecodeOptions{}); err != nil {
			t.Fatalf("%s: decode: %v", tc.name, err)
		}
	}
}

// withChunks inserts chunks right after IHDR.
func withChunks(png []byte, chunks [][2]string) []byte {
	const ihdrEnd = 8 + 8 + 13 + 4
	out := append([]byte(nil), png[:ihdrEnd]...)
	for _, c := range chunks {
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], uint32(len(c[1]))) // #nosec G115 -- short chunks
		out = append(out, b[:]...)
		body := []byte(c[0] + c[1])
		out = append(out, body...)
		binary.BigEndian.PutUint32(b[:], crc32.ChecksumIEEE(body))
		out = append(out, b[:]...)
	}
	return append(out, png[ihdrEnd:]...)
}

func TestUnsupportedFeaturesAreFormatErrors(t *testing.T) {
	src := planted()
	dir := t.TempDir()
	p := filepath.Join(dir, "cfa.dng")
	if err := testimage.WriteDNG(p, src, testimage.DNGOptions{CFA: true}); err != nil {
		t.Fatal(err)
	}
	_, err := buffer.ProbeFile(p)
	if fe, ok := buffer.IsFormatError(err); !ok || !strings.Contains(fe.Feature, "CFA (mosaiced) raw data is not supported") {
		t.Fatalf("CFA: %v", err)
	}
	for name, data := range map[string][]byte{
		"bigtiff": []byte("II+\x00\x08\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"),
		"npy-fortran": []byte("\x93NUMPY\x01\x00\x46\x00{'descr': '<f4', 'fortran_order': True, 'shape': (2, 2), }                \n" +
			strings.Repeat("\x00", 16)),
		"npy-dtype":  []byte("\x93NUMPY\x01\x00\x46\x00{'descr': '<i4', 'fortran_order': False, 'shape': (2, 2), }               \n"),
		"text":       []byte("hello, world, this is not an image"),
		"tiff-trunc": []byte("II*\x00\x08\x00\x00\x00\x05\x00"),
	} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o644); err != nil { // #nosec G306 -- test data
			t.Fatal(err)
		}
		if _, err := buffer.ProbeFile(p); err == nil {
			t.Errorf("%s: no error", name)
		} else if _, ok := buffer.IsFormatError(err); !ok {
			t.Errorf("%s: not a format error: %v", name, err)
		}
	}
}

func TestLinearizeUsesExactTables(t *testing.T) {
	ctx := context.Background()
	b := buffer.New(256, 1, 1, 1)
	for i := range b.Pix {
		b.Pix[i] = float32(i) / 255
	}
	lin, err := buffer.Linearize(ctx, b, buffer.Info{Bits: 8, Sample: "uint"}, buffer.EncSRGB)
	if err != nil {
		t.Fatal(err)
	}
	for i := range lin.Pix {
		if lin.Pix[i] != buffer.Linear8(uint8(i)) {
			t.Fatalf("code %d: %g, table %g", i, lin.Pix[i], buffer.Linear8(uint8(i)))
		}
	}
	if same, _ := buffer.Linearize(ctx, b, buffer.Info{Bits: 8, Sample: "uint"}, buffer.EncLinear); same != b {
		t.Fatal("linear encoding must not convert")
	}
}

func FuzzTIFFProbe(f *testing.F) {
	src := planted()
	for _, o := range []testimage.TIFFOptions{{Bits: 16}, {Bits: 8, Compression: "lzw", Predictor: true}, {Bits: 32, Compression: "deflate"}} {
		small := buffer.New(5, 3, 3, 1)
		copy(small.Pix, src.Pix)
		if b, err := testimage.TIFFBytes(small, o); err == nil {
			f.Add(b)
		}
	}
	if b, err := testimage.DNGBytes(buffer.New(4, 4, 3, 1), testimage.DNGOptions{}); err == nil {
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		p := filepath.Join(t.TempDir(), "f.tif")
		if err := os.WriteFile(p, data, 0o644); err != nil { // #nosec G306 -- test data
			t.Fatal(err)
		}
		if _, err := buffer.ProbeFile(p); err != nil {
			return
		}
		_, _, _ = buffer.DecodeFile(context.Background(), p, buffer.DecodeOptions{}) // must not panic
	})
}

func FuzzNPYProbe(f *testing.F) {
	for _, dtype := range []string{"<f4", "<u2", "|u1"} {
		if b, err := testimage.NPYBytes(buffer.New(3, 2, 3, 1), dtype); err == nil {
			f.Add(b)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		p := filepath.Join(t.TempDir(), "f.npy")
		if err := os.WriteFile(p, data, 0o644); err != nil { // #nosec G306 -- test data
			t.Fatal(err)
		}
		if _, err := buffer.ProbeFile(p); err != nil {
			return
		}
		_, _, _ = buffer.DecodeFile(context.Background(), p, buffer.DecodeOptions{}) // must not panic
	})
}

// TestTIFFEmptyBitsPerSample: a BitsPerSample entry with count 0 is a format
// error, not a panic (review of PR #2).
func TestTIFFEmptyBitsPerSample(t *testing.T) {
	b, err := testimage.TIFFBytes(buffer.New(4, 3, 1, 1), testimage.TIFFOptions{Bits: 16})
	if err != nil {
		t.Fatal(err)
	}
	if string(b[:2]) != "II" {
		t.Fatalf("fixture byte order %q", b[:2])
	}
	ifd := int(binary.LittleEndian.Uint32(b[4:8]))
	n := int(binary.LittleEndian.Uint16(b[ifd : ifd+2]))
	patched := false
	for i := 0; i < n; i++ {
		e := ifd + 2 + 12*i
		if binary.LittleEndian.Uint16(b[e:e+2]) == 258 { // BitsPerSample
			binary.LittleEndian.PutUint32(b[e+4:e+8], 0)
			patched = true
		}
	}
	if !patched {
		t.Fatal("fixture has no BitsPerSample entry")
	}
	p := filepath.Join(t.TempDir(), "empty-bits.tif")
	if err := os.WriteFile(p, b, 0o644); err != nil { // #nosec G306 -- test data
		t.Fatal(err)
	}
	if _, err := buffer.ProbeFile(p); err == nil || !strings.Contains(err.Error(), "BitsPerSample") {
		t.Fatalf("want a BitsPerSample format error, got %v", err)
	}
}
