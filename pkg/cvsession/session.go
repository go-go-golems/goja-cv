// Package cvsession is the cv eval session: a go-go-goja replapi app whose
// runtimes load only the cv module (and timer), with pre-bound globals (cv, P,
// mem, show, img, doc) and the cv result processor (YAML results capped at
// 9000 characters, structured errors with the agent's line, RANGE and
// UNUSED_NODE warnings, a context block, and looks delivered as image
// content).
//
// Agent code is the body of an async function: App.Evaluate wraps it before
// handing it to replapi, so `await` and `return` work at the top level and
// errors map back to the agent's line numbers.
package cvsession

import (
	"context"
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dop251/goja"
	"github.com/go-go-golems/go-go-goja/pkg/engine"
	"github.com/go-go-golems/go-go-goja/pkg/replapi"
	"github.com/go-go-golems/go-go-goja/pkg/replsession"
	"github.com/go-go-golems/go-go-goja/pkg/resultcontent"
	"github.com/go-go-golems/go-go-goja/pkg/runtimebridge"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/look"
	cvmod "github.com/go-go-golems/goja-cv/pkg/gojamod/cv"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

const (
	// DefaultEvalTimeout bounds one eval (Part 2 §12).
	DefaultEvalTimeout = 60 * time.Second
	// DefaultMaxResultChars caps the YAML result text.
	DefaultMaxResultChars = 9000
	// DefaultMaxLooks is the number of images one eval delivers.
	DefaultMaxLooks = 4
	// DefaultMaxLookBytes bounds the total bytes of the content of one eval.
	DefaultMaxLookBytes = 8 << 20
	// ENoImage is the error code of using img when no image is open.
	ENoImage = "E_NO_IMAGE"
)

// Options configure a cv session app.
type Options struct {
	// Image is opened in every new session and bound to img. It is operator
	// input (resolved against the working directory) and must lie in Root or
	// a Read directory.
	Image string
	// Root is the workspace (§7.8): the one directory cv.save writes and
	// cv.open reads relative paths in (default: the working directory).
	Root string
	// Read are read-only directories cv.open also reads, by absolute path or
	// by a relative path the workspace does not have (in order).
	Read []string
	// Engine options for each session's engine (its Workspace is opened from
	// Root and Read, one per session).
	Engine graph.EngineOptions
	// EvalTimeout bounds one eval (default 60 s).
	EvalTimeout time.Duration
	// MaxResultChars caps the YAML result (default 9000).
	MaxResultChars int
	// MaxLooks and MaxLookBytes bound the images of one eval (default 4, 8 MiB
	// for all content together).
	MaxLooks     int
	MaxLookBytes int
	// Preludes are script files loaded, in order, in every new session
	// before agent code: await cv.load(file) (GCV-005 §6.3); their results and
	// looks are not shown. Paths are operator input, resolved against the
	// working directory; each must lie in Root or a Read directory.
	Preludes []string
	// Logger for replapi (default: disabled).
	Logger *zerolog.Logger
}

func (o Options) withDefaults() (Options, error) {
	if o.EvalTimeout <= 0 {
		o.EvalTimeout = DefaultEvalTimeout
	}
	if o.MaxResultChars <= 0 {
		o.MaxResultChars = DefaultMaxResultChars
	}
	if o.MaxLooks <= 0 {
		o.MaxLooks = DefaultMaxLooks
	}
	if o.MaxLookBytes <= 0 {
		o.MaxLookBytes = DefaultMaxLookBytes
	}
	wd, err := os.Getwd()
	if err != nil {
		return o, errors.Wrap(err, "getwd")
	}
	abs := func(p string) string {
		if filepath.IsAbs(p) {
			return filepath.Clean(p)
		}
		return filepath.Join(wd, p)
	}
	if o.Root == "" {
		o.Root = wd
	}
	o.Root = abs(o.Root)
	reads := make([]string, len(o.Read))
	for i, r := range o.Read {
		reads[i] = abs(r)
	}
	o.Read = reads
	ws, err := graph.OpenWorkspace(o.Root, o.Read) // the directories exist
	if err != nil {
		return o, err
	}
	defer ws.Close()
	if o.Image != "" {
		o.Image = abs(o.Image)
	}
	preludes := make([]string, 0, len(o.Preludes))
	for _, p := range o.Preludes {
		p = abs(p)
		f, _, err := ws.Open(p)
		if err != nil {
			if _, serr := os.Stat(p); serr != nil {
				return o, errors.Errorf("prelude %s: %v", p, serr)
			}
			return o, errors.Errorf("prelude %s is outside the workspace %s and the read directories; add --read %s", p, ws.Dir(), filepath.Dir(p))
		}
		_ = f.Close()
		preludes = append(preludes, p)
	}
	o.Preludes = preludes
	return o, nil
}

// ContentLimits are the resultcontent limits of the session: MaxLooks looks
// (the session enforces the count in collect) and MaxLookBytes bytes per
// eval.
func (o Options) ContentLimits() resultcontent.Limits {
	return resultcontent.Limits{MaxItems: o.MaxLooks, MaxTotalBytes: o.MaxLookBytes}
}

// SessionPolicy is the replsession policy of cv sessions: raw execution (the
// session wraps agent code itself), top-level await, console capture, no
// persistence, and the eval timeout.
func SessionPolicy(timeout time.Duration) replsession.SessionPolicy {
	return replsession.SessionPolicy{
		Eval: replsession.EvalPolicy{
			Mode:                 replsession.EvalModeRaw,
			SupportTopLevelAwait: true,
			TimeoutMS:            timeout.Milliseconds(),
		},
		Observe: replsession.ObservePolicy{ConsoleCapture: true},
	}
}

// NewRuntimeFactory builds the runtime factory of cv sessions: the cv module
// (through cvmod.Registrar, so the session engine is in the runtime values),
// timer, and the session initializer. No other modules are available.
func NewRuntimeFactory(opts Options) (*engine.RuntimeFactory, error) {
	opts, err := opts.withDefaults()
	if err != nil {
		return nil, err
	}
	newEngine := func() *graph.Engine {
		eo := opts.Engine
		// one workspace per session: the files it opened are its own (an
		// error here leaves the engine without one: cv.open and cv.save say so)
		eo.Workspace, _ = graph.OpenWorkspace(opts.Root, opts.Read)
		return graph.NewEngine(cvmod.DefaultRegistry(), eo)
	}
	return engine.NewRuntimeFactoryBuilder(engine.WithDataOnlyDefaultRegistryModules(false)).
		UseModuleMiddleware(engine.MiddlewareOnly("timer")).
		WithModules(&cvmod.Registrar{NewEngine: newEngine}).
		WithRuntimeInitializers(&Initializer{opts: opts}).
		Build()
}

// App is a replapi app of cv sessions. Its Evaluate takes agent code (an
// async function body); everything else is replapi's.
type App struct {
	*replapi.App
	opts      Options
	states    sync.Map // session id → *state
	evalLocks sync.Map // session id → *sync.Mutex

	mu       sync.Mutex
	sessions []string // ids created through CreateSession, in order
}

// CreateSession creates a cv session, runs the preludes in it and
// remembers its id (Sessions). A prelude that fails deletes the session and
// is reported as "prelude FILE:LINE: …".
func (a *App) CreateSession(ctx context.Context) (*replsession.SessionSummary, error) {
	s, err := a.App.CreateSession(ctx)
	if err != nil || s == nil {
		return s, err
	}
	if err := a.runPreludes(ctx, s.ID); err != nil {
		_ = a.DeleteSession(context.Background(), s.ID)
		return nil, err
	}
	a.mu.Lock()
	a.sessions = append(a.sessions, s.ID)
	a.mu.Unlock()
	return s, nil
}

// whereFile finds the file location of a rendered error in a loaded file
// ("where: 'dust/helpers.js:57: …'").
var whereFile = regexp.MustCompile(`where: '?((?:read\d+:)?[^\s:']+:\d+):`)

// runPreludes loads the prelude files in session id, in order (await
// cv.load(file)). Preludes have no output: the looks they create are
// discarded, and the context block does not mark their functions as shown.
func (a *App) runPreludes(ctx context.Context, id string) error {
	st, err := a.stateFor(ctx, id)
	if err != nil {
		return err
	}
	st.prelude.Store(true)
	defer st.prelude.Store(false)
	for _, p := range a.opts.Preludes {
		code, _ := json.Marshal(p)
		resp, err := a.Evaluate(ctx, id, "await cv.load("+string(code)+")")
		if err != nil && (resp == nil || resp.Cell == nil) {
			return errors.Wrapf(err, "prelude %s", filepath.Base(p))
		}
		if exec := resp.Cell.Execution; exec.Status != "ok" {
			msg := exec.Error
			if i := strings.Index(exec.Result, "message: "); i >= 0 {
				msg = strings.TrimSpace(strings.SplitN(exec.Result[i+len("message: "):], "\n", 2)[0])
				msg = strings.Trim(msg, "'\"")
			}
			if m := whereFile.FindStringSubmatch(exec.Result); m != nil {
				return errors.Errorf("prelude %s: %s", m[1], msg)
			}
			if strings.HasPrefix(msg, filepath.Base(p)) || strings.Contains(msg, ".js:") {
				return errors.Errorf("prelude %s", msg) // a syntax error names the file and line itself
			}
			return errors.Errorf("prelude %s: %s", filepath.Base(p), msg)
		}
	}
	return nil
}

// Sessions lists the ids of the sessions created through this app.
func (a *App) Sessions() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.sessions...)
}

