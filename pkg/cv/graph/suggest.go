package graph

import (
	"fmt"
	"strings"
)

// Synonyms maps names agents commonly reach for to cv op names. Entries are
// only suggested when the target exists on the receiver kind or elsewhere.
var Synonyms = map[string]string{
	"brightness":    "exposure",
	"brighten":      "exposure",
	"ev":            "exposure",
	"exposureEV":    "exposure",
	"blur":          "gaussian",
	"gaussianBlur":  "gaussian",
	"gaussBlur":     "gaussian",
	"smooth":        "gaussian",
	"grayscale":     "toGray",
	"greyscale":     "toGray",
	"gray":          "toGray",
	"grey":          "toGray",
	"desaturate":    "toGray",
	"toGrey":        "toGray",
	"mean":          "stats",
	"average":       "stats",
	"statistics":    "stats",
	"stat":          "stats",
	"probe":         "measure",
	"ring":          "measure",
	"contrastAt":    "measure",
	"valueAt":       "sample",
	"pixel":         "sample",
	"at":            "sample",
	"lineProfile":   "profile",
	"line":          "profile",
	"trace":         "profile",
	"hist":          "histogram",
	"histo":         "histogram",
	"clone":         "heal",
	"spotHeal":      "heal",
	"autoLevels":    "levels",
	"tone":          "levels",
	"describe":      "explain",
	"graph":         "explain",
	"lab":           "toLab",
	"toLAB":         "toLab",
	"saturation":    "chroma",
	"channelSpread": "chroma",
	"falseColor":    "colormap",
	"falseColour":   "colormap",
	"lut":           "colormap",
	"heatmap":       "colormap",
	"reencode":      "encode",
	"channels":      "split",
	// GCV-002
	"subtract":     "sub",
	"difference":   "sub",
	"diff":         "sub",
	"multiply":     "mul",
	"times":        "mul",
	"hp":           "highpass",
	"highPass":     "highpass",
	"residual":     "highpass",
	"DoG":          "dog",
	"bandpass":     "dog",
	"clahe":        "localContrast",
	"localNorm":    "localContrast",
	"normalize":    "levels",
	"stretch":      "levels",
	"window":       "crop",
	"roi":          "crop",
	"subimage":     "crop",
	"marks":        "annotate",
	"markers":      "annotate",
	"draw":         "annotate",
	"mark":         "annotate",
	"contactSheet": "sheet",
	"grid":         "sheet",
	"mosaic":       "sheet",
	"montage":      "sheet",
	"preset":       "view",
	"views":        "view",
}

// MethodRedirects are method names that are not ops but look options or
// other calls: the error's hint and literal fix say what to write.
var MethodRedirects = map[string]Redirect{
	"zoom":    {Hint: "Zoom is a look option, not a node: crop the window, then look at it zoomed.", Fix: "node.crop([x, y, w, h]).look({ zoom: 8 })"},
	"magnify": {Hint: "Zoom is a look option, not a node: crop the window, then look at it zoomed.", Fix: "node.crop([x, y, w, h]).look({ zoom: 8 })"},
	"ruler":   {Hint: "Rulers are a look option.", Fix: "node.look({ ruler: true })"},
	"compare": {Hint: "compare is a cv function that takes a list of images.", Fix: `cv.compare([a, b], { mode: "side" })`},
	"export":  saveRedirect, "write": saveRedirect, "saveAs": saveRedirect, "saveTo": saveRedirect, "writeFile": saveRedirect, "store": saveRedirect,
	"toCSV": csvRedirect, "toCsv": csvRedirect, "csv": csvRedirect, "table": csvRedirect, "rows": csvRedirect,
	"json": jsonRedirect, "dump": jsonRedirect, "serialize": jsonRedirect,
}

