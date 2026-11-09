package cvsession

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/go-go-golems/go-go-goja/pkg/resultcontent"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	cvmod "github.com/go-go-golems/goja-cv/pkg/gojamod/cv"
	"gopkg.in/yaml.v3"
)

// Rendering limits of result values.
const (
	maxDepth      = 8
	maxProperties = 2000
	maxArrayItems = 48
	maxStringLen  = 2000
	flowMaxItems  = 8
	flowMaxChars  = 100
)

var (
	plainObjectType = reflect.TypeOf(map[string]any{})
	plainArrayType  = reflect.TypeOf([]any{})
	promiseType     = reflect.TypeOf((*goja.Promise)(nil))
	timeType        = reflect.TypeOf(time.Time{})
)

// renderer turns a settled goja value into a YAML node tree, on the owner
// goroutine. It never reads "then", never runs proxy traps and never invokes
// getters. cv nodes render as one-line summaries and looks as their caption;
// the node ids it meets are collected for the UNUSED_NODE lint.
type renderer struct {
	vm        *goja.Runtime
	eng       *graph.Engine
	getDesc   goja.Callable
	providers map[resultcontent.Provider]int // other providers → content number
	props     int
	nodes     []graph.NodeID
	stack     map[*goja.Object]bool
}

func newRenderer(vm *goja.Runtime, eng *graph.Engine) *renderer {
	r := &renderer{vm: vm, eng: eng, providers: map[resultcontent.Provider]int{}, stack: map[*goja.Object]bool{}}
	if objectCtor, ok := vm.GlobalObject().Get("Object").(*goja.Object); ok {
		r.getDesc, _ = goja.AssertFunction(objectCtor.Get("getOwnPropertyDescriptor"))
	}
	return r
}

func scalar(s string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Value: s} }

// str is a scalar that always reads back as a string.
func str(s string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s} }

// FormatNumber renders numbers for agents: integers exactly, other values
// with 5 significant digits.
func FormatNumber(f float64) string {
	switch {
	case math.IsNaN(f):
		return ".nan"
	case math.IsInf(f, 1):
		return ".inf"
	case math.IsInf(f, -1):
		return "-.inf"
	case f == math.Trunc(f) && math.Abs(f) < 1e15:
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', 5, 64)
}

func (r *renderer) render(v goja.Value, depth int) *yaml.Node {
	if v == nil || goja.IsUndefined(v) {
		return scalar("undefined")
	}
	if goja.IsNull(v) {
		return scalar("null")
	}
	obj, ok := v.(*goja.Object)
	if !ok {
		return r.primitive(v)
	}
	if id, ok := cvmod.NodeOf(obj); ok {
		r.nodes = append(r.nodes, id)
		return str(cvmod.DescribeNode(r.eng, id) + " (lazy: await it, or return node.look())")
	}
	if l, ok := cvmod.LookOf(obj); ok {
		return str(l.String())
	}
	if p, ok := resultcontent.ProviderOf(obj); ok {
		desc := resultcontent.Describe(p)
		if n, ok := r.providers[p]; ok && reflect.TypeOf(p).Comparable() {
			return str(strings.TrimSpace(fmt.Sprintf("[content %d: %s]", n, desc)))
		}
		return str("[content: " + desc + "]")
	}
	if _, ok := goja.AssertFunction(obj); ok {
		return str("[Function]")
	}
	t := obj.ExportType()
	switch t {
	case promiseType:
		return str("[Promise] (not awaited: add await)")
	case timeType:
		if tm, ok := obj.Export().(time.Time); ok {
			return str(tm.UTC().Format(time.RFC3339))
		}
	}
	if obj.ClassName() == "Error" {
		return r.errorValue(obj)
	}
	if depth >= maxDepth {
		return str("…")
	}
	if r.stack[obj] {
		return str("[Circular]")
	}
	switch {
	case t == plainArrayType || obj.ClassName() == "Array":
		r.stack[obj] = true
		defer delete(r.stack, obj)
		return r.array(obj, depth)
	case t == plainObjectType || (t != nil && t.Kind() == reflect.Map):
		r.stack[obj] = true
		defer delete(r.stack, obj)
		return r.object(obj, depth)
	}
	return str("[" + obj.ClassName() + "]")
}

func (r *renderer) primitive(v goja.Value) *yaml.Node {
	switch x := v.Export().(type) {
	case bool:
		return scalar(strconv.FormatBool(x))
	case int64:
		return scalar(strconv.FormatInt(x, 10))
	case float64:
		return scalar(FormatNumber(x))
	case string:
		if len(x) > maxStringLen {
			return str(x[:maxStringLen] + fmt.Sprintf("… (%d more characters; return cv.json(x) for all)", len(x)-maxStringLen))
		}
		return str(x)
	}
	return str(v.String())
}

func (r *renderer) budget() bool {
	r.props++
	return r.props <= maxProperties
}

func (r *renderer) array(obj *goja.Object, depth int) *yaml.Node {
	n := &yaml.Node{Kind: yaml.SequenceNode}
	length := int(obj.Get("length").ToInteger())
	shown := min(length, maxArrayItems)
	for i := 0; i < shown; i++ {
		if !r.budget() {
			shown = i
			break
		}
		n.Content = append(n.Content, r.render(obj.Get(strconv.Itoa(i)), depth+1))
	}
	flow(n)
	if more := length - shown; more > 0 {
		comment := fmt.Sprintf("… %d more (return cv.json(x) for all)", more)
		if n.Style == yaml.FlowStyle {
			n.LineComment = comment
		} else {
			n.FootComment = comment
		}
	}
	return n
}

func (r *renderer) object(obj *goja.Object, depth int) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode}
	keys := obj.Keys()
	shown := 0
	for _, k := range keys {
		if !r.budget() {
			break
		}
		shown++
		n.Content = append(n.Content, str(k), r.property(obj, k, depth))
	}
	flow(n)
	if more := len(keys) - shown; more > 0 {
		n.FootComment = fmt.Sprintf("… %d more keys", more)
	}
	return n
}

