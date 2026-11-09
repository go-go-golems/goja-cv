package graph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
)

// Spaces in the graph (GCV-004 §4.2, §7.4). The space is a property of the
// image: cv.open returns file units (encoded), as(s) converts, and every
// other op returns its result in its receiver's space. In an op with several
// image inputs the receiver decides: other image inputs are converted to its
// space by inserted as() nodes (hash-consed and cached); masks are read as
// stored. A Physical op (exposure, heal, inpaint, toGray rec709, toLab,
// encode) works in linear light: the builder converts its inputs to linear
// and its result back to the receiver's space. A Display op draws a picture
// from encoded values. Ops that reduce colour to one grey value read the one
// luminance (OpDef.Luma): toGray(receiver) in the receiver's space, an extra
// input. Kernels never convert. internNode computes every node's static
// encoding and space from (op, args, inputs) alone.

// Names of the ops the builder inserts.
const (
	AsOp         = "as"
	OpenLinearOp = "openLinear"
	ToGrayOp     = "toGray"
)

// lumaArg is the canonical argument that refers to the luminance input.
const lumaArg = "luma"

// WEncoding is the build-time space warning: images of different encodings
// mixed in encoded space.
const WEncoding = "ENCODING"

// rule is the op's space rule for these args (SpaceOf overrides Space).
func (op *OpDef) rule(args Args) SpaceRule {
	if op.SpaceOf != nil {
		return op.SpaceOf(args)
	}
	return op.Space
}

