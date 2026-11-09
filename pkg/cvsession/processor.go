package cvsession

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/dop251/goja"
	"github.com/go-go-golems/go-go-goja/pkg/replsession"
	"github.com/go-go-golems/go-go-goja/pkg/resultcontent"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/look"
	cvmod "github.com/go-go-golems/goja-cv/pkg/gojamod/cv"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// ResultProcessor is the cv result processor: YAML text (capped), warnings,
// a context block, and the looks of the eval as content providers. It also
// resets per-eval state before each eval and renders failures (with the
// looks shown before the error).
type ResultProcessor struct {
	opts Options
}

var (
	_ replsession.ResultProcessor  = &ResultProcessor{}
	_ replsession.BeforeEvalHook   = &ResultProcessor{}
	_ replsession.FailureProcessor = &ResultProcessor{}
)

// NewResultProcessor returns the processor for opts.
func NewResultProcessor(opts Options) (*ResultProcessor, error) {
	opts, err := opts.withDefaults()
	if err != nil {
		return nil, err
	}
	return &ResultProcessor{opts: opts}, nil
}

// BeforeEval implements replsession.BeforeEvalHook: it resets the lint state
// and queued shows and makes ctx (the eval's deadline) the context of the
// evaluations the code starts.
func (p *ResultProcessor) BeforeEval(ctx context.Context, vm *goja.Runtime) error {
	st, ok := stateOf(vm)
	if !ok {
		return nil
	}
	if st.broken {
		return errors.New(brokenMessage)
	}
	st.begin()
	st.watchdog = armWatchdog(ctx, vm)
	cvmod.DrainWarnings(vm)
	cvmod.BeginEval(vm, ctx)
	return nil
}

// Process implements replsession.ResultProcessor.
func (p *ResultProcessor) Process(_ context.Context, vm *goja.Runtime, value goja.Value) (replsession.ProcessedResult, error) {
	st, ok := stateOf(vm)
	if !ok {
		return replsession.DefaultResultProcessor{FullText: true}.Process(context.Background(), vm, value)
	}
	st.watchdog.disarm(vm)
	trace := cvmod.EndEval(vm)
	extraction, err := resultcontent.Extract(vm, value, resultcontent.Limits{})
	if err != nil {
		return replsession.ProcessedResult{}, err
	}
	providers := st.collect(extraction.Providers)

	r := newRenderer(vm, st.eng)
	r.providers = providerNumbers(providers)
	var sb strings.Builder
	if value == nil || goja.IsUndefined(value) {
		sb.WriteString("# nothing returned: end the code with `return …` to see a value (the last expression is not returned).")
	} else {
		sb.WriteString(encodeYAML(r.render(value, 0)))
	}
	if len(st.shows) > 0 {
		sb.WriteString("\n\n")
		sb.WriteString(encodeYAML(mapping(str("shown"), lookList(st.shows))))
	}
	warnings := st.warnings(vm, trace, r.nodes, true)
	return replsession.ProcessedResult{Text: st.finish(vm, sb.String(), warnings), Providers: providers}, nil
}

// ProcessFailure implements replsession.FailureProcessor: the structured
// error in YAML, the looks shown before the error, warnings and context.
func (p *ResultProcessor) ProcessFailure(_ context.Context, vm *goja.Runtime, evalErr error) (replsession.ProcessedResult, error) {
	st, ok := stateOf(vm)
	if !ok {
		return replsession.ProcessedResult{}, nil
	}
	st.watchdog.disarm(vm)
	trace := cvmod.EndEval(vm)
	providers := st.collect(nil)
	var sb strings.Builder
	sb.WriteString(encodeYAML(mapping(str("error"), st.errorNode(vm, evalErr))))
	if len(st.shows) > 0 {
		sb.WriteString("\n\n")
		sb.WriteString(encodeYAML(mapping(str("shown"), lookList(st.shows))))
	}
	warnings := st.warnings(vm, trace, nil, false)
	return replsession.ProcessedResult{Text: st.finish(vm, sb.String(), warnings), Providers: providers}, nil
}

