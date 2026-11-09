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

// WSpace and WEncoding are the build-time space warnings.
const (
	WSpace    = "SPACE"    // a lifted level comes from another space
	WEncoding = "ENCODING" // images of different encodings mixed in encoded space
)

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
	return op.Space.Default
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
	return b.Intern(op, recv.ID, raw, nil)
}

// resolveSpace stores the op's working space in args (so the hash and the
// cache signature include it), replaces luminance inputs by toGray of linear
// light, replaces Image inputs in another space by convert nodes, and returns
// the new inputs with the SPACE and ENCODING warnings.
func (b *Builder) resolveSpace(op *OpDef, args Args, inputs []*Node) ([]*Node, []Warning, error) {
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
	if s == buffer.SpaceEncoded {
		if w, ok := encodingWarning(op, inputs); ok {
			ws = append(ws, w)
		}
	}
	ws = append(ws, b.spaceWarnings(op, args, out, s)...)
	return out, ws, nil
}

// encodingWarning is ENCODING when an op in encoded space reads images of
// different encodings: their encoded scales differ.
func encodingWarning(op *OpDef, inputs []*Node) (Warning, bool) {
	byEnc := map[buffer.Encoding]string{}
	var encs []buffer.Encoding
	for _, in := range inputs {
		if in.Kind != KindImage || in.Enc == buffer.EncNone {
			continue
		}
		if _, ok := byEnc[in.Enc]; !ok {
			byEnc[in.Enc] = sourceLabel(in)
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
		op.Name, strings.Join(parts, " and "), op.Name)}, true
}

// sourceLabel names the image a node was computed from: the label of the
// source at the end of its receiver chain.
func sourceLabel(n *Node) string {
	return Label(n)
}

// spaceWarnings are SPACE warnings for lifted Level arguments whose value
// node measured another space than the op's (a percentile survives a
// conversion, a mean does not, so the number is not converted).
func (b *Builder) spaceWarnings(op *OpDef, args Args, inputs []*Node, s buffer.Space) []Warning {
	var ws []Warning
	for i := range op.Args {
		spec := &op.Args[i]
		if spec.Unit != Level {
			continue
		}
		for _, ref := range inputRefs(args.vals[spec.Name]) {
			if ref.Index >= len(inputs) {
				continue
			}
			v := inputs[ref.Index]
			if v.Space == buffer.SpaceNone || v.Space == s || buffer.Identity(v.Enc, v.Space, s) {
				continue
			}
			root := b.measuringOp(v)
			ws = append(ws, Warning{Code: WSpace, Message: fmt.Sprintf(
				"%s.%s comes from %s() in %s space, but %s runs in %s space here. fix: img.%s({ space: %q })",
				op.Name, spec.Name, root, v.Space, op.Name, s, root, s.String())})
		}
	}
	return ws
}

// measuringOp is the name of the op that measured a Value node: the root
// of its chain of field, pick and arithmetic nodes.
func (b *Builder) measuringOp(n *Node) string {
	for n.Op.On == KindValue && len(n.Inputs) > 0 {
		in, ok := b.Node(n.Inputs[0])
		if !ok {
			break
		}
		n = in
	}
	return n.Op.Name
}

func inputRefs(v any) []InputRef {
	switch x := v.(type) {
	case InputRef:
		return []InputRef{x}
	case []any:
		var out []InputRef
		for _, e := range x {
			out = append(out, inputRefs(e)...)
		}
		return out
	case map[string]any:
		var out []InputRef
		for _, e := range x {
			out = append(out, inputRefs(e)...)
		}
		return out
	}
	return nil
}

// nodeStatics computes a node's encoding and space from (op, args, inputs)
// alone, so a hash hit has exactly the statics a new node would get.
func nodeStatics(op *OpDef, args Args, inputs []*Node) (buffer.Encoding, buffer.Space) {
	if op.Expand != nil && len(inputs) > 0 { // a macro node: its expansion's output
		last := inputs[len(inputs)-1]
		return last.Enc, last.Space
	}
	s := OpSpace(op, args)
	switch op.Returns {
	case KindImage:
		r := op.Result
		if r == nil {
			r = KeepEnc
		}
		return r.Enc(args, inputs), r.Space(args, s, inputs)
	case KindValue:
		if in := firstImage(inputs); in != nil {
			return in.Enc, s
		}
		if s == buffer.SpaceNone && len(inputs) > 0 && (inputs[0].Kind == KindValue || inputs[0].Kind == KindParam) {
			return inputs[0].Enc, inputs[0].Space // field, pick, arithmetic: the receiver's
		}
		return buffer.EncNone, s
	case KindMask, KindRegions, KindParam, KindNone:
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
