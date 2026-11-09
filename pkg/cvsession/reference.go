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
- The code is the body of an async function: use await and end with return; the last expression is not returned. The value comes back as YAML (max 9000 chars), then warnings (RANGE, UNUSED_NODE with your line) and a context block (images, mem keys).
- const/let do not survive between evals: keep values on mem (a plain object that persists).
- Calls build lazy, immutable nodes; await evaluates (data at full resolution; an awaited Image is its summary, nothing computed). A node built and discarded gets UNUSED_NODE. Options are literal values: await a measurement first, levels({ white: (await d.stats()).p99 }).
- Lengths, boxes and points are in doc px (original-image pixels) at every scale; areas in doc px².
- Errors are YAML with code, message, where (your line), path, hint and a literal fix: apply the fix.

Globals: img (the open image; img = cv.open(path) rebinds it; await img gives its file, format, bitDepth, encoding, frame), cv, mem. cv.open(path, { encoding, box, origin, as }): encoding "linear" for linear data in integer files; box reads a window and keeps file coordinates; as: "ref" binds another global name.

Most used calls
- Numbers are in the op's space: linear (exposure, blur, blend, stats, levels) or encoded (what looks show: thresholds, contrast, histogram, specks); pass { space } to change it; cv.docs.read("encodings-and-spaces").
- await img.stats() → { meanL, p01, p99, clipLow, clipHigh, … }; img.stats({ box: [x, y, w, h] }) or img.stats({ mask }) measures only there (+ area, coverage); img.histogram().look()
- img.exposure(ev), img.levels({ black, white, gamma }), img.contrast(a), img.gaussian(sigma), img.toGray(), node.colormap("viridis")
- node.look("label") or node.look({ label, maxSide }) queues an image: every look of an eval is delivered (returned or not, also after a throw), at most four; a LOOKS warning names the others. Captions give the op chain and the scale: 1 image px = S doc px. cv.explain(node) describes a node without evaluating it.
- Magnify with coordinates: img.crop([x, y, w, h]).look({ zoom: 8, ruler: 8 }) (crop last; rulers read absolute doc px).
- Views that make small things stand out: img.view("speck" | "speckDark" | "signed" | "detail" | "neutral" | "scene"), img.highpass(sigma), img.dog(1, 4); cv.docs.read("views").
- Many candidates in one image: cv.sheet(regions.sortBy("area").top(36), { views: [img, img.view("speck")] }).look(); mark points with img.annotate([{ circle: [x, y, r] }]).
- cv.compare([a, b], { mode: "side" | "split" }) to see before/after; a.sub(b) is their signed difference (.mul(4).add(0.5) shows zero as grey).
- Masks: img.mask.luminance({ above, below }), mask.box([x, y, w, h]), mask.gradient({ from: "top", to: 0.5 }), mask.color, mask.flood; a.and(b), a.or(b), a.minus(b), a.not(), m.feather(radius), m.grow(radius), m.coverage().
- img.within(mask, i => i.exposure(-0.4)) edits inside a mask (the function runs at build time).
- Dust: img.detect.specks() → Regions with flat items { id, x, y, w, h, cx, cy, area, … } (pass an item as box, point or id); .where({ score: { gte: 0.5 } }), .sortBy("area").top(n), .without([id]); img.overlay(regions, { labels: "id" }).look(); img.heal(regions, { radius }).
- Measure: img.measure(points) (step, spread, snr, stepChroma), img.sample, img.profile(a, b).look(), regions.measure(img, { ring: true }); cv.docs.read("measuring").
- Data: return cv.csv(x, { name }) or cv.json(x) for all rows; node.save(path) under --out-root; cv.docs.read("data-out").

More: cv.docs.list() (topics), cv.docs.search("heal dust") (sections with snippets), cv.docs.read(id, { section }) (Markdown); start with cv.docs.read("getting-started") and cv.docs.read("recipes"). cv.help(op) describes one op (arguments, units, defaults). Resources: cv://docs/<id>, cv://reference (every op) and cv://guide/<slug> (the guidebook).`

// EvalToolDescription is the description of the cv eval tool (Part 3 §17.4).
const EvalToolDescription = "Run JavaScript against the cv image API in a persistent session. The code is the body of an async function: use await, and return what you want to see (rendered as YAML, max 9000 chars). node.look(\"label\") attaches an image (every look of an eval is delivered). Top-level const/let do not survive between calls; store values on mem. Read the guides from inside eval: cv.docs.search(text), cv.docs.read(id); cv.help(op) describes one op."

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
	kinds := append([]graph.Kind{graph.KindNone}, reg.Receivers()...)
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
	sb.WriteString("- `cv.open(path, { encoding, box, origin, as })` → Image source; `cv.help(name?)`; `cv.explain(node)` (synchronous, evaluates nothing)\n")
	sb.WriteString("- `mem` (persists); `img` (the open image); `await node` evaluates (data at scale 1; an Image is its summary)\n")
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
		s := &f.Args[i]
		line += "  - " + s.Name + ": " + graph.DescribeArgSpec(s) + "\n"
	}
	return line
}

func opLine(op *graph.OpDef) string {
	var args []string
	var details []string
	for i := range op.Args {
		s := &op.Args[i]
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
