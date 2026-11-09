package cvsession

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/dop251/goja"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	cvmod "github.com/go-go-golems/goja-cv/pkg/gojamod/cv"
)

// ENameCode is the error code of a bad or taken image name.
const ENameCode = "E_NAME"

// Named images (GCV-001 §7.10): the session, not the engine, owns names. A
// name is a global backed by this table; img is its first entry. Names do
// not change what a node computes, so they never reach the engine or a hash.
type nameTable struct {
	order   []string              // binding order, for the context block
	nodes   map[string]goja.Value // name → value (a node proxy); owner goroutine only
	defined map[string]bool       // globals the session defined for names (img from the start)
}

func newNameTable() *nameTable {
	return &nameTable{nodes: map[string]goja.Value{}, defined: map[string]bool{"img": true}}
}

// set binds name to v; it reports whether the global must be defined.
func (t *nameTable) set(name string, v goja.Value) bool {
	if _, had := t.nodes[name]; !had {
		t.order = append(t.order, name)
	}
	t.nodes[name] = v
	if t.defined[name] {
		return false
	}
	t.defined[name] = true
	return true
}

func (t *nameTable) get(name string) (goja.Value, bool) {
	v, ok := t.nodes[name]
	return v, ok
}

// reservedNames are session globals and JavaScript names a name may not take.
var reservedNames = map[string]bool{
	"cv": true, "mem": true, "globalThis": true, "Math": true, "JSON": true,
	"Promise": true, "console": true, "undefined": true, "NaN": true, "Infinity": true, "eval": true, "arguments": true,
	// keywords
	"break": true, "case": true, "catch": true, "class": true, "const": true, "continue": true, "debugger": true,
	"default": true, "delete": true, "do": true, "else": true, "export": true, "extends": true, "false": true,
	"finally": true, "for": true, "function": true, "if": true, "import": true, "in": true, "instanceof": true,
	"new": true, "null": true, "return": true, "super": true, "switch": true, "this": true, "throw": true, "true": true,
	"try": true, "typeof": true, "var": true, "void": true, "while": true, "with": true, "yield": true, "let": true,
	"static": true, "await": true, "async": true, "enum": true, "implements": true, "interface": true, "package": true,
	"private": true, "protected": true, "public": true,
}

var nameRE = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

// asSynonyms are the option keys agents use for as.
var asSynonyms = []string{"name", "bind", "alias"}

// nameError is E_NAME with a free name as the fix.
func (n *native) nameError(name, why string) *graph.Error {
	e := graph.Errorf(ENameCode, "cv.open(…, { as: %q }): %s.", name, why)
	e.Hint = "A name is a global for an opened image (img is the first). Pick an identifier that is not a session global."
	base := name
	if !nameRE.MatchString(base) || reservedNames[base] {
		base = "image"
		if nameRE.MatchString(name) {
			base = name
		}
	}
	for i := 2; i < 100; i++ {
		cand := base + strconv.Itoa(i)
		if n.nameFree(cand) {
			e.Fix = fmt.Sprintf(`{ as: %q }`, cand)
			break
		}
	}
	return e
}

// nameFree reports whether name may be bound: a global the table defined
// (img from the start), or not a global at all.
func (n *native) nameFree(name string) bool {
	if n.st.names.defined[name] {
		return true
	}
	if !nameRE.MatchString(name) || reservedNames[name] {
		return false
	}
	return n.vm.GlobalObject().Get(name) == nil
}

func containsString(xs []string, x string) bool {
	for _, e := range xs {
		if e == x {
			return true
		}
	}
	return false
}

// checkName(name) throws E_NAME when name cannot be bound.
func (n *native) checkName(c goja.FunctionCall) goja.Value {
	v := c.Argument(0)
	if goja.IsUndefined(v) || goja.IsNull(v) {
		n.throw(n.nameError("", "the name is empty"))
	}
	name, ok := v.Export().(string)
	switch {
	case !ok:
		n.throw(graph.Errorf(ENameCode, "cv.open(…, { as }): the name must be a string, got %s.", v.String()).WithFix(`{ as: "ref" }`))
	case !nameRE.MatchString(name):
		n.throw(n.nameError(name, "a name must be an identifier (letters, digits, _ and $, not starting with a digit)"))
	case reservedNames[name]:
		n.throw(n.nameError(name, fmt.Sprintf("%q is a session global or a JavaScript name", name)))
	case !n.nameFree(name):
		n.throw(n.nameError(name, fmt.Sprintf("%q is already a global (not an image name)", name)))
	}
	return goja.Undefined()
}

