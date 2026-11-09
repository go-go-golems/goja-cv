package cvsession

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// Instructions are the MCP server instructions: the eval contract, the most
// important calls, and where to read more (cv.docs from inside eval,
// cv://docs/<id> and cv://reference as resources). Keep it under 4000
// characters; the guides carry the detail.
const Instructions = `cv is a lazy image-analysis API. You drive it with the eval tool.

Eval contract
- The code is the body of an async function: use await and end with return; the last expression is not returned. The value comes back as YAML (max 9000 chars), then warnings (RANGE, UNUSED_NODE with your line) and a context block (images, params, mem keys).
- const/let do not survive between evals: keep values on mem (a plain object that persists).
- Calls build lazy, immutable nodes; nothing runs until you await a node, return a look, or show() it. A node built and discarded gets UNUSED_NODE. Comparing a node with < or > is E_LAZY_COMPARE: await it first.
- Lengths, boxes and points are in doc px (original-image pixels) at every scale; areas in doc px².
- Errors are YAML with code, message, where (your line), path, hint and a literal fix: apply the fix.
- The document layer (doc.apply, propose, undo, export) is not available yet: describe edits and show them as looks.

Globals: img (the open image; img = cv.open(path) rebinds it; await img gives its file, format, bitDepth, encoding, frame), cv, P, mem, show. cv.open(path, { encoding, box, origin, as }): encoding "linear" for linear data in integer files; box reads a window and keeps file coordinates; as: "ref" binds another global name.

Most used calls
- Numbers are in the op's space: linear (exposure, blur, blend, stats, levels) or encoded (what looks show: thresholds, contrast, histogram, specks); pass { space } to change it; cv.docs.read("encodings-and-spaces").
- await img.stats() → { meanL, p01, p99, clipLow, clipHigh, … }; img.stats({ box: [x, y, w, h] }) or img.stats({ mask }) measures only there (+ area, coverage); img.histogram().look()
- img.exposure(ev), img.levels({ black, white, gamma }) (black/white in stats' space), img.contrast(a), img.gaussian(sigma), img.toGray(), img.chroma(), node.colormap("viridis")
- node.look("label") or node.look({ label, maxSide }) inside the returned value attaches an image; show(node, opts) queues one that survives a later throw. Captions give the op chain and the scale: 1 image px = S doc px.
- Magnify with coordinates: img.crop([x, y, w, h]).look({ zoom: 8, ruler: 8 }) (crop last; rulers read absolute doc px).
- Views that make small things stand out: img.view("speck" | "speckDark" | "signed" | "detail" | "neutral" | "scene"), img.highpass(sigma), img.dog(1, 4); cv.docs.read("views").
- Many candidates in one image: cv.sheet(regions.sortBy("area").top(36), { views: [img, img.view("speck")] }).look(); mark points with img.annotate([{ circle: [x, y, r] }]).
- cv.compare([a, b], { mode: "side" | "split" }) to see before/after; a.sub(b) is their signed difference (.mul(4).add(0.5) shows zero as grey).
- Masks: img.mask.luminance({ above, below }), mask.box([x, y, w, h]), mask.gradient({ from: "top", to: 0.5 }), mask.color, mask.flood; a.and(b), a.or(b), a.minus(b), a.not(), m.feather(radius), m.grow(radius), m.coverage().
- img.within(mask, i => i.exposure(-0.4)) edits inside a mask (the function runs once, at build time).
- Dust: img.detect.specks() → Regions; .where({ score: { gte: 0.5 } }), .sortBy("area").top(n), .without([id]), .count(); img.overlay(regions, { labels: "id" }).look(); img.heal(regions, { radius }); await img.check.dust().
- Params: P.slider(name, { min, max, value }) can be used anywhere a number goes.

More: cv.docs.list() (topics), cv.docs.search("heal dust") (sections with snippets), cv.docs.read(id, { section }) (Markdown); start with cv.docs.read("getting-started") and cv.docs.read("recipes"). cv.help(op) describes one op (arguments, units, defaults). Resources: cv://docs/<id>, cv://reference (every op) and cv://guide/<slug> (the guidebook).`