// New builds a cv session app owned by ctx.
func New(ctx context.Context, opts Options, extra ...replapi.Option) (*App, error) {
	opts, err := opts.withDefaults()
	if err != nil {
		return nil, err
	}
	factory, err := NewRuntimeFactory(opts)
	if err != nil {
		return nil, err
	}
	logger := zerolog.Nop()
	if opts.Logger != nil {
		logger = *opts.Logger
	}
	appOpts := append([]replapi.Option{
		replapi.WithProfile(replapi.ProfileRaw),
		replapi.WithDefaultSessionPolicy(SessionPolicy(opts.EvalTimeout)),
		replapi.WithResultProcessor(&ResultProcessor{opts: opts}),
		replapi.WithResultContentLimits(opts.ContentLimits()),
	}, extra...)
	app, err := replapi.New(ctx, factory, logger, appOpts...)
	if err != nil {
		return nil, errors.Wrap(err, "create cv session app")
	}
	return &App{App: app, opts: opts}, nil
}

// Options returns the normalized options.
func (a *App) Options() Options { return a.opts }

// WrapPrefix and WrapSuffix surround agent code; the prefix is one line, so
// agent line N is line N+1 of the evaluated source.
const (
	WrapPrefix = "__cv_run(async function () {\n"
	WrapSuffix = "\n})"
)

