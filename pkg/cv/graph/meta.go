package graph

import "github.com/go-go-golems/goja-cv/pkg/cv/buffer"

// Source metadata (GCV-004 §7.12): what Go knows about a source node that
// the agent did not write travels as one typed value in Args (read with
// Meta), not as hidden arguments. It is hashed with the canonical arguments,
// so the node changes when the file changes.

// SourceMeta is what Go knows about an opened file.
type SourceMeta struct {
	Path  string      `json:"path"`  // the absolute path (GCV-004 Phase 6 replaces it by a workspace location)
	Stamp string      `json:"stamp"` // size and modification time
	Info  buffer.Info `json:"info"`  // the header: size, format, bits, channels, sample, origin, notes
	// FileEncoding is the encoding the file declares (Info.Encoding, which
	// buffer.Info does not serialize).
	FileEncoding buffer.Encoding `json:"fileEncoding"`
}

// PlaceholderMeta gives a builder lambda's placeholder the statics of the
// receiver it stands for.
type PlaceholderMeta struct {
	For      NodeID          `json:"for"`
	Frame    Frame           `json:"frame"`
	Enc      buffer.Encoding `json:"enc"`
	Space    buffer.Space    `json:"space"`
	Channels int             `json:"channels"`
}
