package graph

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"golang.org/x/sync/singleflight"
)

// Source is a decoded image file (or a window of one) with its preview
// pyramid of linear values.
type Source struct {
	Path string
	Info buffer.Info
	Pyr  *buffer.Pyramid
}

// SourceKey identifies one decoded source: a file version (path and stamp),
// a window of it (nil: the whole file) and the encoding it is read with.
type SourceKey struct {
	Path, Stamp string
	Box         *[4]int // [x, y, w, h] in file px
	Enc         buffer.Encoding
}

func (k SourceKey) String() string {
	s := k.Path + "\x00" + k.Stamp + "\x00" + k.Enc.String()
	if k.Box != nil {
		s += fmt.Sprintf("\x00%v", *k.Box)
	}
	return s
}

// SourceStore loads each source once (per key) and keeps its pyramid pinned;
// pyramids are not part of the byte-capped cache.
type SourceStore struct {
	mu      sync.Mutex
	m       map[string]*Source
	flight  singleflight.Group
	minLong int
}

// NewSourceStore returns an empty store whose pyramids stop at minLong px.
func NewSourceStore(minLong int) *SourceStore {
	return &SourceStore{m: map[string]*Source{}, minLong: minLong}
}

// Load returns the decoded source for k: the decoder's encoded values
// converted to linear values for encoding k.Enc (exact 8/16-bit tables for
// integer files), with a pyramid that averages linear values.
func (s *SourceStore) Load(ctx context.Context, k SourceKey) (*Source, error) {
	key := k.String()
	s.mu.Lock()
	src, ok := s.m[key]
	s.mu.Unlock()
	if ok {
		return src, nil
	}
	v, err, _ := s.flight.Do(key, func() (any, error) {
		var o buffer.DecodeOptions
		if k.Box != nil {
			o.Box = buffer.RectXYWH(k.Box[0], k.Box[1], k.Box[2], k.Box[3])
		}
		enc, info, err := buffer.DecodeFile(ctx, k.Path, o)
		if err != nil {
			return nil, err
		}
		lin, err := buffer.Linearize(ctx, enc, info, k.Enc)
		if err != nil {
			return nil, err
		}
		pyr, err := buffer.NewPyramid(ctx, lin, s.minLong)
		if err != nil {
			return nil, err
		}
		src := &Source{Path: k.Path, Info: info, Pyr: pyr}
		s.mu.Lock()
		s.m[key] = src
		s.mu.Unlock()
		return src, nil
	})
	if err != nil {
		if fe, ok := buffer.IsFormatError(err); ok {
			return nil, Errorf(EFormat, "cv.open(%q): %s.", filepath.Base(k.Path), fe.Feature).WithHint("Supported: " + buffer.SupportedFormats + ".")
		}
		return nil, err
	}
	return v.(*Source), nil
}

// Bytes is the memory held by all loaded pyramids.
func (s *SourceStore) Bytes() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int64
	for _, src := range s.m {
		n += src.Pyr.Bytes()
	}
	return n
}
