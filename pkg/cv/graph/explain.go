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
// other nodes): file (basename), format, bitDepth, channels in the file and
// the assumed colour space.
func SourceInfo(n *Node) (map[string]any, bool) {
	if n.Op.On != KindNone || n.Op.Name != "open" {
		return nil, false
	}
	info := map[string]any{"file": filepath.Base(n.Args.String("path")), "colorSpace": "sRGB (assumed; ICC profiles are not read)"}
	if n.Args.Has("format") {
		info["format"] = n.Args.String("format")
	}
	if n.Args.Has("bits") {
		info["bitDepth"] = n.Args.Raw("bits")
	}
	if n.Args.Has("channels") {
		info["fileChannels"] = n.Args.Raw("channels")
	}
	return info, true
}

// Label is the short name of a node used in paths: "open(scan.tif)",
// "P.slider(ev)", or the op name.
func Label(n *Node) string {
	if n.Op.On == KindNone {
		switch n.Op.Name {
		case "open":
			return "open(" + filepath.Base(n.Args.String("path")) + ")"
		case "param":
			return "param(" + n.Args.String("name") + ")"
		case PlaceholderOp:
			return "placeholder"
		}
	}
	if n.Op.On == KindValue && n.Op.Name == "field" {
		return "." + n.Args.String("name")
	}
	return n.Op.Name
}

// Describe renders a node as a call, e.g. "gaussian(r: 2 px → 0.5 at scale
// 0.25)". Arguments equal to their default are omitted; lifted arguments
// show the input node id.
func Describe(b *Builder, n *Node, scale float64, params map[string]any) string {
	if n.Op.On == KindNone {
		switch n.Op.Name {
		case "open":
			return "source " + filepath.Base(n.Args.String("path"))
		case "param":
			name := n.Args.String("name")
			v, _ := json.Marshal(params[name])
			return fmt.Sprintf("param %s = %s", name, v)
		case PlaceholderOp:
			return "placeholder (within builder)"
		}
	}
	if n.Op.On == KindValue && n.Op.Name == "field" {
		return "." + n.Args.String("name")
	}
	var parts []string
	for i := range n.Op.Args {
		s := &n.Op.Args[i]
		if s.Hidden || !n.Args.Has(s.Name) {
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
	root := e.newRun(scale)
	type row struct{ left, right string }
	var rows []row
	seen := map[NodeID]bool{}
	var walk func(run *evalRun, id NodeID, depth int)
	walk = func(run *evalRun, id NodeID, depth int) {
		n := e.node(id)
		indent := strings.Repeat("  ", depth)
		if seen[id] {
			rows = append(rows, row{fmt.Sprintf("%s#%d (see above)", indent, id), ""})
			return
		}
		seen[id] = true
		parentScale := run.scale
		run = run.at(run.scaleFor(n))
		right := string(n.Kind)
		if n.Kind == KindImage || n.Kind == KindMask {
			w, h := buffer.ScaledSize(n.DocW, n.DocH, run.scale)
			right += fmt.Sprintf(" %d×%d", w, h)
		}
		if !n.Op.NoCache && e.cache.Contains(run.sig(id)) {
			right += "  cached"
		}
		if run.scale != parentScale {
			right += fmt.Sprintf("  (analysed at scale %s)", fmtNum(run.scale))
		}
		rows = append(rows, row{fmt.Sprintf("%s#%d %s", indent, id, Describe(e.b, n, run.scale, run.params)), right})
		for _, in := range n.Inputs {
			walk(run, in, depth+1)
		}
	}
	walk(root, id, 0)
	width := 0
	for _, r := range rows {
		width = max(width, len([]rune(r.left)))
	}
	out := make([]string, len(rows))
	for i, r := range rows {
		if r.right == "" {
			out[i] = r.left
			continue
		}
		out[i] = r.left + strings.Repeat(" ", width-len([]rune(r.left))+2) + r.right
	}
	return out
}
