package testimage

import (
	"bytes"
	"io"
	"math/rand"
	"testing"

	"golang.org/x/image/tiff/lzw"
)

func TestLZWEncoderRoundTrips(t *testing.T) {
	rng := rand.New(rand.NewSource(1)) // #nosec G404 -- deterministic test data
	for _, n := range []int{0, 1, 2, 300, 5000, 70000, 300000} {
		for _, alphabet := range []int{2, 16, 256} {
			data := make([]byte, n)
			for i := range data {
				data[i] = byte(rng.Intn(alphabet))
			}
			r := lzw.NewReader(bytes.NewReader(lzwEncode(data)), lzw.MSB, 8)
			got, err := io.ReadAll(r)
			if err != nil || !bytes.Equal(got, data) {
				t.Fatalf("n=%d alphabet=%d: err %v, %d bytes back", n, alphabet, err, len(got))
			}
		}
	}
}

func TestPackBitsRoundTrip(t *testing.T) {
	data := []byte{1, 1, 1, 1, 2, 3, 4, 4, 5, 5, 5, 5, 5, 5, 6}
	data = append(data, bytes.Repeat([]byte{9}, 300)...)
	enc := packBits(data)
	// decode by hand (the reader's unpackBits is in pkg/cv/buffer)
	var out []byte
	for i := 0; i < len(enc); {
		n := int(int8(enc[i]))
		i++
		if n >= 0 {
			out = append(out, enc[i:i+n+1]...)
			i += n + 1
		} else {
			out = append(out, bytes.Repeat([]byte{enc[i]}, 1-n)...)
			i++
		}
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("packbits round trip: %v", out)
	}
}
