package graph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
)

// Working spaces in the graph (GCV-001 §7.5). Every op that reads pixel
// values has a working space, resolved when its node is built: the call's
// { space } or the op's default. The builder feeds the op inputs in that
// space by inserting convert nodes (hash-consed, so shared and cached), and
// feeds luminance inputs toGray(rec709) of linear light first. Kernels never
// convert. internNode computes every node's static encoding and space from
// (op, args, inputs) alone.

// Names of the ops the builder inserts.
const (
	ConvertOp = "convert"
	ToGrayOp  = "toGray"
)

// WEncoding is the build-time space warning: images of different encodings
// mixed in encoded space.
const WEncoding = "ENCODING"

// OpSpace is the working space of a node of op with canonical args: the
// resolved { space } option, the fixed space, or SpaceNone.
func OpSpace(op *OpDef, args Args) buffer.Space {
	if op.Space.Default == buffer.SpaceNone {
		return buffer.SpaceNone
	}
	if !op.Space.Fixed {
		if v, ok := args.vals[SpaceArg].(string); ok {
			if s, ok := buffer.ParseSpace(v); ok {
				return s
			}
		}
	}
	return op.DefaultSpace(args)
}

// internOp interns op name on recv with raw arguments through Intern
// (binding, space resolution); it may call Internal ops.
func (b *Builder) internOp(name string, recv *Node, raw ...any) (*Node, []Warning, error) {
	if b.reg == nil {
		return nil, nil, Errorf(EInternal, "the builder has no registry to insert %s", name)
	}
	op, ok := b.reg.Lookup(recv.Kind, name)
	if !ok {
		return nil, nil, Errorf(EInternal, "no %s op is registered for %s", name, A(string(recv.Kind)))
	}
	r, err := b.call(op, recv.ID, raw, nil, true)
	if err != nil {
		return nil, nil, err
	}
	if len(r.Nodes) != 1 {
		return nil, nil, Errorf(EInternal, "%s returns %d nodes", name, len(r.Nodes))
	}
	return r.Nodes[0], r.Warnings, nil
}

// resolveSpace stores the op's working space in args (so the hash and the
// cache signature include it), replaces luminance inputs by toGray of linear
// light, replaces Image inputs in another space by convert nodes, and returns
// the new inputs with the ENCODING warning.
func (b *Builder) resolveSpace(op *OpDef, args Args) ([]*Node, []Warning, error) {
	inputs := args.ins
	s := OpSpace(op, args)
	if op.Space.Default != buffer.SpaceNone && !op.Space.Fixed {
		args.vals[SpaceArg] = s.String()
	}
	out := append([]*Node(nil), inputs...)
	var ws []Warning
	if op.Gray != nil {
		for _, i := range op.Gray(args) {
			if i < 0 || i >= len(out) || out[i].Kind != KindImage {
				continue
			}
			g, w, err := b.internOp(ToGrayOp, out[i], "rec709", map[string]any{SpaceArg: buffer.SpaceLinear.String()})
			if err != nil {
				return nil, nil, err
			}
			out[i] = g
			ws = append(ws, w...)
		}
	}
	if s == buffer.SpaceNone {
		return out, ws, nil
	}
	for i, in := range out {
		if in.Kind != KindImage || in.Space == s || buffer.Identity(in.Enc, in.Space, s) {
			continue
		}
		c, w, err := b.internOp(ConvertOp, in, s.String())
		if err != nil {
			return nil, nil, err
		}
		out[i] = c
		ws = append(ws, w...)
	}
	if s == buffer.SpaceEncoded && op.Result != DisplayEnc { // a layout of display values (compare) mixes encodings on purpose
		if w, ok := b.encodingWarning(op, inputs); ok {
			ws = append(ws, w)
		}
	}
	return out, ws, nil
}

// encodingWarning is ENCODING when an op in encoded space reads images of
// different encodings: their encoded scales differ.
func (b *Builder) encodingWarning(op *OpDef, inputs []*Node) (Warning, bool) {
	byEnc := map[buffer.Encoding]string{}
	var encs []buffer.Encoding
	for _, in := range inputs {
		if in.Kind != KindImage || in.Enc == buffer.EncNone {
			continue
		}
		if _, ok := byEnc[in.Enc]; !ok {
			byEnc[in.Enc] = b.sourceLabel(in)
			encs = append(encs, in.Enc)
		}
	}
	if len(encs) < 2 {
		return Warning{}, false
	}
	sort.Slice(encs, func(i, j int) bool { return encs[i] < encs[j] })
	parts := make([]string, len(encs))
	for i, e := range encs {
		parts[i] = fmt.Sprintf("%s (%s)", byEnc[e], e)
	}
	return Warning{Code: WEncoding, Message: fmt.Sprintf("%s mixes %s in encoded space; their encoded scales differ. fix: %s({ space: \"linear\" })",
		op.Name, strings.Join(parts, " and "), callName(op))}, true
}

// sourceLabel names the image a node was computed from: the label of the
// source at the end of its receiver chain.
func (b *Builder) sourceLabel(n *Node) string {
	for n.HasReceiver() {
		in, ok := b.Node(n.Inputs[0])
		if !ok {
			break
		}
		n = in
	}
	return Label(n)
}

// nodeStatics computes a node's encoding and space from (op, args with its
// inputs) alone, so a hash hit has exactly the statics a new node would get.
func nodeStatics(op *OpDef, args Args) (buffer.Encoding, buffer.Space) {
	if out := macroOutput(op, args); out != nil { // a macro node: its expansion's output
		return out.Enc, out.Space
	}
	s := OpSpace(op, args)
	switch op.Returns {
	case KindImage:
		r := op.Result
		if r == nil {
			r = KeepEnc
		}
		return r.Enc(args), r.Space(args, s)
	case KindValue:
		if in := args.imageSource(); in != nil {
			return in.Enc, s
		}
		return buffer.EncNone, s
	case KindMask, KindRegions, KindNone:
	}
	return buffer.EncNone, buffer.SpaceNone
}

// Statics renders an Image node's statics for explain: "enc srgb · space linear".
func Statics(n *Node) string {
	if n.Kind != KindImage || n.Enc == buffer.EncNone {
		return ""
	}
	return fmt.Sprintf("enc %s · space %s", n.Enc, n.Space)
}
