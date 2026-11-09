package cvsession

import (
	"fmt"
	"regexp"
	"strconv"

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
	set := func(name string, fn func(goja.FunctionCall) goja.Value) {
		_ = o.Set(name, cvmod.Export(n.vm, name, fn))
	}
	set("pendingSaves", func(goja.FunctionCall) goja.Value { return cvmod.SavesPending(n.vm) })
	set("saveFailure", func(goja.FunctionCall) goja.Value { return cvmod.SaveFailure(n.vm) })
	set("checkName", n.checkName)
	set("bindName", n.bindName)
	set("getName", n.getName)
	set("setName", n.setName)
	set("fail", n.fail)
	set("unknownMember", n.unknownMember)
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
	if len(e.Valid) > 0 {
		valid := make([]any, len(e.Valid))
		for i, v := range e.Valid {
			valid[i] = v
		}
		_ = obj.Set("valid", n.vm.NewArray(valid...))
	}
	panic(obj)
}

// unknownMember(obj, name, members) throws the structured unknown-name error
// for cv.name.
func (n *native) unknownMember(c goja.FunctionCall) goja.Value {
	var members []string
	if arr, ok := c.Argument(2).Export().([]any); ok {
		for _, m := range arr {
			members = append(members, fmt.Sprint(m))
		}
	}
	e := cvmod.UnknownMember(n.st.eng.Registry(), c.Argument(0).String(), c.Argument(1).String(), members)
	n.throw(e)
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
