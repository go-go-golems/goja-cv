package graph

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
)

// SourceInfo is the file metadata of an open() source node (ok false for
// other nodes): file (basename), format, bitDepth, sample, channels in the
// file, encoding, frame and notes.
func SourceInfo(n *Node) (map[string]any, bool) {
	if n.Op.On != KindNone || (n.Op.Name != "open" && n.Op.Name != OpenLinearOp) {
		return nil, false
	}
	m := Meta[SourceMeta](n.Args)
	info := map[string]any{"file": filepath.Base(m.Name), "encoding": n.Enc.String(), "frame": n.Frame.Data(),
		"format": m.Info.Format, "bitDepth": float64(m.Info.Bits), "fileChannels": float64(m.Info.Channels)}
	if m.Info.Sample != "" {
		info["sample"] = m.Info.Sample
	}
	if len(m.Info.Notes) > 0 {
		notes := make([]any, len(m.Info.Notes))
		for i, s := range m.Info.Notes {
			notes[i] = s
		}
		info["notes"] = notes
	}
	return info, true
}

// Shown is the node an agent's call stands for: the physical op of a
// bracket's closing as() (img.exposure(1) is as(exposure(…), encoded)),
// else n itself.
func Shown(n *Node) *Node {
	if n.Op.Name == AsOp && len(n.Args.ins) > 0 && n.Args.ins[0].Op.rule(n.Args.ins[0].Args) == Physical {
		return n.Args.ins[0]
	}
	return n
}

// Label is the short name of a node used in paths: "open(scan.tif)" or
// the op name.
func Label(n *Node) string {
	n = Shown(n)
	if n.Op.On == KindNone {
		switch n.Op.Name {
		case "open", OpenLinearOp:
			return "open(" + filepath.Base(n.Args.String("path")) + ")"
		case PlaceholderOp:
			return "placeholder"
		}
	}
	return n.Op.Name
}

// ShortCall is the compact form of a call for captions and fixes: the op
// name with its non-default positional arguments ("gaussian(2)",
// "view(speck)", "crop[300,100,64,64]").
func ShortCall(op *OpDef, args Args) string {
	if op.Name == "crop" && op.Class == GeometricOp {
		if b, ok := literalList(args.vals["box"]); ok {
			return "crop[" + strings.Join(b, ",") + "]"
		}
	}
	var parts []string
	for i := range op.Args {
		s := &op.Args[i]
		if !s.Positional || !args.Has(s.Name) {
			continue
		}
		v := args.vals[s.Name]
		if s.Default != nil && reflect.DeepEqual(v, normalize(s.Default)) {
			continue
		}
		if l, ok := v.([]any); ok && strings.HasSuffix(s.Type, "[]") && isKindType(strings.TrimSuffix(s.Type, "[]")) {
			parts = append(parts, fmt.Sprintf("[%d %s]", len(l), strings.ToLower(strings.TrimSuffix(s.Type, "[]"))+"s"))
			continue
		}
		parts = append(parts, shortValue(v))
	}
	if len(parts) == 0 {
		return op.Name
	}
	return op.Name + "(" + strings.Join(parts, ", ") + ")"
}

func shortValue(v any) string {
	switch x := v.(type) {
	case float64:
		return fmtNum(x)
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case []any:
		if l, ok := literalList(x); ok {
			return "[" + strings.Join(l, ",") + "]"
		}
		for _, e := range x {
			switch e.(type) {
			case []any, map[string]any:
				return fmt.Sprintf("[%d items]", len(x))
			}
		}
		el := make([]string, len(x))
		for i, e := range x {
			el[i] = shortValue(e)
		}
		return "[" + strings.Join(el, ",") + "]"
	}
	return "…"
}

func literalList(v any) ([]string, bool) {
	l, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, len(l))
	for i, e := range l {
		f, ok := e.(float64)
		if !ok {
			return nil, false
		}
		out[i] = fmtNum(f)
	}
	return out, true
}

// Short is the compact form of a node in a caption chain: "open(scan.tif)",
// "param(ev)", ".p99", "crop[300,100,64,64]" (its frame), "view(speck)".
func Short(n *Node) string {
	switch {
	case n.Op.On == KindNone && (n.Op.Name == "open" || n.Op.Name == OpenLinearOp):
		return "open(" + shortFile(filepath.Base(n.Args.String("path"))) + ")"
	case n.Op.On == KindNone && n.Op.Bespoke:
		return Label(n)
	case n.Op.Name == "crop" && n.Op.Class == GeometricOp:
		return fmt.Sprintf("crop[%d,%d,%d,%d]", n.Frame.X, n.Frame.Y, n.Frame.W, n.Frame.H)
	}
	return ShortCall(n.Op, n.Args)
}

// shortFile keeps long file names readable in a caption: "1d955ef4b2…_1536_1536.tif".
func shortFile(name string) string {
	r := []rune(name)
	if len(r) <= 32 {
		return name
	}
	return string(r[:10]) + "…" + string(r[len(r)-12:])
}

// maxChain is the longest caption chain before it is shortened.
const maxChain = 120