// Wrap turns agent code into the source App.Evaluate runs.
func Wrap(body string) string { return WrapPrefix + body + WrapSuffix }

// Evaluate runs agent code (the body of an async function) in session
// sessionID.
//
// Evaluations of one session are serialized here (replapi serializes them
// too, but in lock order, not call order): the agent code handed to the
// runtime state must belong to the eval that runs next, or line numbers and
// source text in errors and warnings would come from a concurrent eval.
func (a *App) Evaluate(ctx context.Context, sessionID string, body string) (*replsession.EvaluateResponse, error) {
	mu, _ := a.evalLocks.LoadOrStore(sessionID, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
	defer mu.(*sync.Mutex).Unlock()
	if st, err := a.stateFor(ctx, sessionID); err == nil {
		st.setPending(body)
	}
	return a.App.Evaluate(ctx, sessionID, Wrap(body))
}

func (a *App) stateFor(ctx context.Context, sessionID string) (*state, error) {
	if st, ok := a.states.Load(sessionID); ok {
		return st.(*state), nil
	}
	var found *state
	err := a.WithRuntime(ctx, sessionID, func(_ context.Context, rt *engine.Runtime) error {
		st, ok := states.Load(rt.VM)
		if !ok {
			return errors.New("cvsession: runtime has no cv session state")
		}
		found = st.(*state)
		return nil
	})
	if err != nil {
		return nil, err
	}
	a.states.Store(sessionID, found)
	return found, nil
}

// states maps *goja.Runtime → *state for the result processor, which only
// receives the VM.
var states sync.Map

// state is the per-runtime session state. Fields marked (owner) are only
// touched on the runtime owner goroutine.
type state struct {
	opts Options
	eng  *graph.Engine

	mu      sync.Mutex
	pending string      // body of the next eval (set by App.Evaluate)
	prelude atomic.Bool // a prelude is running: its looks are discarded

	// (owner)
	wrapped  bool     // the current eval came through App.Evaluate
	lines    []string // agent code of the current eval
	failure  *failure
	lookSeq  int
	names    *nameTable // named images (img first)
	broken   bool       // the VM can no longer run promise jobs (see probePromises)
	watchdog *watchdog
}

// watchdog interrupts the VM when the eval deadline passes. replsession only
// interrupts while the cell's synchronous part runs; JavaScript that loops
// after an await runs in an event-loop callback and would wedge the owner
// goroutine forever without this.
type watchdog struct {
	mu   sync.Mutex
	done bool
	stop func() bool
}

func armWatchdog(ctx context.Context, vm *goja.Runtime) *watchdog {
	w := &watchdog{}
	w.stop = context.AfterFunc(ctx, func() {
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return
		}
		// Give replsession's own interrupt (synchronous part) time to end
		// the eval first: two interrupts in a row can leave goja unable to
		// run promise jobs.
		time.AfterFunc(watchdogGrace, func() {
			w.mu.Lock()
			defer w.mu.Unlock()
			if !w.done {
				vm.Interrupt(context.DeadlineExceeded)
			}
		})
	})
	return w
}

