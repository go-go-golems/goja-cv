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

// native is the Go side of init.js. Its functions are called from JavaScript
// and so run on the runtime owner goroutine.
type native struct {
	vm *goja.Runtime
	st *state
}

func newNative(vm *goja.Runtime, st *state) *native { return &native{vm: vm, st: st} }

func (n *native) object() *goja.Object {
	o := n.vm.NewObject()
	_ = o.Set("checkPath", n.checkPath)
	_ = o.Set("queue", n.queue)
	_ = o.Set("getImg", n.getImg)
	_ = o.Set("setImg", n.setImg)
	_ = o.Set("noDoc", n.noDoc)
	_ = o.Set("fail", n.fail)
	return o
}

// throw raises a cv error built like the module's errors (an Error named by
// the code, with code/hint/fix/cv properties).
func (n *native) throw(e *graph.Error) {
	obj, err := n.vm.New(n.vm.Get("Error"), n.vm.ToValue(e.Message))
	if err != nil {
		panic(n.vm.NewGoError(e))
	}
	_ = obj.Set("name", e.Code)
	_ = obj.Set("code", e.Code)
	_ = obj.Set("cv", true)
	if e.Hint != "" {
		_ = obj.Set("hint", e.Hint)
	}
	if e.Fix != "" {
		_ = obj.Set("fix", e.Fix)
	}
	panic(obj)
}

// checkPath(path) throws E_ACCESS when path is outside the session roots.
func (n *native) checkPath(c goja.FunctionCall) goja.Value {
	roots := n.st.opts.Roots
	if len(roots) == 0 {
		return goja.Undefined()
	}
	p := c.Argument(0)
	if goja.IsUndefined(p) || goja.IsNull(p) {
		return goja.Undefined() // cv.open reports the missing path itself
	}
	abs := filepath.Clean(n.st.opts.resolve(p.String()))
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	for _, r := range roots {
		root := r
		if real, err := filepath.EvalSymlinks(r); err == nil {
			root = real
		}
		if rel, err := filepath.Rel(root, abs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return goja.Undefined()
		}
	}
	n.throw(graph.Errorf(EAccess, "cv.open(%q): the path is outside the directories this session may read.", p.String()).
		WithHint("Allowed roots: " + strings.Join(roots, ", ")))
	return nil
}

// queue(look) adds a look to the current eval's shows.
func (n *native) queue(c goja.FunctionCall) goja.Value {
	l, ok := cvmod.LookOf(c.Argument(0))
	if !ok {
		n.throw(graph.Errorf(graph.EArgType, "show() takes a node or a look; got %s.", c.Argument(0).String()).
			WithFix(`show(img.exposure(0.3), { label: "brighter" })`))
	}
	for _, x := range n.st.shows {
		if x == l {
			return goja.Undefined()
		}
	}
	n.st.shows = append(n.st.shows, l)
	return goja.Undefined()
}

func (n *native) getImg(goja.FunctionCall) goja.Value {
	if n.st.img == nil || goja.IsUndefined(n.st.img) {
		n.throw(graph.Errorf(ENoImage, "No image is open, so img is not defined.").
			WithHint("Open one first; it stays bound to img for the next evals.").
			WithFix(`img = cv.open("scan.tif")`))
	}
	return n.st.img
}

func (n *native) setImg(c goja.FunctionCall) goja.Value {
	n.st.img = c.Argument(0)
	return goja.Undefined()
}

func (n *native) noDoc(c goja.FunctionCall) goja.Value {
	n.throw(graph.Errorf(ENoDocument, "doc.%s: the document layer (history, apply, propose, export) is not available in this version.", c.Argument(0).String()).
		WithHint("Work on img and look at results with node.look() or show(node); keep values in mem.").
		WithFix("return { after: img.exposure(0.3).look(\"after\") }"))
	return nil
}

// failure is an uncaught error of the current eval, read from the thrown value.
type failure struct {
	Name, Code, Message, Hint, Fix string
	Path, Valid                    []string
	CV                             bool
	Line, Column                   int // in the evaluated source
	Stack                          string
}

var stackPos = regexp.MustCompile(`<eval>:(\d+):(\d+)`)

// fail(e) records the eval's uncaught error (called by __cv_run).
func (n *native) fail(c goja.FunctionCall) goja.Value {
	n.st.failure = readFailure(n.vm, c.Argument(0))
	return goja.Undefined()
}

func readFailure(vm *goja.Runtime, v goja.Value) *failure {
	f := &failure{}
	obj, ok := v.(*goja.Object)
	if !ok || obj == nil {
		f.Name, f.Message = "Error", "thrown value: "+v.String()
		return f
	}
	get := func(k string) string {
		x := obj.Get(k)
		if x == nil || goja.IsUndefined(x) || goja.IsNull(x) {
			return ""
		}
		return x.String()
	}
	list := func(k string) []string {
		x := obj.Get(k)
		if x == nil || goja.IsUndefined(x) || goja.IsNull(x) {
			return nil
		}
		arr, ok := x.Export().([]any)
		if !ok {
			return []string{x.String()}
		}
		out := make([]string, len(arr))
		for i, e := range arr {
			out[i] = fmt.Sprint(e)
		}
		return out
	}
	f.Name, f.Code, f.Message = get("name"), get("code"), get("message")
	f.Hint, f.Fix = get("hint"), get("fix")
	f.Path, f.Valid = list("path"), list("valid")
	f.CV = obj.Get("cv") != nil && obj.Get("cv").ToBoolean()
	f.Stack = get("stack")
	if f.Message == "" && f.Name == "" {
		f.Message = "thrown value: " + v.String()
	}
	if m := stackPos.FindStringSubmatch(f.Stack); m != nil {
		f.Line, _ = strconv.Atoi(m[1])
		f.Column, _ = strconv.Atoi(m[2])
	}
	return f
}