// property reads obj[k] through its property descriptor, so accessors are
// reported instead of invoked.
func (r *renderer) property(obj *goja.Object, k string, depth int) *yaml.Node {
	if r.getDesc == nil {
		return r.render(obj.Get(k), depth+1)
	}
	d, err := r.getDesc(goja.Undefined(), obj, r.vm.ToValue(k))
	if err != nil || d == nil || goja.IsUndefined(d) {
		return r.render(obj.Get(k), depth+1)
	}
	desc := d.ToObject(r.vm)
	if g := desc.Get("get"); g != nil && !goja.IsUndefined(g) {
		return str("[Getter]")
	}
	return r.render(desc.Get("value"), depth+1)
}

func (r *renderer) errorValue(obj *goja.Object) *yaml.Node {
	f := readFailure(r.vm, obj)
	n := &yaml.Node{Kind: yaml.MappingNode}
	add := func(k, v string) {
		if v != "" {
			n.Content = append(n.Content, str(k), str(v))
		}
	}
	add("error", f.Name)
	add("code", f.Code)
	add("message", f.Message)
	add("hint", f.Hint)
	add("fix", f.Fix)
	flow(n)
	return n
}

// flow switches small collections of scalars to flow style.
func flow(n *yaml.Node) {
	if len(n.Content) == 0 {
		n.Style = yaml.FlowStyle
		return
	}
	items := len(n.Content)
	if n.Kind == yaml.MappingNode {
		items /= 2
	}
	if items > flowMaxItems && n.Kind == yaml.MappingNode {
		return
	}
	width := 0
	for _, c := range n.Content {
		if c.Kind != yaml.ScalarNode {
			return
		}
		width += len(c.Value) + 2
	}
	if width <= flowMaxChars || n.Kind == yaml.SequenceNode {
		n.Style = yaml.FlowStyle
	}
}

// encodeYAML marshals a node tree.
func encodeYAML(n *yaml.Node) string {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(n); err != nil {
		return fmt.Sprintf("# yaml error: %v", err)
	}
	_ = enc.Close()
	return strings.TrimRight(buf.String(), "\n")
}

// mapping builds a mapping node from alternating keys and values.
func mapping(kv ...*yaml.Node) *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Content: kv}
}

// capText cuts text to at most maxLen characters at a line boundary and appends
// the CUT note.
func capText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	note := fmt.Sprintf("# %s: the result is %d characters; only the first %d are shown. Return less: pick fields, slice arrays, or summarize; return cv.json(x) or cv.csv(x) for complete data.", graph.WCut, len(text), maxLen)
	limit := maxLen - len(note) - 1
	if limit < 0 {
		limit = 0
	}
	for limit > 0 && !utf8Start(text[limit]) {
		limit--
	}
	cut := text[:limit]
	if i := strings.LastIndexByte(cut, '\n'); i > 0 {
		cut = cut[:i]
	}
	return cut + "\n" + note
}

func utf8Start(b byte) bool { return b < 0x80 || b >= 0xC0 }
