package cvmod_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dop251/goja"
	cvmod "github.com/go-go-golems/goja-cv/pkg/gojamod/cv"
)

// TestDescribeFunction: the signature and the doc line (a comment that is
// the first token of the body) of every function form, read from goja's
// Function.prototype.toString; mid-body comments are not doc lines;
// accessors are not functions; bound and native functions have no source.
func TestDescribeFunction(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "helpers-fixture.js"))
	if err != nil {
		t.Fatal(err)
	}
	vm := goja.New()
	if _, err := vm.RunString("var mem = {}; var cv = {}; (function () {" + string(src) + "\n})()"); err != nil {
		t.Fatal(err)
	}
	mem := vm.Get("mem").ToObject(vm)
	lib := mem.Get("lib").ToObject(vm)
	get := func(o *goja.Object, k string) *goja.Object { return o.Get(k).ToObject(vm) }
	accessor := func(k, which string) *goja.Object {
		d, err := vm.RunString(`Object.getOwnPropertyDescriptor(mem.lib, "` + k + `").` + which)
		if err != nil {
			t.Fatal(err)
		}
		return d.ToObject(vm)
	}
	for _, tc := range []struct {
		name   string
		fn     *goja.Object
		params string
		doc    string
		listed bool
		native bool
	}{
		{"arrow, block body", get(mem, "tile"), "id, ox, oy", "open a tile at its origin: every coordinate is an absolute scan coordinate", true, false},
		{"async arrow, mid-body comment only", get(mem, "best"), "node, x, y, opts", "", true, false},
		{"async arrow, comment after a statement", get(mem, "probe"), "x, y", "", true, false},
		{"defaults, block comment", get(mem, "hot"), "level = 0.4, max = 80", "components of the speck view above level,", true, false},
		{"expression body", get(mem, "spots"), "level = 0.4", "blobs of the speck view above level, as regions", true, false},
		{"named async function, destructured", get(mem, "sweep"), "{ q = 0, zoom = 2 } = {}", "one quarter of the speck view, scaled on its own", true, false},
		{"single parameter, inline block comment", get(mem, "one"), "x", "the identity", true, false},
		{"comment on the next line", get(mem, "first"), "", "a comment on the line after the brace is still the first token", true, false},
		{"method shorthand", get(lib, "sheet"), "items, size = 40", "contact sheet of up to 15 items", true, false},
		{"generator method", get(lib, "gen"), "a", "a generator method", true, false},
		{"a method named get", get(lib, "get"), "k", "a method named get", true, false},
		{"getter", accessor("count", "get"), "", "", false, false},
		{"setter", accessor("count", "set"), "", "", false, false},
		{"bound", get(mem, "bound"), "", "", true, true},
		{"native", vm.Get("Math").ToObject(vm).Get("max").ToObject(vm), "", "", true, true},
	} {
		d, listed := cvmod.DescribeFunction(cvmod.FunctionSource(vm, tc.fn))
		if listed != tc.listed || d.Params != tc.params || d.Doc != tc.doc || d.Native != tc.native {
			t.Errorf("%s: %+v listed=%v, want params %q doc %q listed=%v native=%v", tc.name, d, listed, tc.params, tc.doc, tc.listed, tc.native)
		}
	}
	// long doc lines are cut at 120 runes
	long, err := vm.RunString("(x => { // " + "é" + repeat("abc ", 40) + "\n return x })")
	if err != nil {
		t.Fatal(err)
	}
	if d, _ := cvmod.DescribeFunction(cvmod.FunctionSource(vm, long.ToObject(vm))); len([]rune(d.Doc)) != 120 || d.Doc[len(d.Doc)-len("…"):] != "…" {
		t.Errorf("long doc %q (%d runes)", d.Doc, len([]rune(d.Doc)))
	}
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
