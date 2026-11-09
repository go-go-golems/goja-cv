package cvmod

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/docs"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// cv.help(name) never throws: agents batch several help calls in one eval,
// and one bad name used to kill the whole eval. It accepts op names
// ("levels", "mask.box"), kind names ("Mask" → its methods), Kind.method
// ("Mask.coverage", "Regions.filter"), cv functions and globals ("explain",
// "compare", "docs", "mem"), with or without a "cv." prefix.
// Anything else returns { found: false, name, didYouMean, hint }.

type cvFunc struct {
	usage, doc string
	see        []string
}

// cvFuncs are help entries for what is neither an op nor a registered
// function: cv properties, session globals and concepts.
var cvFuncs = map[string]cvFunc{
	"version":  {"cv.version", "The cv module version.", nil},
	"mem":      {"mem.name = value", "A plain object that persists between evals (const/let do not).", nil},
	"img":      {"img · img = cv.open(path)", "The open image (an Image node); assigning a cv.open node rebinds it. More images: cv.open(path, { as: \"ref\" }).", []string{"open", "names"}},
	"encoding": {"cv.open(path, { encoding: \"srgb\" | \"linear\" }) · (await img).encoding · img.encode(\"srgb\")", "How a file's values relate to the quantity: srgb (photographs) or linear (float TIFF, DNG, .npy, density data). Decoders report it; cv.open's option overrides it. Read cv.docs.read(\"encodings-and-spaces\").", []string{"open", "encode", "space"}},
	"space":    {"op({ space: \"linear\" | \"encoded\" })", "The working space of a pixel op: linear (the encoding undone: exposure, blur, blend, stats, levels) or encoded (what looks show: thresholds, contrast, histogram, specks). cv.explain(node) shows the conversions. Read cv.docs.read(\"encodings-and-spaces\").", []string{"stats", "levels", "encoding"}},
	"frame":    {"(await img).frame · cv.open(path, { box, origin })", "The rectangle an image covers in doc px, [x, y, w, h]; boxes, points and region geometry are absolute in it. Read cv.docs.read(\"units-and-scales\", { section: \"Frames\" }).", []string{"open"}},
	"names":    {"cv.open(path, { as: \"ref\" })", "Several images per session: as binds a global that persists between evals (img is the first); the context block lists every name with file, frame and encoding. Read cv.docs.read(\"encodings-and-spaces\", { section: \"Frames and names\" }).", []string{"open", "img"}},
}

// helpRedirects map names agents reach for to the call that does it.
var helpRedirects = map[string][2]string{
	"mask.stats":    {"stats", "Measure inside a mask with img.stats({ mask: m }); inside a box with img.stats({ box: [x, y, w, h] })."},
	"Mask.stats":    {"stats", "Measure inside a mask with img.stats({ mask: m })."},
	"Regions.stats": {"stats", "Measure inside regions with img.stats({ mask: regions.toMask() })."},
	"mask.mean":     {"stats", "img.stats({ mask: m }).meanL is the mean inside a mask."},
	"metadata":      {"open", "Await the image for its metadata: (await img) has file, format, bitDepth, sample, fileChannels, encoding, frame and notes."},
	"meta":          {"open", "Await the image for its metadata: (await img) has file, format, bitDepth, sample, fileChannels, encoding, frame and notes."},
	"info":          {"open", "Await the image for its metadata: (await img) has file, format, bitDepth, sample, fileChannels, encoding, frame and notes."},
}

