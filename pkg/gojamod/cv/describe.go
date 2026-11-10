package cvmod

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dop251/goja"
)

// Function signatures and doc lines (GCV-005 §4.2, §10.4, D7). goja's
// Function.prototype.toString returns a function's exact source, so a
// function's parameter list and the comment that opens its body can be read
// from the function itself, wherever it was defined:
//
//	mem.probe = async (x, y) => { // the best point within 2 px
//
// has the signature probe(x, y) and the doc line "the best point within 2 px".
// A comment above the function or in the middle of its body is not a doc
// line.

// maxDocRunes cuts a doc line.
const maxDocRunes = 120

// FunctionDoc is what DescribeFunction reads from a function's source.
type FunctionDoc struct {
	Params string // the parameter list as written, whitespace collapsed ("" for none)
	Doc    string // the doc line ("" when the body does not open with a comment)
	Native bool   // a native or bound function: no source
}

// Signature is name(params).
func (d FunctionDoc) Signature(name string) string { return name + "(" + d.Params + ")" }

// FunctionSource is fn's source text (Function.prototype.toString, which
// user code cannot replace on the function itself).
func FunctionSource(vm *goja.Runtime, fn *goja.Object) string {
	fp, ok := vm.Get("Function").(*goja.Object)
	if !ok {
		return ""
	}
	proto, ok := fp.Get("prototype").(*goja.Object)
	if !ok {
		return ""
	}
	ts, ok := goja.AssertFunction(proto.Get("toString"))
	if !ok {
		return ""
	}
	v, err := ts(fn)
	if err != nil {
		return ""
	}
	return v.String()
}

// DescribeFunction parses a function's source. ok is false for an accessor
// (get x() / set x(v)), which is not listed as a function.
func DescribeFunction(src string) (FunctionDoc, bool) {
	if strings.Contains(src, "[native code]") {
		return FunctionDoc{Native: true}, true
	}
	s := strings.TrimSpace(src)
	if rest, ok := cutWord(s, "async"); ok {
		s = rest
	}
	for _, w := range []string{"get", "set"} {
		if rest, ok := cutWord(s, w); ok && rest != "" && rest[0] != '(' {
			return FunctionDoc{}, false // an accessor (a method named get or set reads "get(…) {")
		}
	}
	var params string
	var ok bool
	var body int
	if rest, isFn := cutWord(s, "function"); isFn || strings.HasPrefix(s, "function(") {
		if !isFn {
			rest = s[len("function"):]
		}
		i := strings.IndexByte(rest, '(')
		if i < 0 {
			return FunctionDoc{}, true
		}
		if params, s, ok = parenthesized(rest[i:]); !ok {
			return FunctionDoc{}, true
		}
		body = openBrace(s)
	} else if strings.HasPrefix(s, "(") {
		if params, s, ok = parenthesized(s); !ok {
			return FunctionDoc{}, true
		}
		body = arrowBody(s)
	} else {
		s = strings.TrimLeft(s, "*") // a generator method
		id := identifier(s)
		after := strings.TrimLeft(s[len(id):], " \t\n")
		switch {
		case id != "" && strings.HasPrefix(after, "=>"):
			params, s = id, after
			body = arrowBody(s)
		case id != "" && strings.HasPrefix(after, "("):
			if params, s, ok = parenthesized(after); !ok {
				return FunctionDoc{}, true
			}
			body = openBrace(s)
		default:
			return FunctionDoc{}, true // a class, or something this parser does not know
		}
	}
	d := FunctionDoc{Params: collapse(params)}
	if body >= 0 {
		d.Doc = docComment(s[body:])
	}
	return d, true
}

// cutWord removes the keyword w and the whitespace after it.
func cutWord(s, w string) (string, bool) {
	if !strings.HasPrefix(s, w) || len(s) == len(w) {
		return s, false
	}
	r, _ := utf8.DecodeRuneInString(s[len(w):])
	if isIdent(r) {
		return s, false
	}
	return strings.TrimLeft(s[len(w):], " \t\n"), true
}

func isIdent(r rune) bool { return r == '_' || r == '$' || unicode.IsLetter(r) || unicode.IsDigit(r) }

func identifier(s string) string {
	for i, r := range s {
		if !isIdent(r) {
			return s[:i]
		}
	}
	return s
}

// parenthesized reads a balanced "( … )" at the start of s (strings,
// template literals and comments skipped) and returns its inside and the
// rest after it.
func parenthesized(s string) (string, string, bool) {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth == 0 {
				return s[1:i], s[i+1:], c == ')'
			}
		case '"', '\'', '`':
			i = skipString(s, i)
		case '/':
			if i+1 < len(s) && (s[i+1] == '/' || s[i+1] == '*') {
				i = skipComment(s, i)
			}
		}
	}
	return "", "", false
}

func skipString(s string, i int) int {
	q := s[i]
	for i++; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case q:
			return i
		}
	}
	return len(s)
}

func skipComment(s string, i int) int {
	if s[i+1] == '/' {
		if j := strings.IndexByte(s[i:], '\n'); j >= 0 {
			return i + j
		}
		return len(s)
	}
	if j := strings.Index(s[i+2:], "*/"); j >= 0 {
		return i + 2 + j + 1
	}
	return len(s)
}

// openBrace is the offset just after the first "{" of s (-1 if none).
func openBrace(s string) int {
	if i := strings.IndexByte(s, '{'); i >= 0 {
		return i + 1
	}
	return -1
}

// arrowBody is the offset where an arrow function's body starts: after
// "=>", and after its "{" when the body is a block.
func arrowBody(s string) int {
	i := strings.Index(s, "=>")
	if i < 0 {
		return -1
	}
	i += 2
	rest := strings.TrimLeft(s[i:], " \t\n")
	if strings.HasPrefix(rest, "{") {
		return len(s) - len(rest) + 1
	}
	return i
}

// docComment is the comment that is the first token of a body (only
// whitespace before it): a // comment's line, or the first non-empty line of
// a /* comment; cut at maxDocRunes.
func docComment(body string) string {
	b := strings.TrimLeft(body, " \t\r\n")
	var doc string
	switch {
	case strings.HasPrefix(b, "//"):
		doc, _, _ = strings.Cut(b[2:], "\n")
	case strings.HasPrefix(b, "/*"):
		inner, _, _ := strings.Cut(b[2:], "*/")
		for _, l := range strings.Split(inner, "\n") {
			if l = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(l), "*")); l != "" {
				doc = l
				break
			}
		}
	default:
		return ""
	}
	doc = strings.TrimSpace(doc)
	return CutRunes(doc, maxDocRunes)
}

// CutRunes cuts s to at most n runes, ending in "…" when cut.
func CutRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}

func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }
