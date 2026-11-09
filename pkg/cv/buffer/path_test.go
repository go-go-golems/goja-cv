package buffer_test

import (
	"context"
	"os"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
)

// probePath and decodePath open a file by path for the decoders, which take
// open files.
func probePath(p string) (buffer.Info, error) {
	f, err := os.Open(p) // #nosec G304 -- test files
	if err != nil {
		return buffer.Info{}, err
	}
	defer func() { _ = f.Close() }()
	return buffer.ProbeFile(f)
}

func decodePath(ctx context.Context, p string, o buffer.DecodeOptions) (*buffer.Buffer, buffer.Info, error) {
	f, err := os.Open(p) // #nosec G304 -- test files
	if err != nil {
		return nil, buffer.Info{}, err
	}
	defer func() { _ = f.Close() }()
	return buffer.DecodeFile(ctx, f, o)
}
