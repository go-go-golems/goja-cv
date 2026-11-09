package cvmod_test

import (
	"context"
	"testing"

	"github.com/dop251/goja"
	"github.com/go-go-golems/go-go-goja/pkg/engine"
	_ "github.com/go-go-golems/goja-cv/pkg/gojamod/cv"
)

func newRuntime(t *testing.T) *engine.Runtime {
	t.Helper()
	factory, err := engine.NewRuntimeFactoryBuilder().UseModuleMiddleware(engine.MiddlewareOnly("cv")).Build()
	if err != nil {
		t.Fatalf("build runtime factory: %v", err)
	}
	rt, err := factory.NewRuntime(engine.WithStartupContext(context.Background()), engine.WithLifetimeContext(context.Background()))
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close(context.Background()) })
	return rt
}

func TestRequireCVHelp(t *testing.T) {
	rt := newRuntime(t)
	ret, err := rt.Owner.Call(context.Background(), "cv.test.help", func(_ context.Context, vm *goja.Runtime) (any, error) {
		v, err := vm.RunString(`const cv = require("cv"); const h = cv.help(); ({ kinds: h.kinds.join(","), nOps: Object.keys(h.ops).length })`)
		if err != nil {
			return nil, err
		}
		return v.Export(), nil
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	got := ret.(map[string]any)
	if got["kinds"] != "Image,Mask,Regions,Value,Param" {
		t.Fatalf("kinds = %v", got["kinds"])
	}
	if got["nOps"] != int64(0) {
		t.Fatalf("nOps = %#v", got["nOps"])
	}
}