// workSpace is the space an op computes in: linear for a Physical op,
// encoded for a Display op, else its receiver's (for an op called on regions
// or a mask, its image argument's); SpaceNone without an image.
func workSpace(op *OpDef, args Args) buffer.Space {
	switch op.rule(args) {
	case Physical:
		return buffer.SpaceLinear
	case Display:
		return buffer.SpaceEncoded
	case Follows:
	}
	if in := args.imageSource(); in != nil {
		return in.Space
	}
	return buffer.SpaceNone
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

// as returns x in space s: x itself when the conversion is the identity;
// the input of x when x is a conversion from s (a conversion and its inverse
// cancel); the pyramid level itself (openLinear) for the linear view of an
// opened file, and the opened file for the encoded view of that; otherwise
// the as node. Every inserted or written as() goes
// through here.
func (b *Builder) as(x *Node, s buffer.Space) (*Node, error) {
	if x.Kind != KindImage || s == buffer.SpaceNone || x.Space == s || buffer.Identity(x.Enc, x.Space, s) {
		return x, nil
	}
	if x.Op.Name == AsOp && x.HasReceiver() {
		if in := x.Args.ins[0]; in.Space == s {
			return in, nil
		}
	}
	if x.Op.On == KindNone && x.Op.Name == OpenLinearOp && s == buffer.SpaceEncoded {
		op, ok := b.reg.Lookup(KindNone, "open")
		if !ok {
			return nil, Errorf(EInternal, "no open op is registered")
		}
		return b.internNode(op, Args{op: op, vals: x.Args.vals, meta: x.Args.meta}, nil), nil
	}
	if x.Op.On == KindNone && x.Op.Name == "open" && s == buffer.SpaceLinear {
		op, ok := b.reg.Lookup(KindNone, OpenLinearOp)
		if !ok {
			return nil, Errorf(EInternal, "no %s op is registered", OpenLinearOp)
		}
		return b.internNode(op, Args{op: op, vals: x.Args.vals, meta: x.Args.meta}, nil), nil
	}
	op, ok := b.reg.Lookup(KindImage, AsOp)
	if !ok {
		return nil, Errorf(EInternal, "no %s op is registered", AsOp)
	}
	return b.internNode(op, Args{op: op, vals: map[string]any{"to": s.String()}}, []*Node{x}), nil
}

// lumaSource is the image whose luminance an op with Luma reads: its
// receiver, or the argument LumaOf names (refine's guide).
func lumaSource(op *OpDef, args Args) *Node {
	if !op.Luma || (op.LumaWhen != nil && !op.LumaWhen(args)) {
		return nil
	}
	if op.LumaOf != "" {
		return args.Input(op.LumaOf)
	}
	if args.op.On == KindNone || len(args.ins) == 0 {
		return nil
	}
	return args.ins[0]
}

// resolveSpace adds the luminance input of a Luma op (toGray of its source
// with the op's gray weights, in the source's space) and converts the image
// inputs that are not in the op's work space (§7.4). It returns the new
// inputs and the ENCODING warning; args.vals gains the luma reference.
func (b *Builder) resolveSpace(op *OpDef, args Args) ([]*Node, []Warning, error) {
	out := append([]*Node(nil), args.ins...)
	var ws []Warning
	if src := lumaSource(op, args); src != nil && src.Kind == KindImage {
		gray := "rec709"
		if s := op.spec("gray"); s != nil && args.Has("gray") {
			gray = args.String("gray")
		}
		g := src // a grey image is its own luminance: no toGray node (§7.5)
		if src.Channels != 1 {
			var w []Warning
			var err error
			if g, w, err = b.internOp(ToGrayOp, src, gray); err != nil {
				return nil, nil, err
			}
			ws = append(ws, w...)
		}
		args.vals[lumaArg] = InputRef{Index: len(out)}
		out = append(out, g)
	}
	work := workSpace(op, Args{op: op, vals: args.vals, ins: out})
	masks := maskInputs(op, args)
	if len(out) > len(args.ins) {
		masks[len(args.ins)] = true // the luminance is in its source's space (refine: the guide's, not the mask's)
	}
	if work != buffer.SpaceNone {
		for i, in := range out {
			if in.Kind != KindImage || masks[i] {
				continue // regions; masks are weights, read as stored
			}
			c, err := b.as(in, work)
			if err != nil {
				return nil, nil, err
			}
			out[i] = c
		}
	}
	if work == buffer.SpaceEncoded && op.rule(args) != Display { // a layout of display values (compare) mixes encodings on purpose
		if w, ok := b.encodingWarning(op, args.ins, masks); ok {
			ws = append(ws, w)
		}
	}
	return out, ws, nil
}

// maskInputs are the input positions read as masks: arguments declared
// Mask (or Regions|Mask) and the receiver of a OneChannel op (§7.5). They
// are never converted between spaces.
func maskInputs(op *OpDef, args Args) map[int]bool {
	m := map[int]bool{}
	if op.OneChannel && op.On == KindImage {
		m[0] = true
	}
	for i := range op.Args {
		s := &op.Args[i]
		if !takesMask(s.Type) || !args.Has(s.Name) {
			continue
		}
		for _, r := range args.refs(s.Name) {
			m[r.Index] = true
		}
	}
	return m
}

// closeBracket converts the result of a Physical op back to its receiver's
// space (the bracket's closing conversion).
func (b *Builder) closeBracket(op *OpDef, args Args, n *Node, recv *Node) (*Node, error) {
	if op.rule(args) != Physical || n.Kind != KindImage || recv == nil || recv.Kind != KindImage {
		return n, nil
	}
	return b.as(n, recv.Space)
}

// encodingWarning is ENCODING when an op in encoded space reads images of
// different encodings: their encoded scales differ.
func (b *Builder) encodingWarning(op *OpDef, inputs []*Node, masks map[int]bool) (Warning, bool) {
	byEnc := map[buffer.Encoding]string{}
	var encs []buffer.Encoding
	for i, in := range inputs {
		if in.Kind != KindImage || in.Enc == buffer.EncNone || masks[i] {
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
	return Warning{Code: WEncoding, Message: fmt.Sprintf("%s mixes %s in encoded space; their encoded scales differ. fix: img.as(\"linear\").%s(…)",
		op.Name, strings.Join(parts, " and "), op.Name)}, true
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
	s := workSpace(op, args)
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
	case KindRegions, KindNone:
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
