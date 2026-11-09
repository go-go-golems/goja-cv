// Package cvmod is the go-go-goja native module "cv": it exposes the cv
// engine to JavaScript through opaque node handles and an embedded prelude
// that builds the fluent surface (node proxies, namespaces, thenables,
// diagnostics).
package cvmod

import (
	"context"
	_ "embed"
	"sync"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/go-go-golems/go-go-goja/modules"
	"github.com/go-go-golems/go-go-goja/pkg/engine"
	"github.com/go-go-golems/go-go-goja/pkg/runtimebridge"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/ops"
	"github.com/pkg/errors"
)

// Version is the cv module version reported as cv.version.
const Version = "0.1.0-m1"

// EngineValueKey is the runtime Values key under which Registrar stores the
// session engine.
const EngineValueKey = "cv.engine"

//go:embed prelude.js
var preludeSource string

var (
	preludeOnce    sync.Once
	preludeProgram *goja.Program
	preludeErr     error

	defaultRegistryOnce sync.Once
	defaultRegistry     *graph.Registry

	bindings sync.Map // *goja.Runtime → *binding
)

func compiledPrelude() (*goja.Program, error) {
	preludeOnce.Do(func() {
		preludeProgram, preludeErr = goja.Compile("cv/prelude.js", preludeSource, true)
	})
	return preludeProgram, preludeErr
}

// DefaultRegistry is the shared registry with every built-in op. Registries
// are read-only after construction, so engines can share it.
func DefaultRegistry() *graph.Registry {
	defaultRegistryOnce.Do(func() { defaultRegistry = ops.NewRegistry() })
	return defaultRegistry
}

// DefaultEngine builds a new engine over DefaultRegistry.
func DefaultEngine() *graph.Engine { return graph.NewEngine(DefaultRegistry(), graph.EngineOptions{}) }

// Module is the "cv" native module. Each runtime that loads it gets its own
// engine from the module's engine factory.
type Module struct {
	newEngine func() *graph.Engine
}

var _ modules.NativeModule = &Module{}

// Option configures a Module.
type Option func(*Module)

// WithEngineFactory makes every runtime that loads the module use an engine
// built by fn.
func WithEngineFactory(fn func() *graph.Engine) Option {
	return func(m *Module) { m.newEngine = fn }
}

// New returns a cv module.
func New(opts ...Option) *Module {
	m := &Module{newEngine: DefaultEngine}
	for _, o := range opts {
		o(m)
	}
	return m
}

func (m *Module) Name() string { return "cv" }

func (m *Module) Doc() string {
	return "Lazy image-analysis and editing graph. Call cv.help() or cv.help('name') for the reference."
}

// Loader builds the module for one runtime.
func (m *Module) Loader(vm *goja.Runtime, moduleObj *goja.Object) {
	load(vm, moduleObj, m.newEngine())
}

func load(vm *goja.Runtime, moduleObj *goja.Object, eng *graph.Engine) {
	b := newBinding(vm, eng)
	bindings.Store(vm, b)
	if svc, ok := runtimebridge.Lookup(vm); ok {
		context.AfterFunc(svc.Lifetime(), func() { bindings.CompareAndDelete(vm, b) })
	}
	exports := moduleObj.Get("exports").(*goja.Object)
	cvObj, err := runPrelude(vm, b.object())
	if err != nil {
		panic(vm.NewGoError(err))
	}
	for _, k := range cvObj.Keys() {
		modules.SetExport(exports, "cv", k, cvObj.Get(k))
	}
}

// EngineFor returns the engine of the cv module loaded in vm.
func EngineFor(vm *goja.Runtime) (*graph.Engine, bool) {
	b, ok := bindings.Load(vm)
	if !ok {
		return nil, false
	}
	return b.(*binding).eng, true
}

// DrainWarnings returns and clears the warnings (RANGE, …) collected while
// building nodes in vm. Call it on the owner goroutine.
func DrainWarnings(vm *goja.Runtime) []graph.Warning {
	b, ok := bindings.Load(vm)
	if !ok {
		return nil
	}
	return b.(*binding).drainWarnings()
}

func runPrelude(vm *goja.Runtime, native *goja.Object) (*goja.Object, error) {
	prog, err := compiledPrelude()
	if err != nil {
		return nil, errors.Wrap(err, "compile cv prelude")
	}
	fnVal, err := vm.RunProgram(prog)
	if err != nil {
		return nil, errors.Wrap(err, "run cv prelude")
	}
	fn, ok := goja.AssertFunction(fnVal)
	if !ok {
		return nil, errors.New("cv prelude did not evaluate to a function")
	}
	res, err := fn(goja.Undefined(), native)
	if err != nil {
		return nil, errors.Wrap(err, "initialize cv prelude")
	}
	return res.ToObject(vm), nil
}

// Registrar is a runtime-aware registration of the "cv" module for
// engine.RuntimeFactoryBuilder.WithModules: it creates one engine per runtime,
// stores it in the runtime Values under EngineValueKey (for session code such
// as the eval tool) and registers a loader bound to it.
type Registrar struct {
	NewEngine func() *graph.Engine
}

var _ engine.RuntimeModuleRegistrar = &Registrar{}

func (r *Registrar) ID() string { return "goja-cv:cv" }

func (r *Registrar) RegisterRuntimeModule(ctx *engine.RuntimeModuleRegistrationContext, reg *require.Registry) error {
	newEngine := r.NewEngine
	if newEngine == nil {
		newEngine = DefaultEngine
	}
	eng := newEngine()
	ctx.SetValue(EngineValueKey, eng)
	reg.RegisterNativeModule("cv", func(vm *goja.Runtime, moduleObj *goja.Object) {
		load(vm, moduleObj, eng)
	})
	return nil
}

func init() {
	modules.Register(New())
}
