// Package cvmod is the go-go-goja native module "cv": it exposes the cv
// engine to JavaScript through opaque node handles and an embedded prelude
// that builds the fluent surface.
package cvmod

import (
	_ "embed"
	"sync"

	"github.com/dop251/goja"
	"github.com/go-go-golems/go-go-goja/modules"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/pkg/errors"
)

//go:embed prelude.js
var preludeSource string

var (
	preludeOnce    sync.Once
	preludeProgram *goja.Program
	preludeErr     error
)

func compiledPrelude() (*goja.Program, error) {
	preludeOnce.Do(func() {
		preludeProgram, preludeErr = goja.Compile("cv/prelude.js", preludeSource, true)
	})
	return preludeProgram, preludeErr
}

// Module is the "cv" native module.
type Module struct {
	newRegistry func() *graph.Registry
}

var _ modules.NativeModule = &Module{}

// Option configures a Module.
type Option func(*Module)

// WithRegistry makes every runtime that loads the module use registries built
// by fn (one per runtime).
func WithRegistry(fn func() *graph.Registry) Option {
	return func(m *Module) { m.newRegistry = fn }
}

// New returns a cv module. The default registry is empty in M0.
func New(opts ...Option) *Module {
	m := &Module{newRegistry: graph.NewRegistry}
	for _, o := range opts {
		o(m)
	}
	return m
}

func (m *Module) Name() string { return "cv" }

func (m *Module) Doc() string {
	return "Lazy image-analysis and editing graph. Call cv.help() or cv.help('name') for the reference."
}

func (m *Module) Loader(vm *goja.Runtime, moduleObj *goja.Object) {
	b := newBinding(vm, m.newRegistry())
	exports := moduleObj.Get("exports").(*goja.Object)
	cvObj, err := runPrelude(vm, b.object())
	if err != nil {
		panic(vm.NewGoError(err))
	}
	for _, k := range cvObj.Keys() {
		modules.SetExport(exports, m.Name(), k, cvObj.Get(k))
	}
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

func init() {
	modules.Register(New())
}