// Chain is the receiver chain of node id in compact form, from the source:
// "open(t.tif) → view(speck) → crop[300,100,64,64]". Macro nodes are one
// step (their expansion is in explain()); inserted conversions are skipped.
// A chain longer than 120 characters keeps the source and the last three
// steps.
func (e *Evaluator) Chain(id NodeID) string {
	var rev []string
	for n := e.node(id); n != nil; {
		if (!n.Op.Internal || n.Op.Name == OpenLinearOp) && n.Op.Name != AsOp {
			rev = append(rev, Short(n))
		}
		if !n.HasReceiver() {
			break
		}
		n = e.node(n.Inputs[0])
	}
	steps := make([]string, len(rev))
	for i, s := range rev {
		steps[len(rev)-1-i] = s
	}
	out := strings.Join(steps, " → ")
	if len([]rune(out)) > maxChain && len(steps) > 4 {
		out = steps[0] + " → … → " + strings.Join(steps[len(steps)-3:], " → ")
	}
	return out
}

// Describe renders a node as a call, e.g. "gaussian(sigma: 2 px → 0.5 at scale
// 0.25)". Arguments equal to their default are omitted; lifted arguments
// show the input node id.
func Describe(b *Builder, n *Node, scale float64) string {
	if n.Op.On == KindNone {
		switch n.Op.Name {
		case "open":
			return "source " + filepath.Base(n.Args.String("path"))
		case OpenLinearOp:
			return "source " + filepath.Base(n.Args.String("path")) + " (linear)"
		case PlaceholderOp:
			return "placeholder (within builder)"
		}
	}
	var parts []string
	for i := range n.Op.Args {
		s := &n.Op.Args[i]
		if !n.Args.Has(s.Name) {
			continue
		}
		v := n.Args.Raw(s.Name)
		if s.Default != nil && reflect.DeepEqual(v, normalize(s.Default)) {
			continue
		}
		parts = append(parts, s.Name+": "+describeArg(b, n, s, v, scale))
	}
	return n.Op.Name + "(" + strings.Join(parts, ", ") + ")"
}

func describeArg(b *Builder, n *Node, s *ArgSpec, v any, scale float64) string {
	switch x := v.(type) {
	case InputRef:
		return fmt.Sprintf("#%d", n.Inputs[x.Index])
	case float64:
		if s.Unit == DocPx2 {
			out := fmtNum(x) + " px²"
			if scale < 1 {
				out += fmt.Sprintf(" → %s at scale %s", fmtNum(x*scale*scale), fmtNum(scale))
			}
			return out
		}
		if s.Unit == DocPx {
			out := fmtNum(x) + " px"
			if scale < 1 {
				out += fmt.Sprintf(" → %s at scale %s", fmtNum(x*scale), fmtNum(scale))
			}
			return out
		}
		return fmtNum(x)
	case string:
		return strconv.Quote(x)
	case []any:
		el := make([]string, len(x))
		for i, e := range x {
			el[i] = describeArg(b, n, &ArgSpec{}, e, scale)
		}
		return "[" + strings.Join(el, ", ") + "]"
	case map[string]any:
		var el []string
		for _, k := range sortedKeys(x) {
			el = append(el, k+": "+describeArg(b, n, &ArgSpec{}, x[k], scale))
		}
		return "{ " + strings.Join(el, ", ") + " }"
	}
	js, _ := json.Marshal(v)
	return string(js)
}

// Explain renders the graph under node id as indented lines with the
// result kind, its size at scale, and whether it is cached at that scale.
// It computes no pixels. Nodes reached twice are printed once.
func (e *Evaluator) Explain(id NodeID, scale float64) []string {
	root := e.newRun(scale, nil)
	type row struct{ left, right, statics string }
	var rows []row
	seen := map[NodeID]bool{}
	var walk func(run *evalRun, id NodeID, depth int)
	walk = func(run *evalRun, id NodeID, depth int) {
		n := e.node(id)
		indent := strings.Repeat("  ", depth)
		if seen[id] {
			rows = append(rows, row{left: fmt.Sprintf("%s#%d (see above)", indent, id)})
			return
		}
		seen[id] = true
		parentScale := run.scale
		run = run.at(run.scaleFor(n))
		right := string(n.Kind)
		if n.Kind == KindImage {
			w, h := buffer.ScaledSize(n.Frame.W, n.Frame.H, run.scale)
			right += fmt.Sprintf(" %d×%d", w, h)
		}
		if e.cache.Contains(run.sig(id)) {
			right += "  cached"
		}
		if run.scale != parentScale {
			right += fmt.Sprintf("  (analysed at scale %s)", fmtNum(run.scale))
		}
		rows = append(rows, row{fmt.Sprintf("%s#%d %s", indent, id, Describe(e.b, n, run.scale)), right, Statics(n)})
		if n.Op.Expand != nil { // a macro: its expansion below it
			walk(run, n.Inputs[len(n.Inputs)-1], depth+1)
			return
		}
		for _, in := range n.Inputs {
			walk(run, in, depth+1)
		}
	}
	walk(root, id, 0)
	width, rwidth := 0, 0
	for _, r := range rows {
		width = max(width, len([]rune(r.left)))
		if r.statics != "" {
			rwidth = max(rwidth, len([]rune(r.right)))
		}
	}
	out := make([]string, len(rows))
	for i, r := range rows {
		if r.right == "" {
			out[i] = r.left
			continue
		}
		out[i] = r.left + strings.Repeat(" ", width-len([]rune(r.left))+2) + r.right
		if r.statics != "" {
			out[i] += strings.Repeat(" ", rwidth-len([]rune(r.right))+3) + r.statics
		}
	}
	return out
}
