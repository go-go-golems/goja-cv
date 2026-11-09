package cvsession

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// Instructions are the eval rules an agent would otherwise learn by failing.
// They are the MCP server instructions and the head of the cv-agent prompt.
const Instructions = `cv is a lazy image-analysis API driven through the eval tool. Rules:
- The code is the body of an async function: use await, and return what you want to see. The last expression is NOT returned. Results come back as YAML (max 9000 characters); long arrays are cut.
- Calls on nodes build lazy, immutable, hash-consed nodes: img.exposure(0.3) computes nothing. Nothing runs until you await a node, return or show a look, or use it in another node. A node built and discarded gets an UNUSED_NODE warning.
- await a Value node to get data ((await img.stats()).meanL); awaiting an Image gives a summary { kind, w, h, scale }, never pixels.
- Looks are images you return: node.look("label") or node.look({ label, maxSide }) inside the returned value (alone, in an object or array) attaches the image; show(node, opts) also queues it so it arrives even if the eval throws later. Each look is numbered (the number is burned into its top-left corner) and its caption gives the scale: "1 image px = S doc px"; multiply look coordinates by S for document coordinates.
- Globals: cv (cv.help(), cv.help("name"), cv.open(path)), P (params: P.slider(name, { min, max, value })), img (the open image; img = cv.open(path) rebinds it), mem (a plain object that persists between evals), show. Top-level const/let do not survive between evals: keep values on mem.
- Lengths are in document pixels (doc px) at every scale; levels black/white are linear-light levels like stats.
- Errors come back as YAML with code, message, where (your line), hint and a literal fix: apply the fix.
- The document layer (doc.apply, doc.propose, export) is not available yet; describe edits as looks.
- Read cv://reference (or call cv.help()) for every op.`

// EvalToolDescription is the description of the cv eval tool (Part 3 §17.4).
const EvalToolDescription = "Run JavaScript against the cv image API in a persistent session. The code is the body of an async function: use await, and return what you want to see (rendered as YAML, max 9000 chars). node.look(\"label\") in the returned value, or show(node), attaches an image. Top-level const/let do not survive between calls; store values on mem. Call cv.help() or cv.help('name') for the reference."

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
			sb.WriteString("## Sources (cv.open, P)\n\n")
		} else {
			fmt.Fprintf(&sb, "## %s methods\n\n", k)
		}
		for _, op := range ops {
			sb.WriteString(opLine(op))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("## Every node\n\n")
	sb.WriteString("- `look(label | { label, maxSide })` → look (an image you return; Image, Mask, histogram)\n")
	sb.WriteString("- `explain()` → Value: the graph under the node, without computing pixels\n\n")
	sb.WriteString("## Globals\n\n")
	sb.WriteString("- `cv.open(path)` → Image source; `cv.help(name?)`; `cv.evaluate(node, { scale })` (number, \"preview\", \"look\", \"full\")\n")
	sb.WriteString("- `P.slider(name, { min, max, value, step, label, unit })`, `P.toggle(name, { value })`, `P.choice(name, { options, value })`\n")
	sb.WriteString("- `show(node, opts)` / `show(look)`; `mem` (persists); `img` (the open image)\n")
	return sb.String()
}

func opLine(op *graph.OpDef) string {
	var args []string
	var details []string
	for i := range op.Args {
		s := &op.Args[i]
		if s.Hidden {
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