// EvalToolDescription is the description of the cv eval tool (Part 3 §17.4).
const EvalToolDescription = "Run JavaScript against the cv image API in a persistent session. The code is the body of an async function: use await, and return what you want to see (rendered as YAML, max 9000 chars). node.look(\"label\") in the returned value, or show(node), attaches an image. Top-level const/let do not survive between calls; store values on mem. Read the guides from inside eval: cv.docs.search(text), cv.docs.read(id); cv.help(op) describes one op."

// Reference renders the op registry as Markdown: every op by receiver kind
// with its usage, arguments and one line of documentation.
func Reference(reg *graph.Registry) string {
	var sb strings.Builder
	sb.WriteString("# cv reference\n\n")
	sb.WriteString("Generated from the op registry. Calls build lazy nodes; `await` a node (or return `node.look()`) to compute it.\n\n")
	byKind := map[graph.Kind][]*graph.OpDef{}
	for _, op := range reg.Ops() {
		byKind[op.On] = append(byKind[op.On], op)
	}
	kinds := append([]graph.Kind{graph.KindNone}, graph.Kinds...)
	for _, k := range kinds {
		ops := byKind[k]
		if len(ops) == 0 {
			continue
		}
		sort.Slice(ops, func(i, j int) bool { return ops[i].Name < ops[j].Name })
		if k == graph.KindNone {
			sb.WriteString("## Module ops and sources (cv.open, cv.compare, P)\n\n")
		} else {
			fmt.Fprintf(&sb, "## %s methods\n\n", k)
		}
		for _, op := range ops {
			sb.WriteString(opLine(op))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("## Functions\n\n")
	for _, f := range reg.Funcs() {
		sb.WriteString(funcLine(f))
	}
	sb.WriteString("\n")
	sb.WriteString("## Globals\n\n")
	sb.WriteString("- `cv.open(path, { encoding, box, origin, as })` → Image source; `cv.help(name?)`; `cv.evaluate(node, { scale })` (number, \"preview\", \"look\", \"full\")\n")
	sb.WriteString("- `P.slider(name, { min, max, value, step, label, unit })`, `P.toggle(name, { value })`, `P.choice(name, { options, value })`\n")
	sb.WriteString("- `show(node, opts)` / `show(look)`; `mem` (persists); `img` (the open image)\n")
	return sb.String()
}

func funcLine(f *graph.FuncDef) string {
	line := "- `" + f.UsageLine() + "`"
	if len(f.On) > 0 {
		line += " (on " + strings.Join(f.On, ", ") + ")"
	}
	if doc := f.Doc; doc != "" {
		if i := strings.Index(doc, ". "); i > 0 {
			doc = doc[:i+1]
		}
		line += " — " + doc
	}
	line += "\n"
	for i := range f.Args {
		if s := &f.Args[i]; !s.Hidden {
			line += "  - " + s.Name + ": " + graph.DescribeArgSpec(s) + "\n"
		}
	}
	return line
}

func opLine(op *graph.OpDef) string {
	var args []string
	var details []string
	for i := range op.Args {
		s := &op.Args[i]
		if s.Hidden {
			continue
		}
		if s.Name == graph.SpaceArg && op.Space.Default != buffer.SpaceNone {
			details = append(details, fmt.Sprintf("space: \"linear\" | \"encoded\", default %q. working space (the option is not part of the usage line)", op.Space.Default.String()))
			continue
		}
		args = append(args, s.Name)
		details = append(details, s.Name+": "+graph.DescribeArgSpec(s))
	}
	recv := ""
	if op.On == graph.KindNone {
		recv = "cv."
	}
	line := fmt.Sprintf("- `%s%s(%s)` → %s", recv, op.Name, strings.Join(args, ", "), op.Returns)
	if op.Doc != "" {
		doc := op.Doc
		if i := strings.Index(doc, ". "); i > 0 {
			doc = doc[:i+1]
		}
		line += " — " + doc
	}
	line += "\n"
	for _, d := range details {
		line += "  - " + d + "\n"
	}
	return line
}
