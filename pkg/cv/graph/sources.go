package graph

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"golang.org/x/sync/singleflight"
)

// Source is a decoded image file (or a window of one) with its preview
// pyramid of linear values.
type Source struct {
	Loc  Location
	Info buffer.Info
	Pyr  *buffer.Pyramid
}

// SourceKey identifies one decoded source: a file version (location and
// stamp), a window of it (nil: the whole file) and the encoding it is read
// with. Name is how errors name the file.
type SourceKey struct {
	Loc   Location
	Name  string
	Stamp string
	Box   *[4]int // [x, y, w, h] in file px
	Enc   buffer.Encoding
}

func (k SourceKey) String() string {
	s := fmt.Sprintf("%d\x00%s", k.Loc.Root, k.Loc.Rel) + "\x00" + k.Stamp + "\x00" + k.Enc.String()
	if k.Box != nil {
		s += fmt.Sprintf("\x00%v", *k.Box)
	}
	return s
}

// SourceStore loads each source once (per key) and keeps its pyramid pinned;
// pyramids are not part of the byte-capped cache. Files are read through
// the workspace.
type SourceStore struct {
	mu      sync.Mutex
	m       map[string]*Source
	flight  singleflight.Group
	minLong int
	ws      *Workspace
}

// NewSourceStore returns an empty store whose pyramids stop at minLong px,
// reading files in ws.
func NewSourceStore(minLong int, ws *Workspace) *SourceStore {
	return &SourceStore{m: map[string]*Source{}, minLong: minLong, ws: ws}
}

// Load returns the decoded source for k: the decoder's encoded values
// converted to linear values for encoding k.Enc (exact 8/16-bit tables for
// integer files), with a pyramid that averages linear values. A file that
// changed since cv.open (its stamp differs) is not decoded: E_EVAL, open it
// again. A pyramid already loaded stays pinned (decided in revision 3).
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
		if s.ws == nil {
			return nil, Errorf(EAccess, "%s: no workspace to read from", k.Name)
		}
		f, err := s.ws.OpenAt(k.Loc)
		if err != nil {
			return nil, Errorf(EEval, "cannot read %s: %v; open it again.", k.Name, unwrapPath(err)).WithFix(fmt.Sprintf("cv.open(%q)", k.Name))
		}
		defer func() { _ = f.Close() }() // read-only
		st, err := f.Stat()
		if err != nil || Stamp(st) != k.Stamp {
			return nil, Errorf(EEval, "%s changed since it was opened; open it again.", k.Name).
				WithHint("A node stands for the file as it was when cv.open read its header.").
				WithFix(fmt.Sprintf("img = cv.open(%q)", k.Name))
		}
		enc, info, err := buffer.DecodeFile(ctx, f, o)
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
		src := &Source{Loc: k.Loc, Info: info, Pyr: pyr}
		s.mu.Lock()
		s.m[key] = src
		s.mu.Unlock()
		return src, nil
	})
	if err != nil {
		if fe, ok := buffer.IsFormatError(err); ok {
			return nil, Errorf(EFormat, "cv.open(%q): %s.", k.Name, fe.Feature).WithHint("Supported: " + buffer.SupportedFormats + ".")
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
