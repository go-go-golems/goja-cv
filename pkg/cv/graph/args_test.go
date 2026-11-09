package graph

import (
	"strings"
	"testing"
)

// mustPanic runs f and returns the panic message; it fails when f returns.
func mustPanic(t *testing.T, what string, f func()) string {
	t.Helper()
	var msg string
	func() {
		defer func() {
			if p := recover(); p != nil {
				msg = p.(string)
			}
		}()
		f()
	}()
	if msg == "" {
		t.Fatalf("%s did not panic", what)
	}
	return msg
}

// TestAccessorsAreLoud (GCV-004 §7.2): every accessor panics on a wrong
// spec type, a wrong value type and an optional argument read without Has;
// none returns a zero value.
func TestAccessorsAreLoud(t *testing.T) {
	op := &OpDef{Name: "t", On: KindImage, Args: []ArgSpec{
		{Name: "s", Type: "string"}, {Name: "e", Type: "enum:a,b"}, {Name: "n", Type: "number"}, {Name: "b", Type: "bool"},
		{Name: "ns", Type: "number[]"}, {Name: "ss", Type: "string[]"}, {Name: "o", Type: "object"},
		{Name: "opt", Type: "string", Optional: true}, {Name: "m", Type: "Mask", Optional: true}, {Name: "ims", Type: "Image[]"},
	}}
	recv, other := &Node{ID: 1, Kind: KindImage}, &Node{ID: 2, Kind: KindImage}
	a := Args{op: op, ins: []*Node{recv, other, other}, vals: map[string]any{
		"s": "x", "e": "a", "n": 2.0, "b": true, "ns": []any{1.0, "two"}, "ss": []any{"a", 2.0}, "o": "not an object",
		"ims": []any{InputRef{Index: 1}, InputRef{Index: 2}},
	}}
	if a.String("s") != "x" || a.String("e") != "a" || a.Number("n") != 2 || !a.Bool("b") {
		t.Fatal("well-typed reads")
	}
	for name, f := range map[string]func(){
		"String of a number spec":  func() { a.String("n") },
		"Bool of a string spec":    func() { a.Bool("s") },
		"Number of a string value": func() { a.Number("s") },
		"Numbers with a string":    func() { a.Numbers("ns") },
		"Strings with a number":    func() { a.Strings("ss") },
		"Object of a string":       func() { a.Object("o") },
		"optional without Has":     func() { a.String("opt") },
		"unknown argument":         func() { a.Number("nope") },
		"Value at build time":      func() { a.Value("m") },
		"Input of a literal":       func() { a.Input("s") },
		"Func of a string":         func() { a.Func("s") },
	} {
		if msg := mustPanic(t, name, f); !strings.HasPrefix(msg, "cv: ") {
			t.Fatalf("%s: %q", name, msg)
		}
	}
	if a.Has("opt") || a.Input("m") != nil {
		t.Fatal("absent optional")
	}
	// a node list keeps its order and repetitions
	if ins := a.Inputs("ims"); len(ins) != 2 || ins[0] != other || ins[1] != other {
		t.Fatalf("Inputs: %v", ins)
	}
	if a.Receiver() != recv {
		t.Fatal("Receiver")
	}
	src := Args{op: &OpDef{Name: "src", On: KindNone}}
	mustPanic(t, "Receiver of a source op", func() { src.Receiver() })
	// at evaluation Values resolves the same references
	a.vs = []Value{&DataValue{Data: 0.0}, &DataValue{Data: 1.0}, &DataValue{Data: 2.0}}
	if vs := a.Values("ims"); len(vs) != 2 || vs[0] != a.vs[1] || vs[1] != a.vs[2] {
		t.Fatalf("Values: %v", vs)
	}
}

// TestNodeArgumentsAreOwnInputs: every node argument is its own input, in
// list order with repetitions (no deduplication).
func TestNodeArgumentsAreOwnInputs(t *testing.T) {
	e, _ := newTestEngine(t)
	src := mustCall(t, e, "fill", 0, 0.5)
	m := mustCall(t, e, "mix", src.ID, NodeArg{src.ID}, map[string]any{"w": 0.25})
	if len(m.Inputs) != 2 || m.Inputs[0] != src.ID || m.Inputs[1] != src.ID {
		t.Fatalf("inputs %v", m.Inputs)
	}
	if m.Args.Input("other") != src || m.Args.Receiver() != src {
		t.Fatal("Input/Receiver")
	}
}