var (
	saveRedirect = Redirect{Hint: "save writes a file under the out-root (--out-root): node.save(\"x.tif\") for an image or mask, cv.csv(x).save(\"x.csv\") for data.",
		Fix: `node.save("x.tif")`}
	csvRedirect  = Redirect{Hint: "cv.csv(x) is data for programs: every row, delivered as a file or text; cv.docs.read(\"data-out\").", Fix: `cv.csv(node, { name: "rows" })`}
	jsonRedirect = Redirect{Hint: "cv.json(x) is data for programs: every item, delivered as a file or text; cv.docs.read(\"data-out\").", Fix: `cv.json(node, { name: "data" })`}
)

// levenshtein is the case-insensitive edit distance with adjacent
// transpositions counted as one edit.
func levenshtein(a, b string) int {
	ra, rb := []rune(strings.ToLower(a)), []rune(strings.ToLower(b))
	n, m := len(ra), len(rb)
	d := make([][]int, n+1)
	for i := range d {
		d[i] = make([]int, m+1)
		d[i][0] = i
	}
	for j := 0; j <= m; j++ {
		d[0][j] = j
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}
	return d[n][m]
}

// closest returns the candidate nearest to name within a length-dependent
// edit budget, or "".
func closest(name string, candidates []string) string {
	best, bestD := "", 1<<30
	budget := max(1, min(3, len(name)/3))
	for _, c := range candidates {
		d := levenshtein(name, c)
		if strings.EqualFold(name, c) {
			d = 0
		}
		if d < bestD || (d == bestD && c < best) {
			best, bestD = c, d
		}
	}
	if bestD <= budget {
		return best
	}
	return ""
}

// Closest returns the candidate nearest to name within a small edit
// distance, or "" (for did-you-mean hints in ops).
func Closest(name string, candidates []string) string { return closest(name, candidates) }

// UnknownMethod builds the E_UNKNOWN_METHOD (or E_NAMESPACE) error for
// receiverExpr.name on a node of kind k. Suggestions come from, in order: a
// namespaced op with that member name, the synonym table, edit distance over
// the kind's methods, and the cross-kind index.
func (r *Registry) UnknownMethod(k Kind, name string, path []string) *Error {
	methods := r.Methods(k)
	topLevel := map[string]bool{}
	for _, m := range methods {
		topLevel[strings.SplitN(m, ".", 2)[0]] = true
	}
	path = append(append([]string{}, path...), pathFailureMarker+name)
	// img.luminance → img.mask.luminance
	if !strings.Contains(name, ".") {
		for _, m := range methods {
			if i := strings.Index(m, "."); i >= 0 && m[i+1:] == name {
				e := Errorf(ENamespace, "%s has no method %q; it lives in the %s namespace.", k, name, m[:i])
				e.Path, e.Fix = path, fmt.Sprintf("%s.%s(…)", receiverVar(k), m)
				return e
			}
		}
	}
	e := Errorf(EUnknownMethod, "%s has no method %q.", k, name)
	e.Path = path
	if r, ok := MethodRedirects[name]; ok {
		e.Hint, e.Fix = r.Hint, r.Fix
		return e
	}
	if target, ok := Synonyms[name]; ok {
		if contains(methods, target) {
			e.Hint = fmt.Sprintf("Did you mean %q?", target)
			e.Fix = fmt.Sprintf("%s.%s(…)", receiverVar(k), target)
			return e
		}
		if op, ok := r.Lookup(KindNone, target); ok && !op.Bespoke {
			e.Hint = fmt.Sprintf("Did you mean %q? It is a cv function.", target)
			e.Fix = fmt.Sprintf("cv.%s(…)", target)
			return e
		}
	}
	cands := make([]string, 0, len(topLevel))
	for m := range topLevel {
		cands = append(cands, m)
	}
	if sug := closest(name, cands); sug != "" {
		e.Hint = fmt.Sprintf("Did you mean %q?", sug)
		e.Fix = fmt.Sprintf("%s.%s(…)", receiverVar(k), sug)
		return e
	}
	// cross-kind: an op of that name on another kind, or a documented field.
	for _, op := range r.Find(name) {
		if op.On != k {
			e.Hint, e.Fix = crossKind(k, name, op)
			return e
		}
	}
	for _, op := range r.Ops() {
		if op.On == k && op.Returns == KindValue && !needsArgs(op) && contains(FieldNames(op.DocFields()), name) {
			e.Hint = fmt.Sprintf("%q is a field of the Value returned by %s().", name, op.Name)
			e.Fix = fmt.Sprintf("%s.%s().%s   // or (await %s.%s()).%s for the number", receiverVar(k), op.Name, name, receiverVar(k), op.Name, name)
			return e
		}
	}
	e.Valid = cands
	sortStrings(e.Valid)
	e.Hint = fmt.Sprintf("%s methods: %s.", k, strings.Join(e.Valid, ", "))
	return e
}