const watchdogGrace = 100 * time.Millisecond

// disarm stops the watchdog and clears an interrupt it may have raised (owner).
func (w *watchdog) disarm(vm *goja.Runtime) {
	if w == nil {
		return
	}
	w.mu.Lock()
	w.done = true
	w.mu.Unlock()
	w.stop()
	vm.ClearInterrupt()
}

func stateOf(vm *goja.Runtime) (*state, bool) {
	st, ok := states.Load(vm)
	if !ok {
		return nil, false
	}
	return st.(*state), true
}

func (s *state) setPending(body string) {
	s.mu.Lock()
	s.pending = body
	s.mu.Unlock()
}

// begin resets the per-eval state (owner).
func (s *state) begin() {
	s.mu.Lock()
	body, wrapped := s.pending, s.pending != ""
	s.pending = ""
	s.mu.Unlock()
	s.wrapped = wrapped
	s.lines = strings.Split(body, "\n")
	s.failure = nil
}

// agentLine maps a line of the evaluated source to the agent's line (0 when
// it is outside the agent's code).
func (s *state) agentLine(evalLine int) int {
	l := evalLine
	if s.wrapped {
		l--
	}
	if l < 1 || (s.wrapped && l > len(s.lines)) {
		return 0
	}
	return l
}

// sourceLine is agent line l, trimmed ("" when unknown).
func (s *state) sourceLine(l int) string {
	if l < 1 || l > len(s.lines) || !s.wrapped {
		return ""
	}
	return strings.TrimSpace(s.lines[l-1])
}

// nextLookNumber numbers l if it has no number yet (owner).
func (s *state) number(l *look.Look) {
	if l.Number() > 0 {
		return
	}
	s.lookSeq++
	if got := l.AssignNumber(s.lookSeq); got != s.lookSeq {
		s.lookSeq--
	}
}

// Initializer sets up a cv session runtime: it loads cv, defines the session
// globals and opens Options.Image.
type Initializer struct {
	opts Options
}

var _ engine.RuntimeInitializer = &Initializer{}

// NewInitializer returns the runtime initializer for opts.
func NewInitializer(opts Options) (*Initializer, error) {
	opts, err := opts.withDefaults()
	if err != nil {
		return nil, err
	}
	return &Initializer{opts: opts}, nil
}

func (i *Initializer) ID() string { return "goja-cv:session" }

//go:embed init.js
var initSource string

var (
	initOnce    sync.Once
	initProgram *goja.Program
	initErr     error
)

// InitRuntime implements engine.RuntimeInitializer.
func (i *Initializer) InitRuntime(ctx *engine.RuntimeInitializationContext) error {
	vm := ctx.VM
	cvExports, err := ctx.Require.Require("cv")
	if err != nil {
		return errors.Wrap(err, "require cv")
	}
	engVal, ok := ctx.Value(cvmod.EngineValueKey)
	if !ok {
		return errors.New("cvsession: runtime has no cv engine (register cvmod.Registrar)")
	}
	eng, ok := engVal.(*graph.Engine)
	if !ok {
		return errors.Errorf("cvsession: %s is %T", cvmod.EngineValueKey, engVal)
	}
	st := &state{opts: i.opts, eng: eng, names: newNameTable()}
	states.Store(vm, st)
	if svc, ok := runtimebridge.Lookup(vm); ok {
		context.AfterFunc(svc.Lifetime(), func() { states.CompareAndDelete(vm, st) })
	}
	initOnce.Do(func() { initProgram, initErr = goja.Compile("cv/session.js", initSource, true) })
	if initErr != nil {
		return errors.Wrap(initErr, "compile session init")
	}
	fnVal, err := vm.RunProgram(initProgram)
	if err != nil {
		return errors.Wrap(err, "run session init")
	}
	fn, ok := goja.AssertFunction(fnVal)
	if !ok {
		return errors.New("session init did not evaluate to a function")
	}
	if _, err := fn(goja.Undefined(), newNative(vm, st).object(), cvExports); err != nil {
		return errors.Wrap(err, "initialize session globals")
	}
	if i.opts.Image != "" {
		open, _ := goja.AssertFunction(cvExports.ToObject(vm).Get("open"))
		node, err := open(goja.Undefined(), vm.ToValue(i.opts.Image))
		if err != nil {
			return errors.Wrapf(err, "open --image %s", i.opts.Image)
		}
		st.names.set("img", node)
	}
	return nil
}
