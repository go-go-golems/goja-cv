package cvmod

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/dop251/goja"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// cv.load and cv.scripts (GCV-005 §6.1, §7.2). The native side resolves,
// reads and compiles a script file; prelude.js calls the file's function, so
// the call stays JavaScript → JavaScript and the load stack is a JavaScript
// array. The compiled source is "(async function (args) {" + text + "\n})",
// on the file's first line, so line N of the source is line N of the file.

// ESyntax is the code of a script file that does not compile.
const ESyntax = "SyntaxError"

const (
	maxScriptBytes = 1 << 20
	maxLoadDepth   = 32
	loadWrapPrefix = "(async function (args) {"
)

// __compile(path) → { name, run }: the script file read through the
// workspace (then the read directories) and compiled as an async function
// of args.
func (b *binding) compile(c goja.FunctionCall) goja.Value {
	p := c.Argument(0)
	if goja.IsUndefined(p) || p.ExportType() == nil || p.ExportType().Kind().String() != "string" {
		b.throw(graph.Errorf(graph.EArgType, "cv.load needs a script path string.").WithFix(`await cv.load("dir/file.js")`))
	}
	path := p.String()
	if !strings.HasSuffix(path, ".js") {
		b.throw(graph.Errorf(graph.EArgType, "cv.load runs .js files; %q is not one.", path).WithFix(fmt.Sprintf("await cv.load(%q)", path+".js")))
	}
	if !graph.LoadableName(path) {
		b.throw(graph.Errorf(graph.EArgType, "cv.load runs .js files whose names have no spaces or parentheses; %q has some.", path).
			WithHint("Stack frames name the file; rename it."))
	}
	ws := b.eng.Workspace()
	if ws == nil {
		b.throw(graph.Errorf(graph.EAccess, "cv.load(%q): this runtime has no workspace to read in.", path))
	}
	f, loc, err := ws.Open(path)
	if err != nil {
		if ce, ok := graph.AsError(err); ok && ce.Code == graph.EArgType {
			ce.Hint = strings.TrimSpace(ce.Hint + " cv.scripts() lists the files cv.load can run.")
		}
		b.throw(err)
	}
	data, err := io.ReadAll(io.LimitReader(f, maxScriptBytes+1))
	_ = f.Close()
	if err != nil {
		b.throw(graph.Errorf(graph.EAccess, "cv.load(%q): %v", path, err))
	}
	if len(data) > maxScriptBytes {
		b.throw(graph.Errorf(graph.EArgType, "%s is too large for a script (over 1 MiB).", path))
	}
	name := ws.ScriptName(loc)
	text := string(data)
	prg, err := goja.Compile(name, loadWrapPrefix+text+"\n})", false)
	if err != nil {
		b.throw(&graph.Error{Code: ESyntax, Message: syntaxMessage(name, text, err),
			Hint: "A script file is the body of an async function: top-level await and return work, args holds cv.load's second argument."})
	}
	b.mu.Lock()
	if b.files == nil {
		b.files = map[string][]string{}
	}
	b.files[name] = strings.Split(text, "\n")
	b.mu.Unlock()
	fn, err := b.vm.RunProgram(prg)
	if err != nil {
		b.throw(err)
	}
	o := b.vm.NewObject()
	_ = o.Set("name", name)
	_ = o.Set("run", fn)
	return o
}

// syntaxMessage is "name:line:col: message" for a compile error of a script
// file, in the file's own lines and columns. goja reports
// "name: Line L:C message (and N more errors)".
func syntaxMessage(name, text string, err error) string {
	msg := err.Error()
	var se *goja.CompilerSyntaxError
	if errors.As(err, &se) {
		msg = se.Message
	}
	m := syntaxPos.FindStringSubmatch(msg)
	if m == nil {
		return name + ": " + strings.TrimPrefix(msg, name+": ")
	}
	line, _ := strconv.Atoi(m[1])
	col, _ := strconv.Atoi(m[2])
	msg = strings.TrimSpace(syntaxMore.ReplaceAllString(m[3], ""))
	if line == 1 {
		col -= len(loadWrapPrefix)
	}
	if n := strings.Count(text, "\n") + 1; line > n { // the wrapper's closing line: the file ended early
		return fmt.Sprintf("%s:%d: %s at the end of the file (an unclosed bracket, string or comment?)", name, n, msg)
	}
	return fmt.Sprintf("%s:%d:%d: %s", name, line, col, msg)
}

var (
	syntaxPos  = regexp.MustCompile(`Line (\d+):(\d+) (.*)$`)
	syntaxMore = regexp.MustCompile(`\(and \d+ more errors?\)`)
)

// __pending(promise): a load (or save) the current eval waits for before it
// ends.
func (b *binding) pending(c goja.FunctionCall) goja.Value {
	b.mu.Lock()
	if b.track.active {
		b.track.pending = append(b.track.pending, c.Argument(0))
	}
	b.mu.Unlock()
	return goja.Undefined()
}

// __tooDeep(stack) throws E_TOO_MANY naming the load stack.
func (b *binding) tooDeep(c goja.FunctionCall) goja.Value {
	var names []string
	if arr, ok := c.Argument(0).Export().([]any); ok {
		for _, x := range arr {
			names = append(names, fmt.Sprint(x))
		}
	}
	shown := names
	if len(shown) > 6 {
		shown = append(append([]string(nil), shown[:6]...), "…")
	}
	b.throw(graph.Errorf(graph.ETooMany, "cv.load is nested %d deep: %s; does a file load itself?", maxLoadDepth, strings.Join(shown, " → ")).
		WithHint("A file that loads another before its first await nests; loads started in parallel do not count."))
	return nil
}

// __scripts() → [{ path, doc }]: the .js files cv.load can run.
func (b *binding) scripts(goja.FunctionCall) goja.Value {
	ws := b.eng.Workspace()
	if ws == nil {
		return b.vm.NewArray()
	}
	list := ws.Scripts()
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = graph.Record{Keys: []string{"path", "doc"}, Vals: map[string]any{"path": s.Path, "doc": s.Doc}}
	}
	return toJS(b.vm, out)
}

// FileLine is line l (1-based) of the script file last loaded under name in
// vm, and whether name is a loaded file.
func FileLine(vm *goja.Runtime, name string, l int) (string, bool) {
	b, ok := lookupBinding(vm)
	if !ok {
		return "", false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	lines, ok := b.files[name]
	if !ok {
		return "", false
	}
	if l < 1 || l > len(lines) {
		return "", true
	}
	return lines[l-1], true
}

// stackFrame matches one position of a stack trace; script names have no
// whitespace or parentheses (graph.LoadableName).
var stackFrame = regexp.MustCompile(`([^\s()]+):(\d+):(\d+)`)

// Frame is a position in an eval ("<eval>") or a loaded script file.
type Frame struct {
	Source string // "" for eval code, else the script's name
	Line   int
	Column int
}

// StackFrames are the positions of a stack trace that lie in eval code or
// in a script file loaded in vm, innermost first.
func StackFrames(vm *goja.Runtime, stack string) []Frame {
	var out []Frame
	for _, m := range stackFrame.FindAllStringSubmatch(stack, -1) {
		src := m[1]
		if src == "<eval>" {
			src = ""
		} else if _, ok := FileLine(vm, src, 0); !ok {
			continue
		}
		l, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		out = append(out, Frame{Source: src, Line: l, Column: col})
	}
	return out
}
