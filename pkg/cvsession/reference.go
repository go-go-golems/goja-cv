package cvsession

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// Instructions are the MCP server instructions: the eval contract, the most
// important calls, and where to read more (cv.docs from inside eval,
// cv://docs/<id> and cv://reference as resources). Keep it under 4000
// characters; the guides carry the detail.
const Instructions = `cv is a lazy image-analysis API. You drive it with the eval tool.

Eval contract
- The code is the body of an async function: use await and end with return. The value comes back as YAML (max 9000 chars; a list of flat rows comes back as a table, paged), then warnings (with your line) and a context block (images, mem, and the docs of new mem functions).
- const/let do not survive between evals: keep values on mem.
- Calls build lazy, immutable nodes; a node you don't await, look at or save does nothing. await gives data: a Value or Regions node is computed at full resolution (regions become an array of flat items { id, x, y, w, h, cx, cy, … }); await img gives the image's file, encoding and frame. Options are literal values: levels({ white: (await d.stats()).p99 }).
- node.look(label | { zoom, ruler }) shows an image; every look is delivered, at most 4 per eval. Never read a coordinate off a picture: take it from data (peaks, items, measure). cv.explain(node) describes a node.
- Coordinates, lengths and boxes are doc px (original-image pixels).
- Errors are YAML with code, message, where (your line, or file:line in a loaded file), hint and a literal fix: apply the fix.

Task libraries
- await cv.load("dir/file.js", args) runs a script file in the session (a task library that defines mem functions, or a saved pipeline); cv.scripts() lists the .js files you can load. Use a library's functions when a playbook names one; never paste a file into an eval.

Images, spaces, files
- img is the open image; cv.open(path, { name, encoding }) opens another (workspace first, then --read directories). Images open in file units; img.as("linear") converts; every op works in its receiver's space.
- await cv.save("out/x.tif" | ".json" | ".csv", x) writes into the workspace.

Most used calls
- await img.stats() → { meanL, p01, p99, … }; img.stats({ box: [x, y, w, h] }) or { mask } measures there; img.histogram().look()
- img.levels({ black, white, auto }), img.exposure(ev), img.gaussian(sigma), img.toGray(), x.colormap("viridis")
- Magnify: img.crop([x, y, w, h]).look({ zoom: 8, ruler: true }); ops after a crop see only the crop.
- Views: img.view("speck" | "speckDark" | "signed" | "detail" | "scene"); img.dog(1, 4); the speck view's scale is set over the image it is computed on. img.detect.specks() → regions.
- Find spots: await img.as("linear").peaks({ radius: [2, 4, 8] }) → disks { id, cx, cy, r, step, snr, … } sorted by snr (snr against each spot's own ring; default snr ≥ 1 keeps faint ones: slice or filter); on img.crop(b) only the crop. cv.docs.read("spots").
- Many candidates in one picture: cv.sheet(items.slice(0, 15), { views: [img, img.view("speck")] }).look(); its caption lists the cells' ids.
- Measure: await img.as("linear").measure(items) (step, spread, snr around each item, sized from r); img.profile(a, b, { width }) across a line; img.sample(points).
- Filter, sort and page items with JavaScript: items.filter(r => r.snr >= 1.4 && r.aspect < 3).
- Masks are one-channel images: img.mask.luminance({ above }), mask.box, img.mask.regions(items); m.and/or/minus/not, m.feather(r), m.components(), m.coverage().
- Morphology on one-channel images: x.erode(r), x.dilate(r) (a disk; { line: angle } for a line). Opening: x.erode(r).dilate(r). Top-hat: x.sub(x.erode(r).dilate(r)) keeps bright things smaller than r, filled.
- img.overlay(items, { labels: "id" }).look() draws items; img.annotate([{ circle: [x, y, r] }]) marks points; img.within(mask, i => i.exposure(-0.4)) edits inside a mask; img.heal(items) removes them; cv.compare([a, b]) before/after.

More: cv.help(op) (one op or function), cv.docs.search("width of a line"), cv.docs.read("recipes"); resources cv://docs/<id>, cv://reference, cv://guide/<slug>.`

// EvalToolDescription is the description of the cv eval tool (Part 3 §17.4).
const EvalToolDescription = "Run JavaScript against the cv image API in a persistent session. The code is the body of an async function: use await, and return what you want to see (rendered as YAML, max 9000 chars). node.look(\"label\") attaches an image (every look of an eval is delivered). Top-level const/let do not survive between calls; store values on mem. Task libraries: await cv.load(path, args); cv.scripts() lists them. Read the guides from inside eval: cv.docs.search(text), cv.docs.read(id); cv.help(op) describes one op."

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
			sb.WriteString("## Module ops and sources (cv.open, cv.compare, cv.sheet)\n\n")
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
	sb.WriteString("- `cv.open(path, { encoding, box, origin, name })` → Image source (the workspace, then the read directories); `await cv.save(path, x, { depth })` → the saved file; `cv.help(name?)`; `cv.explain(node)` (synchronous, evaluates nothing)\n")
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