// collect appends the queued shows to the value's providers (once each) and
// numbers every look (owner).
func (s *state) collect(fromValue []resultcontent.Provider) []resultcontent.Provider {
	out := make([]resultcontent.Provider, 0, len(fromValue)+len(s.shows))
	seen := map[*look.Look]bool{}
	for _, p := range fromValue {
		if l, ok := p.(*look.Look); ok {
			if seen[l] {
				continue
			}
			seen[l] = true
		}
		out = append(out, p)
	}
	for _, l := range s.shows {
		if !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	for _, p := range out {
		if l, ok := p.(*look.Look); ok {
			s.number(l)
		}
	}
	return out
}

func providerNumbers(providers []resultcontent.Provider) map[resultcontent.Provider]int {
	out := map[resultcontent.Provider]int{}
	for i, p := range providers {
		if p != nil && reflect.TypeOf(p).Comparable() {
			if _, ok := out[p]; !ok {
				out[p] = i + 1
			}
		}
	}
	return out
}

func lookList(ls []*look.Look) *yaml.Node {
	n := &yaml.Node{Kind: yaml.SequenceNode}
	for _, l := range ls {
		n.Content = append(n.Content, str(l.String()))
	}
	return n
}

// finish appends warnings and the context block and applies the cap (owner).
func (s *state) finish(vm *goja.Runtime, body string, warnings []string) string {
	var sb strings.Builder
	sb.WriteString(body)
	if len(warnings) > 0 {
		list := &yaml.Node{Kind: yaml.SequenceNode}
		for _, w := range warnings {
			list.Content = append(list.Content, str(w))
		}
		sb.WriteString("\n\n")
		sb.WriteString(encodeYAML(mapping(str("warnings"), list)))
	}
	if ctx := s.contextBlock(vm); ctx != nil {
		sb.WriteString("\n\n")
		sb.WriteString(encodeYAML(mapping(str("context"), ctx)))
	}
	return capText(sb.String(), s.opts.MaxResultChars)
}

// warnings renders RANGE warnings (with lines) and, after a successful eval,
// UNUSED_NODE for nodes built by the eval and never returned, awaited,
// looked at, shown, stored in mem or used as an input (owner).
func (s *state) warnings(vm *goja.Runtime, trace cvmod.EvalTrace, returned []graph.NodeID, lint bool) []string {
	var out []string
	for _, w := range trace.Warnings {
		out = append(out, s.lineTag(w.Code, w.Site)+w.Message)
	}
	if len(trace.Warnings) == 0 {
		for _, w := range cvmod.DrainWarnings(vm) {
			out = append(out, w.Code+": "+w.Message)
		}
	} else {
		cvmod.DrainWarnings(vm)
	}
	if lint {
		out = append(out, s.unusedNodes(vm, trace, returned)...)
	}
	return out
}

func (s *state) lineTag(code string, site cvmod.Site) string {
	if site.Source == "" {
		if l := s.agentLine(site.Line); l > 0 {
			return fmt.Sprintf("%s line %d: ", code, l)
		}
	}
	return code + ": "
}

func (s *state) unusedNodes(vm *goja.Runtime, trace cvmod.EvalTrace, returned []graph.NodeID) []string {
	if len(trace.Created) == 0 {
		return nil
	}
	roots := append(append([]graph.NodeID{}, trace.Used...), returned...)
	// nodes kept in mem or img count as used
	if mem, ok := vm.GlobalObject().Get("mem").(*goja.Object); ok {
		r := newRenderer(vm, s.eng)
		r.render(mem, 0)
		roots = append(roots, r.nodes...)
	}
	if id, ok := cvmod.NodeOf(s.img); ok {
		roots = append(roots, id)
	}
	used := map[graph.NodeID]bool{}
	var mark func(id graph.NodeID)
	mark = func(id graph.NodeID) {
		if used[id] {
			return
		}
		used[id] = true
		if n, ok := s.eng.Node(id); ok {
			for _, in := range n.Inputs {
				mark(in)
			}
		}
	}
	for _, id := range roots {
		mark(id)
	}
	// report only the last node of each discarded chain
	feeds := map[graph.NodeID]bool{}
	for _, c := range trace.Created {
		if used[c.ID] {
			continue
		}
		if n, ok := s.eng.Node(c.ID); ok {
			for _, in := range n.Inputs {
				feeds[in] = true
			}
		}
	}
	var out []string
	for _, c := range trace.Created {
		if used[c.ID] || feeds[c.ID] || s.onPlaceholder(c.ID, map[graph.NodeID]bool{}) {
			continue
		}
		what := s.sourceLine(s.agentLine(c.Site.Line))
		if c.Site.Source != "" || what == "" {
			what = cvmod.DescribeNode(s.eng, c.ID)
		}
		if len(what) > 80 {
			what = what[:77] + "…"
		}
		out = append(out, s.lineTag(graph.WUnusedNode, c.Site)+what+
			" builds a node and discards it. Nodes are lazy: nothing ran. Return it, await it, show() it, or keep it in mem.")
	}
	return out
}

// onPlaceholder reports whether node id is built on a builder-lambda
// placeholder (within(mask, i => i.exposure(-0.4)) builds i.exposure on a
// placeholder that Go substitutes); such nodes are intermediate, not discarded.
func (s *state) onPlaceholder(id graph.NodeID, seen map[graph.NodeID]bool) bool {
	if seen[id] {
		return false
	}
	seen[id] = true
	n, ok := s.eng.Node(id)
	if !ok {
		return false
	}
	if n.Op.On == graph.KindNone && n.Op.Name == "placeholder" {
		return true
	}
	for _, in := range n.Inputs {
		if s.onPlaceholder(in, seen) {
			return true
		}
	}
	return false
}

var syntaxPos = regexp.MustCompile(`Line (\d+):(\d+)\s*`)

const brokenMessage = "The eval was interrupted inside an async continuation, and the JavaScript runtime can no longer run promises (a goja limitation). Start a new session; values in mem are lost."

// probePromises reports whether vm still runs promise jobs. goja leaves the VM
// unable to run them when an interrupt hits a resumed async function (owner).
func probePromises(vm *goja.Runtime) bool {
	v, err := vm.RunString("(function () { let ok = false; Promise.resolve().then(function () { ok = true; }); return function () { return ok; }; })()")
	if err != nil {
		return false
	}
	fn, ok := goja.AssertFunction(v)
	if !ok {
		return false
	}
	res, err := fn(goja.Undefined())
	return err == nil && res.ToBoolean()
}

// errorNode renders the eval's failure in the shape of Part 2 §11 (owner).
func (s *state) errorNode(vm *goja.Runtime, evalErr error) *yaml.Node {
	f := s.failure
	msg := ""
	if evalErr != nil {
		msg = evalErr.Error()
	}
	if f == nil {
		f = &failure{Name: "Error", Message: strings.TrimPrefix(msg, "promise rejected: ")}
		switch {
		case strings.Contains(msg, "deadline exceeded") || strings.Contains(msg, "timed out") || strings.Contains(msg, "evaluation exceeded"):
			f.Code = graph.ETimeout
			f.Message = fmt.Sprintf("The eval ran longer than %s and was stopped.", s.opts.EvalTimeout)
			f.Hint = "Evaluate at preview scale, return less, or split the work over several evals."
		case strings.Contains(msg, "context canceled"):
			f.Code = graph.ECancelled
			f.Message = "The eval was cancelled."
		case strings.Contains(msg, "SyntaxError"):
			f.Name, f.Code = "SyntaxError", "SyntaxError"
			if m := syntaxPos.FindStringSubmatchIndex(msg); m != nil {
				f.Line, _ = strconv.Atoi(msg[m[2]:m[3]])
				f.Message = strings.TrimSpace(msg[m[1]:])
			} else {
				f.Message = strings.TrimPrefix(strings.TrimPrefix(msg, "SyntaxError: "), "SyntaxError: ")
			}
			f.Hint = "The code is the body of an async function: use await and return; the last expression is not returned."
		}
	}
	if f.Code == graph.ETimeout && !probePromises(vm) {
		s.broken = true
		f.Hint = brokenMessage
	}
	code := f.Code
	if code == "" {
		code = f.Name
	}
	if code == "" {
		code = "Error"
	}
	n := &yaml.Node{Kind: yaml.MappingNode}
	add := func(k string, v *yaml.Node) { n.Content = append(n.Content, str(k), v) }
	add("code", str(code))
	add("message", str(f.Message))
	if l := s.agentLine(f.Line); l > 0 {
		where := fmt.Sprintf("line %d", l)
		if src := s.sourceLine(l); src != "" {
			where += ": " + src
		}
		add("where", str(where))
	}
	if len(f.Path) > 0 {
		add("path", str(strings.Join(f.Path, " → ")))
	}
	if f.Hint == "" && !f.CV {
		f.Hint = jsHint(f)
	}
	if f.Hint != "" {
		add("hint", str(f.Hint))
	}
	if f.Fix != "" {
		add("fix", str(f.Fix))
	}
	if len(f.Valid) > 0 {
		v := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
		for _, x := range f.Valid {
			v.Content = append(v.Content, str(x))
		}
		add("valid", v)
	}
	return n
}

func jsHint(f *failure) string {
	switch {
	case strings.Contains(f.Message, "is not defined"):
		return "Top-level const/let do not survive between evals; keep values in mem (mem.x = …)."
	case strings.Contains(f.Message, "of undefined") || strings.Contains(f.Message, "of null"):
		return "A value is undefined; if it came from an earlier eval, keep it in mem."
	}
	return ""
}

// contextBlock is rebuilt for every eval: open images, params, mem (owner).
func (s *state) contextBlock(vm *goja.Runtime) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode}
	add := func(k, v string) {
		if v != "" {
			n.Content = append(n.Content, str(k), str(v))
		}
	}
	imgID, _ := cvmod.NodeOf(s.img)
	var images []string
	for _, id := range cvmod.Opened(vm) {
		node, ok := s.eng.Node(id)
		if !ok {
			continue
		}
		desc := fmt.Sprintf("%s (%d×%d)", filepath.Base(node.Args.String("path")), node.DocW, node.DocH)
		if id == imgID {
			desc = "img = " + desc
		}
		images = append(images, desc)
	}
	add("images", strings.Join(images, ", "))

	params := s.eng.Params().List()
	var ps, moved []string
	now := map[string]any{}
	for _, d := range params {
		now[d.Name] = d.Value
		item := fmt.Sprintf("%s = %s", d.Name, formatAny(d.Value))
		if d.Touched {
			item += " (moved by the person)"
		}
		ps = append(ps, item)
		if prev, ok := s.lastParams[d.Name]; ok && !reflect.DeepEqual(prev, d.Value) {
			moved = append(moved, fmt.Sprintf("moved %s %s → %s", d.Name, formatAny(prev), formatAny(d.Value)))
		}
	}
	s.lastParams = now
	add("params", strings.Join(ps, ", "))
	if len(moved) > 0 {
		add("person", "since your last eval: "+strings.Join(moved, "; "))
	}

	if mem, ok := vm.GlobalObject().Get("mem").(*goja.Object); ok {
		keys := mem.Keys()
		sort.Strings(keys)
		var items []string
		for _, k := range keys {
			items = append(items, fmt.Sprintf("%s (%s)", k, memKind(mem.Get(k))))
		}
		add("mem", strings.Join(items, ", "))
	}
	if len(n.Content) == 0 {
		return nil
	}
	return n
}

