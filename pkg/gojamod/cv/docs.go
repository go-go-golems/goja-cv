package cvmod

import (
	"strings"

	"github.com/dop251/goja"
	"github.com/go-go-golems/goja-cv/pkg/cv/docs"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// cv.docs natives: list, search and read the embedded documentation topics
// (pkg/cv/docs). They run on the owner goroutine and touch no engine state.

func (b *binding) addDocs(o *goja.Object) {
	_ = o.Set("__docsList", b.docsList)
	_ = o.Set("__docsSearch", b.docsSearch)
	_ = o.Set("__docsRead", b.docsRead)
}

// optionsArg exports an options object and rejects keys outside valid.
func (b *binding) optionsArg(v goja.Value, fn string, valid []string, fix string) map[string]any {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return map[string]any{}
	}
	m, ok := graph.Normalize(v.Export()).(map[string]any)
	if !ok {
		b.throw(graph.Errorf(graph.EArgType, "%s takes an options object; got %s.", fn, v.String()).WithFix(fix))
	}
	for _, k := range sortedKeys(m) {
		if !contains(valid, k) {
			e := graph.Errorf(graph.EUnknownOption, "%s has no option %q. Valid options: %s.", fn, k, strings.Join(valid, ", ")).WithValid(valid)
			if sug := graph.Closest(k, valid); sug != "" {
				e.Hint = "Did you mean \"" + sug + "\"?"
			}
			e.Fix = fix
			b.throw(e)
		}
	}
	return m
}

// __docsList() → [{id, title, summary}]
func (b *binding) docsList(goja.FunctionCall) goja.Value {
	list := docs.List()
	out := make([]any, len(list))
	for i, e := range list {
		out[i] = map[string]any{"id": e.ID, "title": e.Title, "summary": e.Summary}
	}
	return toJS(b.vm, out)
}

// __docsSearch(text, opts) → [{id, title, section, snippet, score}]
func (b *binding) docsSearch(c goja.FunctionCall) goja.Value {
	q := c.Argument(0)
	if goja.IsUndefined(q) || goja.IsNull(q) || strings.TrimSpace(q.String()) == "" {
		b.throw(graph.Errorf(graph.EArgType, "cv.docs.search needs search text.").WithFix(`cv.docs.search("heal dust", { limit: 5 })`))
	}
	opts := b.optionsArg(c.Argument(1), "cv.docs.search", []string{"limit"}, `cv.docs.search("heal dust", { limit: 5 })`)
	limit := 8
	if v, ok := opts["limit"]; ok {
		f, ok := v.(float64)
		if !ok || f < 1 || f != float64(int(f)) {
			b.throw(graph.Errorf(graph.EArgType, "cv.docs.search limit must be a positive integer.").WithFix(`cv.docs.search("heal dust", { limit: 5 })`))
		}
		limit = int(f)
	}
	hits := docs.Search(q.String(), limit)
	out := make([]any, len(hits))
	for i, h := range hits {
		out[i] = map[string]any{"id": h.ID, "title": h.Title, "section": h.Section, "snippet": h.Snippet, "score": h.Score}
	}
	return toJS(b.vm, out)
}

// __docsRead(id, opts) → Markdown
func (b *binding) docsRead(c goja.FunctionCall) goja.Value {
	id := c.Argument(0)
	if goja.IsUndefined(id) || goja.IsNull(id) {
		b.throw(graph.Errorf(graph.EArgType, "cv.docs.read needs a topic id.").WithValid(docs.IDs()).WithFix(`cv.docs.read("getting-started")`))
	}
	opts := b.optionsArg(c.Argument(1), "cv.docs.read", []string{"section"}, `cv.docs.read("recipes", { section: "Remove dust" })`)
	section := ""
	if v, ok := opts["section"]; ok {
		s, ok := v.(string)
		if !ok {
			b.throw(graph.Errorf(graph.EArgType, "cv.docs.read section must be a section title string.").WithFix(`cv.docs.read("recipes", { section: "Remove dust" })`))
		}
		section = s
	}
	md, err := docs.Read(id.String(), section)
	if err != nil {
		b.throw(err)
	}
	return b.vm.ToValue(md)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
