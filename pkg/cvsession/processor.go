package cvsession

import (
	"context"
	"fmt"
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
// looks created before the error).
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
// and makes ctx (the eval's deadline) the context of the
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
	providers, dropped, lookWarnings := st.collect(vm, extraction.Providers, trace)

	r := newRenderer(vm, st.eng)
	r.tableLeft = st.opts.MaxResultChars * 2 / 3
	r.providers = providerNumbers(providers)
	r.dropped = dropped
	var sb strings.Builder
	if value == nil || goja.IsUndefined(value) {
		sb.WriteString("# nothing returned: end the code with `return …` to see a value (the last expression is not returned).")
	} else {
		sb.WriteString(encodeYAML(r.render(value, 0, "")))
	}
	warnings := append(st.warnings(vm, trace, r.nodes, true), lookWarnings...)
	return replsession.ProcessedResult{Text: st.finish(vm, sb.String(), warnings), Providers: providers}, nil
}

// ProcessFailure implements replsession.FailureProcessor: the structured
// error in YAML, warnings and context; the looks created before the error
// are delivered.
func (p *ResultProcessor) ProcessFailure(_ context.Context, vm *goja.Runtime, evalErr error) (replsession.ProcessedResult, error) {
	st, ok := stateOf(vm)
	if !ok {
		return replsession.ProcessedResult{}, nil
	}
	st.watchdog.disarm(vm)
	trace := cvmod.EndEval(vm)
	providers, _, lookWarnings := st.collect(vm, nil, trace)
	var sb strings.Builder
	sb.WriteString(encodeYAML(mapping(str("error"), st.errorNode(vm, evalErr, trace))))
	warnings := append(st.warnings(vm, trace, nil, false), lookWarnings...)
	return replsession.ProcessedResult{Text: st.finish(vm, sb.String(), warnings), Providers: providers}, nil
}

// collect decides what an eval delivers (owner). Every look the eval
// created is delivered, whether or not it was returned: the looks found in
// the returned value first, then the others in creation order; equal looks
// (the same node with equal options) once. At most MaxLooks are delivered;
// the others are not rendered, one LOOKS warning names each by line, and a
// returned one keeps a note in its placeholder. Looks created by a prelude
// are discarded (preludes have no output). It numbers the delivered looks
// and returns the LOOKS warnings (the v1 texts, GCV-004 Phase 0a).
func (s *state) collect(vm *goja.Runtime, fromValue []resultcontent.Provider, trace cvmod.EvalTrace) ([]resultcontent.Provider, map[resultcontent.Provider]string, []string) {
	if s.prelude.Load() {
		return nil, nil, nil
	}
	sites := map[*look.Look]cvmod.Site{}
	cands := append([]resultcontent.Provider(nil), fromValue...)
	for _, q := range trace.Looks {
		sites[q.Look] = q.Site
		cands = append(cands, q.Look)
	}
	out := make([]resultcontent.Provider, 0, len(cands))
	dropped := map[resultcontent.Provider]string{}
	seen := map[resultcontent.Provider]bool{}
	byKey := map[string]*look.Look{}
	alias := map[*look.Look]*look.Look{}
	var undelivered []*look.Look
	looks := 0
	for _, p := range cands {
		switch x := p.(type) {
		case *look.Look:
			if seen[x] {
				continue
			}
			seen[x] = true
			if first, ok := byKey[x.Key()]; ok {
				alias[x] = first
				continue
			}
			byKey[x.Key()] = x
			if looks++; looks > s.opts.MaxLooks {
				dropped[x] = fmt.Sprintf("more than %d looks in one eval; return fewer, or one cv.sheet of many crops", s.opts.MaxLooks)
				undelivered = append(undelivered, x)
				continue
			}
		}
		out = append(out, p)
	}
	for _, p := range out {
		if x, ok := p.(*look.Look); ok {
			s.number(x)
		}
	}
	for l, first := range alias {
		if why, ok := dropped[first]; ok {
			dropped[l] = why
		} else {
			l.AssignNumber(first.Number())
		}
	}
	var warnings []string
	if len(undelivered) > 0 {
		names := make([]string, len(undelivered))
		for i, l := range undelivered {
			names[i] = s.lookName(vm, l, sites[l])
		}
		warnings = append(warnings, s.lineTag(vm, graph.WLooks, sites[undelivered[0]])+fmt.Sprintf("%d %s not delivered (at most %d per eval): %s. Split the work over several calls, or return fewer looks (one cv.sheet of many crops).",
			len(undelivered), plural(len(undelivered), "look was", "looks were"), s.opts.MaxLooks, strings.Join(names, ", ")))
	}
	for _, q := range trace.Late {
		warnings = append(warnings, fmt.Sprintf("%s: %s was created after the previous eval had returned (in a callback that ran later) and was not delivered. Await what it needs inside the eval.",
			graph.WLooks, lookCall(q.Look)))
	}
	return out, dropped, warnings
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// notDelivered is the placeholder of a look left out by the budget.
func notDelivered(l *look.Look, why string) string {
	return "[not delivered: " + strings.TrimSuffix(strings.TrimPrefix(l.String(), "["), "]") + " — " + why + "]"
}

// lookName names a look in a warning: "line 7 look("b")".
func (s *state) lookName(vm *goja.Runtime, l *look.Look, site cvmod.Site) string {
	if t := s.siteTag(vm, site); t != "" {
		return t + " " + lookCall(l)
	}
	return lookCall(l)
}

func lookCall(l *look.Look) string {
	if label := l.AgentLabel(); label != "" {
		return fmt.Sprintf("look(%q)", label)
	}
	return "look()"
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
		out = append(out, s.lineTag(vm, w.Code, w.Site)+w.Message)
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
	return dedupe(out)
}

// dedupe folds identical warnings (a call in a loop) into one with a count.
func dedupe(ws []string) []string {
	count := map[string]int{}
	var order []string
	for _, w := range ws {
		if count[w] == 0 {
			order = append(order, w)
		}
		count[w]++
	}
	out := make([]string, len(order))
	for i, w := range order {
		out[i] = w
		if n := count[w]; n > 1 {
			out[i] = fmt.Sprintf("%s (×%d)", w, n)
		}
	}
	return out
}

// evalWhere is "line N: source" for a line of the evaluated source ("" when
// it is outside the agent's code).
func (s *state) evalWhere(evalLine int) string {
	l := s.agentLine(evalLine)
	if l <= 0 {
		return ""
	}
	where := fmt.Sprintf("line %d", l)
	if src := s.sourceLine(l); src != "" {
		where += ": " + src
	}
	return where
}

// siteTag is "line N" for a site in the eval's code, "name:N" for a site
// in a loaded script file, "" when unknown.
func (s *state) siteTag(vm *goja.Runtime, site cvmod.Site) string {
	if site.Source == "" {
		if l := s.agentLine(site.Line); l > 0 {
			return fmt.Sprintf("line %d", l)
		}
		return ""
	}
	if _, ok := cvmod.FileLine(vm, site.Source, 0); ok && site.Line > 0 {
		return fmt.Sprintf("%s:%d", site.Source, site.Line)
	}
	return ""
}

func (s *state) lineTag(vm *goja.Runtime, code string, site cvmod.Site) string {
	if t := s.siteTag(vm, site); t != "" {
		return code + " " + t + ": "
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
		r.render(mem, 0, "")
		roots = append(roots, r.nodes...)
	}
	roots = append(roots, s.namedNodes()...)
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
		out = append(out, s.lineTag(vm, graph.WUnusedNode, c.Site)+what+
			" builds a node and discards it. Nodes are lazy: nothing ran. Return it, await it, look at it, or keep it in mem.")
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
func (s *state) errorNode(vm *goja.Runtime, evalErr error, trace cvmod.EvalTrace) *yaml.Node {
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
			f.Hint = "Crop before measuring, return less, or split the work over several evals."
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
	s.explainDataMisuse(f)
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
	if f.Source != "" {
		where := fmt.Sprintf("%s:%d", f.Source, f.Line)
		if src, _ := cvmod.FileLine(vm, f.Source, f.Line); strings.TrimSpace(src) != "" {
			where += ": " + strings.TrimSpace(src)
		}
		add("where", str(where))
		if called := s.evalWhere(f.CalledFrom); called != "" {
			add("called from", str(called))
		}
	} else if where := s.evalWhere(f.Line); where != "" {
		add("where", str(where))
	}
	if len(f.Path) > 0 {
		add("path", str(strings.Join(f.Path, " → ")))
	}
	if f.Hint == "" && !f.CV && f.Source == "" {
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

var noMember = regexp.MustCompile(`Object has no member '([A-Za-z_$][\w$]*)'`)

// receiverOf finds the expression a member was called on in a source line
// ("rows" in "return { first: rows.slice(0, 3) }").
func receiverOf(src, member string) string {
	re := regexp.MustCompile(`([A-Za-z_$][\w$]*(?:\.[A-Za-z_$][\w$]*)*)\.` + regexp.QuoteMeta(member) + `\s*\(`)
	if m := re.FindStringSubmatch(src); m != nil {
		return m[1]
	}
	return ""
}

// explainDataMisuse explains a raw TypeError from calling an op on plain
// data (an awaited array or object): the op belongs to a node, and an op
// that takes regions takes the array as its argument.
func (s *state) explainDataMisuse(f *failure) {
	src := s.sourceLine(s.agentLine(f.Line))
	if f.CV || f.Name != "TypeError" || src == "" || f.Source != "" {
		return
	}
	m := noMember.FindStringSubmatch(f.Message)
	if m == nil {
		return
	}
	recv := receiverOf(src, m[1])
	if recv == "" {
		return
	}
	if m[1] == "count" {
		f.Hint = fmt.Sprintf("%s is plain data here (awaited regions are an array), so it has no count(); its length is the count.", recv)
		f.Fix = recv + ".length"
		return
	}
	for _, op := range s.eng.Registry().Find(m[1]) {
		if op.On != graph.KindImage {
			continue
		}
		f.Hint = fmt.Sprintf("%s is plain data here, not a node, so it has no %s(). %q is an Image method.", recv, m[1], m[1])
		f.Fix = fmt.Sprintf("img.%s(…)", m[1])
		for _, a := range op.Args {
			if a.Positional && strings.Contains(a.Type, graph.ItemsType) {
				f.Hint += " Regions are its argument: pass the array."
				f.Fix = fmt.Sprintf("img.%s(%s, …)", m[1], recv)
				break
			}
		}
		return
	}
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

// contextBlock is rebuilt for every eval: named and unnamed images, the
// keys of mem with a kind each, the functions on mem with short signatures,
// and the signature and doc line of each function whose name is new or
// whose source changed since the last agent eval (owner). mem is read
// through property descriptors: getters are not run. A prelude's eval
// neither reads nor updates what has been shown (its output is never seen).
func (s *state) contextBlock(vm *goja.Runtime) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode}
	add := func(k string, v *yaml.Node) {
		if v.Value != "" {
			n.Content = append(n.Content, str(k), v)
		}
	}
	add("images", str(s.imagesLine(vm)))

	if mem, ok := vm.GlobalObject().Get("mem").(*goja.Object); ok {
		r := newRenderer(vm, s.eng)
		keys := mem.Keys()
		sort.Strings(keys)
		var items, short, docs []string
		seen := map[string]string{}
		for _, k := range keys {
			v, data := r.dataProperty(mem, k)
			if !data {
				items = append(items, k+" (getter)")
				continue
			}
			fn, isFn := v.(*goja.Object)
			if isFn {
				_, isFn = goja.AssertFunction(fn)
			}
			if !isFn {
				items = append(items, fmt.Sprintf("%s (%s)", k, memKind(v)))
				continue
			}
			src := cvmod.FunctionSource(vm, fn)
			d, listed := cvmod.DescribeFunction(src)
			if !listed {
				items = append(items, k+" (getter)")
				continue
			}
			name := "mem." + k
			sig := d.Signature(name)
			if d.Native {
				sig = name + "()"
			}
			short = append(short, cvmod.CutRunes(sig, maxShortSignature))
			seen[name] = src
			if !s.prelude.Load() && s.fnSeen[name] != src {
				if d.Doc != "" {
					sig += ": " + d.Doc
				}
				docs = append(docs, sig)
			}
		}
		add("mem", str(strings.Join(items, ", ")))
		add("functions", str(strings.Join(short, ", ")))
		if len(docs) > 0 {
			add("new functions", &yaml.Node{Kind: yaml.ScalarNode, Style: yaml.LiteralStyle, Tag: "!!str", Value: strings.Join(docs, "\n")})
		}
		if !s.prelude.Load() {
			s.fnSeen = seen
		}
	}
	if len(n.Content) == 0 {
		return nil
	}
	return n
}

// maxShortSignature cuts the signatures of the context block's functions line.
const maxShortSignature = 40

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