func memKind(v goja.Value) string {
	if v == nil || goja.IsUndefined(v) {
		return "undefined"
	}
	if goja.IsNull(v) {
		return "null"
	}
	obj, ok := v.(*goja.Object)
	if !ok {
		switch v.Export().(type) {
		case string:
			return "string"
		case bool:
			return "boolean"
		}
		return "number"
	}
	if id, ok := cvmod.NodeOf(obj); ok {
		return "#" + strconv.FormatUint(uint64(id), 10) + " " + nodeKind(obj, id)
	}
	if _, ok := cvmod.LookOf(obj); ok {
		return "look"
	}
	if _, ok := goja.AssertFunction(obj); ok {
		return "function"
	}
	if obj.ClassName() == "Array" {
		return fmt.Sprintf("array[%d]", obj.Get("length").ToInteger())
	}
	return "object"
}

func nodeKind(obj *goja.Object, _ graph.NodeID) string {
	if px, ok := obj.Export().(goja.Proxy); ok {
		if k := px.Target().Get("__kind"); k != nil {
			return k.String()
		}
	}
	return "node"
}

func formatAny(v any) string {
	switch x := graph.Normalize(v).(type) {
	case float64:
		return FormatNumber(x)
	case string:
		return strconv.Quote(x)
	case nil:
		return "null"
	}
	return fmt.Sprint(v)
}