// Help returns cv.help(name) data; it never fails.
func Help(reg *graph.Registry, name string) map[string]any {
	name = strings.TrimSpace(name)
	if name == "" {
		h, _ := reg.Help("")
		h["sources"] = toAny([]string{"cv.open(path, { encoding, box, origin, as })"})
		h["functions"] = toAny(functionNames(reg))
		h["docs"] = "Guides: cv.docs.list(), cv.docs.search(text), cv.docs.read(id, { section }). Start with cv.docs.read(\"getting-started\")."
		h["topics"] = toAny(docs.IDs())
		h["usage"] = "cv.help(name) takes an op (\"levels\"), a kind (\"Mask\"), Kind.method (\"Mask.coverage\") or a cv function (\"explain\"). Calls build lazy nodes; await a node to compute it; node.look() shows it."
		return h
	}
	if r, ok := helpRedirects[name]; ok {
		return notFound(reg, name, []string{r[0]}, r[1])
	}
	name = strings.TrimPrefix(name, "cv.")
	// exact op names first: "mask.box" is an op, not Mask.box
	if strings.Contains(name, ".") && len(reg.Find(name)) > 0 {
		return opHelp(reg, name, "")
	}
	// a kind: its methods with one line of doc each
	if k, ok := kindNamed(name); ok {
		return kindHelp(reg, k)
	}
	// Kind.method
	if i := strings.Index(name, "."); i > 0 {
		if k, ok := kindNamed(name[:i]); ok {
			method := name[i+1:]
			if op, ok := reg.Lookup(k, method); ok {
				return opHelp(reg, op.Name, k)
			}
			for _, f := range reg.FuncsOn(k) {
				if f.Name == method {
					h, _ := reg.FuncHelp(method)
					return h
				}
			}
			e := reg.UnknownMethod(k, method, nil)
			hint := e.Message
			if e.Hint != "" {
				hint += " " + e.Hint
			}
			out := notFound(reg, name, suggest(reg, method, k), hint+fmt.Sprintf(" cv.help(%q) lists its methods.", string(k)))
			if e.Fix != "" {
				out["fix"] = e.Fix
			}
			return out
		}
	}
	if h, ok := reg.FuncHelp(name); ok {
		return h
	}
	if f, ok := cvFuncs[name]; ok {
		return funcHelp(name, f)
	}
	if len(reg.Find(name)) > 0 {
		return opHelp(reg, name, "")
	}
	return notFound(reg, name, suggest(reg, name, ""), "")
}

// functionNames lists the module ops and registered functions as they are
// called: cv.compare, cv.explain, …, node.look.
func functionNames(reg *graph.Registry) []string {
	var out []string
	for _, n := range reg.ModuleOps() {
		out = append(out, "cv."+n)
	}
	for _, f := range reg.Funcs() {
		out = append(out, f.CallName())
	}
	sort.Strings(out)
	return out
}

// helpKinds are the kind names help resolves.
var helpKinds = []graph.Kind{graph.KindImage, graph.KindMask, graph.KindRegions, graph.KindValue}

func kindNamed(s string) (graph.Kind, bool) {
	for _, k := range helpKinds {
		if strings.EqualFold(s, string(k)) {
			return k, true
		}
	}
	return "", false
}

func kindHelp(reg *graph.Registry, k graph.Kind) map[string]any {
	methods := map[string]any{}
	for _, m := range reg.Methods(k) {
		op, ok := reg.Lookup(k, m)
		if !ok {
			continue
		}
		methods[m] = oneLine(op.Doc)
	}
	for _, f := range reg.FuncsOn(k) {
		methods[f.Name] = oneLine(f.Doc)
	}
	out := map[string]any{"kind": string(k), "methods": methods, "usage": fmt.Sprintf("cv.help(\"%s.<method>\") or cv.help(\"<method>\") describes one.", k)}
	if k == graph.KindRegions {
		out["fields"] = toAny(graph.FieldNames(graph.GeometryFields))
		out["items"] = "Awaited Regions are { count, items, frame }; every item has these flat fields (x, y, w, h: the bounding box; cx, cy: the centroid; doc px) plus the measures of the op that made it (cv.help(\"detect.specks\").fields). Pass an item where a box, a point or an id is expected."
	}
	return out
}

func opHelp(reg *graph.Registry, name string, k graph.Kind) map[string]any {
	h, err := reg.Help(name)
	if err != nil {
		return notFound(reg, name, nil, "")
	}
	if k != "" { // Kind.method: describe that kind's op only
		if entry, ok := h[name].(map[string]any); ok {
			if _, multi := entry["on"].([]any); multi {
				entry["on"] = string(k)
			}
		}
	}
	return h
}

func funcHelp(name string, f cvFunc) map[string]any {
	entry := map[string]any{"on": "cv", "usage": f.usage, "doc": f.doc}
	if len(f.see) > 0 {
		entry["see"] = toAny(f.see)
	}
	return map[string]any{name: entry}
}

func notFound(reg *graph.Registry, name string, sug []string, hint string) map[string]any {
	if hint == "" {
		switch {
		case len(sug) > 0:
			hint = fmt.Sprintf("Did you mean %q? cv.help() lists every op; cv.docs.search(%q) searches the guides.", sug[0], name)
		default:
			hint = fmt.Sprintf("cv.help() lists every op; cv.docs.search(%q) searches the guides.", name)
		}
	}
	if sug == nil {
		sug = []string{}
	}
	return map[string]any{"found": false, "name": name, "didYouMean": toAny(sug), "hint": hint}
}

func oneLine(doc string) string {
	if i := strings.Index(doc, ". "); i > 0 {
		return doc[:i+1]
	}
	return doc
}

