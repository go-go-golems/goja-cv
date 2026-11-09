package cvmod_test

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"

	"github.com/go-go-golems/go-go-goja/pkg/engine"
	"github.com/go-go-golems/goja-cv/internal/testimage"
	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph/graphtest"
	"github.com/go-go-golems/goja-cv/pkg/cv/ops"
	cvmod "github.com/go-go-golems/goja-cv/pkg/gojamod/cv"
)

// panicRuntime is a runtime whose registry has three test ops that panic in
// Validate, DocFrame and Run.
func panicRuntime(t *testing.T) *engine.Runtime {
	t.Helper()
	reg := ops.NewRegistry()
	run := func(context.Context, *graph.EvalCtx, []graph.Value, graph.Args) (graph.Value, error) {
		return &graph.DataValue{Data: 1.0}, nil
	}
	var nilSlice []int
	reg.MustRegister(
		graph.OpDef{Name: "test.panicValidate", On: graph.KindImage, Returns: graph.KindValue, Run: run,
			Validate: func(graph.Args) error { panic("validate exploded") }},
		graph.OpDef{Name: "test.panicFrame", On: graph.KindImage, Returns: graph.KindImage, Run: run,
			DocFrame: func(graph.Args) graph.Frame { return graph.Frame{W: nilSlice[3]} }},
		graph.OpDef{Name: "test.panicRun", On: graph.KindImage, Returns: graph.KindValue,
			Run: func(context.Context, *graph.EvalCtx, []graph.Value, graph.Args) (graph.Value, error) {
				return &graph.DataValue{Data: float64(nilSlice[1])}, nil
			}},
	)
	factory, err := engine.NewRuntimeFactoryBuilder().
		WithModules(&cvmod.Registrar{NewEngine: func() *graph.Engine { return graph.NewEngine(reg, graphtest.Options(t)) }}).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	return startRuntime(t, factory)
}

// TestPanicBoundary (GCV-004 §7.11): a Go panic at build time, in an op's
// Run, in any native function and in a decoder surfaces as E_INTERNAL with
// the function's name; the runtime survives and the next call works.
func TestPanicBoundary(t *testing.T) {
	rt := panicRuntime(t)
	for _, c := range []struct{ body, want string }{
		{`return img.test.panicValidate()`, "__call panicked: validate exploded"},
		{`return img.test.panicFrame()`, "__call panicked: runtime error: index out of range"},
		{`return await img.test.panicRun()`, "op test.panicRun panicked: runtime error: index out of range"},
	} {
		e := mustFail(t, rt, graph.EInternal, c.body)
		if !strings.Contains(e.Message, c.want) || !strings.Contains(e.Hint, "goroutine") {
			t.Fatalf("%s: %+v", c.body, e)
		}
		if got := mustRun(t, rt, `return (await img.stats()).w`); got != int64(64) {
			t.Fatalf("after %s: %v", c.body, got)
		}
	}

	// a native function that panics (the test hook stands for any bug in a
	// binding function); a JavaScript try catches it like any cv error
	cvmod.SetPanicHook(func(name string) {
		if name == "__help" {
			panic(fmt.Sprintf("%s hook", name))
		}
	})
	e := mustFail(t, rt, graph.EInternal, `return cv.help("gaussian")`)
	if !strings.Contains(e.Message, "__help panicked: __help hook") {
		t.Fatalf("hook: %+v", e)
	}
	if got := mustRun(t, rt, `try { cv.help() } catch (e) { return e.code }`); got != graph.EInternal {
		t.Fatalf("try/catch: %v", got)
	}
	cvmod.SetPanicHook(nil)
	if got := mustRun(t, rt, `return JSON.stringify(cv.help("gaussian")).includes("sigma")`); got != true {
		t.Fatalf("after the hook: %v", got)
	}
}

// TestOpenEmptyBitsPerSample: the GCV-001 review's TIFF with an empty
// BitsPerSample entry is a format error at cv.open, never a panic.
func TestOpenEmptyBitsPerSample(t *testing.T) {
	b, err := testimage.TIFFBytes(buffer.New(4, 3, 1, 1), testimage.TIFFOptions{Bits: 16})
	if err != nil {
		t.Fatal(err)
	}
	ifd := int(binary.LittleEndian.Uint32(b[4:8]))
	n := int(binary.LittleEndian.Uint16(b[ifd : ifd+2]))
	for i := 0; i < n; i++ {
		if e := ifd + 2 + 12*i; binary.LittleEndian.Uint16(b[e:e+2]) == 258 { // BitsPerSample
			binary.LittleEndian.PutUint32(b[e+4:e+8], 0)
		}
	}
	p := filepath.Join(t.TempDir(), "empty-bits.tif")
	if err := os.WriteFile(p, b, 0o644); err != nil { // #nosec G306 -- test data
		t.Fatal(err)
	}
	rt := newRuntime(t)
	e := mustFail(t, rt, graph.EFormat, fmt.Sprintf(`return await cv.open(%q)`, p))
	if !strings.Contains(e.Message, "BitsPerSample") {
		t.Fatalf("empty BitsPerSample: %+v", e)
	}
}

// TestInterruptPassesThroughNativeFunctions: a native function that runs
// agent JavaScript (cv.json reads a getter) is interrupted by the watchdog;
// the panic boundary passes goja's uncatchable interrupt through, so a
// JavaScript try cannot catch it and the run ends as interrupted (a timeout),
// not as a caught E_INTERNAL.
func TestInterruptPassesThroughNativeFunctions(t *testing.T) {
	rt := newRuntime(t)
	ret, err := rt.Owner.Call(context.Background(), "cv.test.interrupt", func(_ context.Context, vm *goja.Runtime) (any, error) {
		timer := time.AfterFunc(100*time.Millisecond, func() { vm.Interrupt("timeout") })
		defer timer.Stop()
		defer vm.ClearInterrupt()
		v, err := vm.RunString(`const cv = require("cv"); try { cv.save("x.json", { get x() { while (true) {} } }); "no interrupt" } catch (e) { "caught " + e.code }`)
		if err != nil {
			return nil, err
		}
		return v.Export(), nil
	})
	var ie *goja.InterruptedError
	if !errors.As(err, &ie) {
		t.Fatalf("got %v, %v; want an InterruptedError", ret, err)
	}
}

// TestStackOverflowPassesThroughNativeFunctions: goja's other uncatchable
// exception, a stack overflow in agent JavaScript run by a native function,
// is not turned into a catchable E_INTERNAL either.
func TestStackOverflowPassesThroughNativeFunctions(t *testing.T) {
	rt := newRuntime(t)
	ret, err := rt.Owner.Call(context.Background(), "cv.test.overflow", func(_ context.Context, vm *goja.Runtime) (any, error) {
		vm.SetMaxCallStackSize(200)
		defer vm.SetMaxCallStackSize(1 << 20)
		v, err := vm.RunString(`const cv = require("cv"); function f() { return f() }; try { cv.save("x.json", { get x() { return f() } }); "no overflow" } catch (e) { "caught " + e.code }`)
		if err != nil {
			return nil, err
		}
		return v.Export(), nil
	})
	var so *goja.StackOverflowError
	if !errors.As(err, &so) {
		t.Fatalf("got %v, %v; want a StackOverflowError", ret, err)
	}
}