// crossKind is the hint and literal fix for calling op (defined on another
// kind) on a node of kind k: pass the node to the op on the right receiver
// when the op takes k as an input (img.heal(regions, …)), convert when a
// conversion exists, or call it on the right receiver.
func crossKind(k Kind, name string, op *OpDef) (string, string) {
	if op.On == KindNone {
		return fmt.Sprintf("%q is a cv function, not a method.", name), fmt.Sprintf("cv.%s(…)", name)
	}
	on, self := receiverVar(op.On), receiverVar(k)
	for _, s := range op.Args {
		if s.Positional && isKindType(s.Type) && accepts(Kind(s.Type), k) {
			return fmt.Sprintf("%q is %s method that takes %s: call it on the %s and pass this node.", name, A(string(op.On)), string(k), kindNoun(op.On)),
				fmt.Sprintf("%s.%s(%s, …)", on, name, self)
		}
	}
	hint := fmt.Sprintf("%q is %s method%s.", name, A(string(op.On)), conversionHint(k, op.On))
	var fix string
	switch {
	case k == KindRegions && op.On == KindMask:
		fix = fmt.Sprintf("%s.toMask().%s(…)", self, name)
	case k == KindMask && op.On == KindRegions:
		fix = fmt.Sprintf("%s.components().%s(…)", self, name)
	case k == KindImage && op.On == KindValue:
		fix = fmt.Sprintf("%s.stats().%s(…)", self, name)
	case k == KindMask && op.On == KindImage:
		hint = fmt.Sprintf("%q is an Image method; to apply it inside this mask use within.", name)
		fix = fmt.Sprintf("img.within(%s, i => i.%s(…))", self, name)
	case k == KindRegions && op.On == KindImage:
		hint = fmt.Sprintf("%q is an Image method; to apply it inside these regions use within with their mask.", name)
		fix = fmt.Sprintf("img.within(%s.toMask(), i => i.%s(…))", self, name)
	default:
		fix = fmt.Sprintf("%s.%s(…)", on, name)
	}
	return hint, fix
}

func kindNoun(k Kind) string {
	if k == KindImage {
		return "image"
	}
	return strings.ToLower(string(k))
}

func conversionHint(from, to Kind) string {
	switch {
	case from == KindImage && to == KindValue:
		return "; measure the image first, e.g. img.stats()"
	case from == KindRegions && to == KindMask:
		return "; convert with .toMask()"
	case from == KindMask && to == KindRegions:
		return "; convert with .components()"
	case from == KindValue && to == KindImage:
		return "; call it on the image, not on a measurement"
	}
	return ""
}

func receiverVar(k Kind) string {
	switch k {
	case KindImage:
		return "img"
	case KindMask:
		return "mask"
	case KindRegions:
		return "regions"
	case KindNone:
		return "cv"
	case KindValue:
		return "value"
	}
	return "node"
}

func contains(xs []string, x string) bool {
	for _, e := range xs {
		if e == x {
			return true
		}
	}
	return false
}

func sortStrings(xs []string) {
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && xs[j] < xs[j-1]; j-- {
			xs[j], xs[j-1] = xs[j-1], xs[j]
		}
	}
}

// needsArgs reports whether op has a required argument (so op() alone is
// not a call the agent can copy).
func needsArgs(op *OpDef) bool {
	for _, a := range op.Args {
		if a.Default == nil && !a.Optional && !a.Variadic {
			return true
		}
	}
	return false
}