// suggest ranks candidate names for an unknown help name: the synonym
// table, the last dotted segment as an op, then edit distance over every
// op, kind and cv function (preferring k's methods).
func suggest(reg *graph.Registry, name string, k graph.Kind) []string {
	var out []string
	add := func(s string) {
		if s == "" {
			return
		}
		for _, x := range out {
			if x == s {
				return
			}
		}
		out = append(out, s)
	}
	last := name
	if i := strings.LastIndex(name, "."); i >= 0 {
		last = name[i+1:]
	}
	if t, ok := graph.Synonyms[last]; ok && len(reg.Find(t)) > 0 {
		add(t)
	} else {
		syn := make([]string, 0, len(graph.Synonyms))
		for s := range graph.Synonyms {
			syn = append(syn, s)
		}
		sort.Strings(syn)
		if s := graph.Closest(last, syn); s != "" && len(reg.Find(graph.Synonyms[s])) > 0 {
			add(graph.Synonyms[s]) // "blurr" → blur → gaussian
		}
	}
	if last != name && len(reg.Find(last)) > 0 {
		add(last)
	}
	var cands []string
	seen := map[string]bool{}
	if k != "" {
		if m := graph.Closest(last, reg.Methods(k)); m != "" {
			add(string(k) + "." + m)
		}
	}
	for _, op := range reg.Ops() {
		if !seen[op.Name] {
			seen[op.Name] = true
			cands = append(cands, op.Name)
		}
	}
	for f := range cvFuncs {
		cands = append(cands, f)
	}
	for _, f := range reg.Funcs() {
		cands = append(cands, f.Name)
	}
	for _, kk := range helpKinds {
		cands = append(cands, string(kk))
	}
	sort.Strings(cands)
	for len(out) < 3 {
		c := graph.Closest(name, cands)
		if c == "" {
			if last != name {
				c = graph.Closest(last, cands)
			}
			if c == "" {
				break
			}
		}
		add(c)
		cands = remove(cands, c)
	}
	return out
}

func remove(xs []string, x string) []string {
	out := xs[:0:0]
	for _, e := range xs {
		if e != x {
			out = append(out, e)
		}
	}
	return out
}

// memberFixes are literal fixes for names agents call on cv that are not
// functions there.
var memberFixes = map[string]string{
	"info":     "await img   // or cv.explain(img)",
	"meta":     "await img   // or cv.explain(img)",
	"metadata": "await img   // or cv.explain(img)",
	"load":     `img = cv.open("scan.tif")`,
	"read":     `img = cv.open("scan.tif")`,
	"look":     "node.look(\"label\")",
}

// UnknownMember is the structured error for obj.name when the cv module
// object ("cv") has no such member: a did-you-mean
// over the members, an op of that name on a node kind, or a help redirect.
func UnknownMember(reg *graph.Registry, obj, name string, members []string) *graph.Error {
	sort.Strings(members)
	e := graph.Errorf(graph.EUnknownMethod, "%s has no %q.", obj, name).WithValid(members)
	if fix, ok := memberFixes[name]; ok && obj == "cv" {
		e.Fix = fix
		if r, ok := helpRedirects[name]; ok {
			e.Hint = r[1]
		} else {
			e.Hint = fmt.Sprintf("%q is not a cv function.", name)
		}
		return e
	}
	if sug := graph.Closest(name, members); sug != "" {
		e.Hint = fmt.Sprintf("Did you mean %q?", sug)
		e.Fix = fmt.Sprintf("%s.%s(…)", obj, sug)
		return e
	}
	if obj == "cv" {
		if r, ok := graph.MethodRedirects[name]; ok {
			e.Hint, e.Fix = r.Hint, r.Fix
			return e
		}
		if t, ok := graph.Synonyms[name]; ok && contains(members, t) { // cv.grid → cv.sheet
			e.Hint = fmt.Sprintf("Did you mean %q?", t)
			e.Fix = fmt.Sprintf("cv.%s(…)", t)
			return e
		}
		if t, ok := graph.Synonyms[name]; ok && len(reg.Find(t)) > 0 {
			name = t // a synonym of a node method: say where it lives
		}
		for _, op := range reg.Find(name) {
			if op.On == graph.KindNone {
				continue
			}
			recv := map[graph.Kind]string{graph.KindImage: "img", graph.KindMask: "mask", graph.KindRegions: "regions", graph.KindValue: "value"}[op.On]
			if recv == "" {
				recv = "node"
			}
			e.Hint = fmt.Sprintf("%q is %s method: call it on the node.", name, graph.A(string(op.On)))
			e.Fix = fmt.Sprintf("%s.%s(…)", recv, name)
			return e
		}
	}
	e.Hint = fmt.Sprintf("%s members: %s. cv.help(%q) and cv.docs.search(%q) look it up.", obj, strings.Join(members, ", "), name, name)
	return e
}
