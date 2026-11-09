package graph

import (
	"context"
	"sync"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"golang.org/x/sync/singleflight"
)

// Source is a decoded image file with its preview pyramid.
type Source struct {
	Path string
	Info buffer.Info
	Pyr  *buffer.Pyramid
}

// SourceStore loads each source file once (per path and file stamp) and
// keeps its pyramid pinned; pyramids are not part of the byte-capped cache.
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

// Load returns the decoded source for path; stamp distinguishes versions of
// the same file (size and modification time).
func (s *SourceStore) Load(ctx context.Context, path, stamp string) (*Source, error) {
	key := path + "\x00" + stamp
	s.mu.Lock()
	src, ok := s.m[key]
	s.mu.Unlock()
	if ok {
		return src, nil
	}
	v, err, _ := s.flight.Do(key, func() (any, error) {
		buf, info, err := buffer.DecodeFile(ctx, path)
		if err != nil {
			return nil, err
		}
		pyr, err := buffer.NewPyramid(ctx, buf, s.minLong)
		if err != nil {
			return nil, err
		}
		src := &Source{Path: path, Info: info, Pyr: pyr}
		s.mu.Lock()
		s.m[key] = src
		s.mu.Unlock()
		return src, nil
	})
	if err != nil {
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