// bindName(name, node) binds name; it returns true when the global must be
// defined (a new name).
func (n *native) bindName(c goja.FunctionCall) goja.Value {
	n.checkName(c)
	return n.vm.ToValue(n.st.names.set(c.Argument(0).String(), c.Argument(1)))
}

// getName(name) reads a name; img throws E_NO_IMAGE while unbound.
func (n *native) getName(c goja.FunctionCall) goja.Value {
	name := c.Argument(0).String()
	v, ok := n.st.names.get(name)
	if !ok || v == nil || goja.IsUndefined(v) {
		n.throw(graph.Errorf(ENoImage, "No image is open, so %s is not defined.", name).
			WithHint("Open one first; it stays bound for the next evals.").
			WithFix(fmt.Sprintf(`%s = cv.open("scan.tif")`, name)))
	}
	return v
}

// setName(name, value) rebinds a name (img = cv.open(p)).
func (n *native) setName(c goja.FunctionCall) goja.Value {
	n.st.names.set(c.Argument(0).String(), c.Argument(1))
	return goja.Undefined()
}

// asKey(keys) returns the key of an options object that means as: as
// itself, else a synonym (name, bind, alias); null when there is none.
func (n *native) asKey(c goja.FunctionCall) goja.Value {
	var keys []string
	if arr, ok := c.Argument(0).Export().([]any); ok {
		for _, k := range arr {
			keys = append(keys, fmt.Sprint(k))
		}
	}
	if containsString(keys, "as") {
		return n.vm.ToValue("as")
	}
	for _, s := range asSynonyms {
		if containsString(keys, s) {
			return n.vm.ToValue(s)
		}
	}
	return goja.Null()
}

// imageDesc describes an opened source for the context block:
// "file (1024×1024, 16-bit TIFF, srgb)" or, for a window,
// "file [1536,1536,1024,1024] (16-bit DNG, linear)".
func imageDesc(node *graph.Node) string {
	info, ok := graph.SourceInfo(node)
	if !ok {
		return fmt.Sprintf("#%d %s (%s)", node.ID, graph.Label(node), node.Frame.Size())
	}
	f := node.Frame
	depth := fmt.Sprintf("%v-bit", info["bitDepth"])
	if info["sample"] == "float" {
		depth = fmt.Sprintf("float%v", info["bitDepth"])
	}
	kind := strings.TrimSpace(depth + " " + strings.ToUpper(fmt.Sprint(info["format"])))
	if f.X != 0 || f.Y != 0 {
		return fmt.Sprintf("%s [%d,%d,%d,%d] (%s, %s)", filepath.Base(node.Args.String("path")), f.X, f.Y, f.W, f.H, kind, node.Enc)
	}
	return fmt.Sprintf("%s (%s, %s, %s)", filepath.Base(node.Args.String("path")), f.Size(), kind, node.Enc)
}

// imagesLine is the context block's images entry: names in binding order,
// then unnamed opens.
func (s *state) imagesLine(vm *goja.Runtime) string {
	var parts []string
	named := map[graph.NodeID]bool{}
	for _, name := range s.names.order {
		v, _ := s.names.get(name)
		id, ok := cvmod.NodeOf(v)
		if !ok {
			continue
		}
		node, ok := s.eng.Node(id)
		if !ok {
			continue
		}
		named[id] = true
		parts = append(parts, name+" = "+imageDesc(node))
	}
	for _, id := range cvmod.Opened(vm) {
		if named[id] {
			continue
		}
		if node, ok := s.eng.Node(id); ok {
			parts = append(parts, "(unnamed) "+imageDesc(node))
		}
	}
	return strings.Join(parts, ", ")
}

// namedNodes are the nodes bound to names (they count as used).
func (s *state) namedNodes() []graph.NodeID {
	var out []graph.NodeID
	for _, name := range s.names.order {
		v, _ := s.names.get(name)
		if id, ok := cvmod.NodeOf(v); ok {
			out = append(out, id)
		}
	}
	return out
}
