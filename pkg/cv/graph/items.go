package graph

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Items (GCV-004 §4.5, §7.6). An argument of type Items takes a Regions
// node or an array of items: awaited region items, still linked to their
// set by [node, index] (the prelude sends the link as __cvref), and literal
// boxes ({ x, y, w, h } or [x, y, w, h]) and circles ({ cx, cy, r }). An
// argument "Items|point[]" (measure, sample, cv.sheet) also takes centres
// without a radius ([x, y] or { cx, cy }). The binder interns an array as an
// internal items node over at most one region set; its Run takes the linked
// regions by position, so their exact pixels are used, and builds the
// literal ones from their geometry.

// ItemsType is the type word of an argument that takes items.
const ItemsType = "Items"

// ItemsOp is the internal op an item array becomes.
const ItemsOp = "items"

// WItems: unlinked objects that look like region items (they lost their link).
const WItems = "ITEMS"

// cvRefKey is the key the prelude sends an awaited item's link under.
const cvRefKey = "__cvref"

// takesItems reports whether a declared type takes items.
func takesItems(typ string) bool { return hasPart(typ, ItemsType) }

// takesPoints reports whether a declared Items type also takes centres
// without a radius.
func takesPoints(typ string) bool { return hasPart(typ, "point[]") }

func hasPart(typ, part string) bool {
	for _, d := range strings.Split(typ, "|") {
		if d == part {
			return true
		}
	}
	return false
}

// stripRefs drops the item links from a value bound to an argument that
// does not take items (a box, a point, an options object, annotate's
// marks): there an item is just its fields.
func stripRefs(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			if k != cvRefKey {
				out[k] = stripRefs(e)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = stripRefs(e)
		}
		return out
	}
	return v
}

// twoSetsFix is the op-specific fix for an array that mixes the items of
// two region sets (D19).
func twoSetsFix(call string) string {
	switch call {
	case "overlay":
		return "img.overlay(a).overlay(b)"
	case "cv.sheet":
		return `img.detect.specks({ polarity: "both" })   // one set; or one sheet per set`
	case "heal":
		return "img.heal(a).heal(b)"
	case "measure":
		return "await img.measure(a); await img.measure(b)"
	case "mask.regions":
		return "img.mask.regions(a).or(img.mask.regions(b))"
	case "stats":
		return "img.stats({ mask: img.mask.regions(a).or(img.mask.regions(b)) })"
	}
	return call + "(a), then " + call + "(b)"
}

// checkItems binds an Items argument: a Regions node (an input), a mask for
// "Items|Mask", or an array, interned as an items node.
func (b *binder) checkItems(s *ArgSpec, typ string, v any, where string) (any, error) {
	nodeTyp := strings.ReplaceAll(typ, "|point[]", "")
	if na, ok := v.(NodeArg); ok {
		return b.lift(s, na.ID, where, nodeTyp)
	}
	list, ok := v.([]any)
	if !ok {
		e := Errorf(EArgType, "%s takes regions: a Regions node or an array of items, got %s.", where, describeRaw(v))
		if m, isObj := v.(map[string]any); isObj && (hasNumericKeys(m, boxKeys) || hasNumericKeys(m, pointKeys)) {
			e.Fix = fmt.Sprintf("%s([item], …)   // an array, even for one item", b.callName())
		}
		return nil, e
	}
	if b.items == nil {
		return nil, Errorf(EArgType, "%s: item arrays are not accepted here.", where)
	}
	entries, src, err := b.parseItems(list, takesPoints(typ), where)
	if err != nil {
		return nil, err
	}
	id, err := b.items(entries, src)
	if err != nil {
		return nil, err
	}
	b.inputs = append(b.inputs, id)
	return InputRef{Index: b.base + len(b.inputs) - 1}, nil
}

// parseItems turns an item array into the canonical entries of an items
// node: { index, x, y, w, h } for a linked item, { shape: box|disk|point,
// geometry } for a literal one, each with id and label when given.
func (b *binder) parseItems(list []any, points bool, where string) ([]any, NodeID, error) {
	var src NodeID
	var lost []int
	entries := make([]any, len(list))
	shapes := "a box { x, y, w, h } or [x, y, w, h], or a circle { cx, cy, r }"
	if points {
		shapes += ", or a point [x, y] or { cx, cy }"
	}
	for i, e := range list {
		w := fmt.Sprintf("%s[%d]", where, i)
		ent := map[string]any{}
		switch x := e.(type) {
		case map[string]any:
			num := func(k string) (float64, bool) {
				f, ok := x[k].(float64)
				return f, ok && !math.IsNaN(f) && !math.IsInf(f, 0)
			}
			has := func(keys ...string) bool {
				for _, k := range keys {
					if _, ok := num(k); !ok {
						return false
					}
				}
				return true
			}
			if ref, ok := x[cvRefKey]; ok {
				node, idx, ok := parseRef(ref)
				if !ok {
					return nil, 0, Errorf(EArgType, "%s carries a broken item link %v.", w, ref)
				}
				n, ok := b.nodeOf(node)
				if !ok || n.Kind != KindRegions {
					return nil, 0, Errorf(EArgType, "%s is linked to node #%d, which is not a region set of this session.", w, node).
						WithHint("Await the regions in this session and pass the items, or pass plain boxes.")
				}
				if src != 0 && src != node {
					return nil, 0, Errorf(EArgType, "%s mixes the items of two region sets (#%d and #%d): one array holds the items of one set, plus any literal boxes and circles.", where, src, node).
						WithFix(twoSetsFix(b.callName()))
				}
				src = node
				if !has("x", "y", "w", "h") {
					return nil, 0, Errorf(EArgType, "%s is a linked region item without its box x, y, w, h.", w).
						WithFix("pass the item unchanged, or { ...r } (spread keeps the link)")
				}
				ent["index"] = float64(idx)
				for _, k := range boxKeys {
					ent[k], _ = num(k)
				}
			} else {
				switch {
				case has("cx", "cy", "r"):
					r, _ := num("r")
					if r <= 0 {
						return nil, 0, Errorf(EArgRange, "%s.r = %s: a circle's radius must be positive (doc px).", w, fmtNum(r))
					}
					ent["shape"] = "disk"
					ent["cx"], _ = num("cx")
					ent["cy"], _ = num("cy")
					ent["r"] = r
				case has("x", "y", "w", "h"):
					ent["shape"] = "box"
					for _, k := range boxKeys {
						ent[k], _ = num(k)
					}
					if ent["w"].(float64) < 0 || ent["h"].(float64) < 0 {
						return nil, 0, Errorf(EArgRange, "%s has a negative size; a box is { x, y, w, h } with w, h ≥ 0 (doc px).", w)
					}
					if _, ok := x["id"]; ok {
						if _, ok := x["area"]; ok {
							lost = append(lost, i)
						}
					}
				case points && has("cx", "cy"):
					ent["shape"] = "point"
					ent["cx"], _ = num("cx")
					ent["cy"], _ = num("cy")
				default:
					return nil, 0, Errorf(EArgType, "%s is neither a region item nor %s (doc px).", w, shapes).
						WithHint("Region items keep a link to their set; objects built field by field lose it and need a box or a circle.")
				}
			}
			if err := itemIDLabel(x, ent, w); err != nil {
				return nil, 0, err
			}
		case []any:
			c, err := b.coords(&ArgSpec{}, x, w)
			if err != nil {
				return nil, 0, err
			}
			f := c.([]any)
			switch {
			case len(f) == 4:
				ent["shape"] = "box"
				for k, name := range boxKeys {
					ent[name] = f[k]
				}
				if f[2].(float64) < 0 || f[3].(float64) < 0 {
					return nil, 0, Errorf(EArgRange, "%s has a negative size; a box is [x, y, w, h] with w, h ≥ 0 (doc px).", w)
				}
			case len(f) == 2 && points:
				ent["shape"], ent["cx"], ent["cy"] = "point", f[0], f[1]
			default:
				return nil, 0, Errorf(EArgType, "%s is a list of %d numbers; an item is %s.", w, len(f), shapes)
			}
		default:
			return nil, 0, Errorf(EArgType, "%s is %s; an item is a region item or %s.", w, describeRaw(e), shapes)
		}
		entries[i] = ent
	}
	if len(lost) > 0 {
		b.warnings = append(b.warnings, Warning{Code: WItems, Message: fmt.Sprintf(
			"%s: items %s look like region items but lost their link (a map that built new objects?); they are used as plain boxes. Keep the objects (filter, sort, slice) or spread them ({ ...r, note }) to keep their pixels.",
			where, IndexRanges(lost))})
	}
	return entries, src, nil
}

// itemIDLabel copies an item's id (a number; ids may repeat) and label (a
// string printed instead of the id) into its entry.
func itemIDLabel(x, ent map[string]any, where string) error {
	if v, ok := x["id"]; ok && v != nil {
		f, isNum := v.(float64)
		if !isNum || f != math.Trunc(f) || math.IsInf(f, 0) {
			text := fmt.Sprint(v)
			if s, ok := v.(string); ok {
				text = s
			}
			return Errorf(EArgType, "%s.id is %s; an id is a whole number (ids may repeat). Text belongs in label, which overlay and sheet labels print instead of the id.", where, describeRaw(v)).
				WithFix(fmt.Sprintf("{ ...item, id: 1, label: %q }", text))
		}
		ent["id"] = f
	}
	if v, ok := x["label"]; ok && v != nil {
		switch l := v.(type) {
		case string:
			if l != "" {
				ent["label"] = l
			}
		case float64:
			ent["label"] = fmtNum(l)
		default:
			return Errorf(EArgType, "%s.label is %s; a label is a string.", where, describeRaw(v)).WithFix(`{ ...item, label: "dust 1" }`)
		}
	}
	return nil
}

func parseRef(v any) (NodeID, int, bool) {
	l, ok := v.([]any)
	if !ok || len(l) != 2 {
		return 0, 0, false
	}
	n, ok1 := l[0].(float64)
	i, ok2 := l[1].(float64)
	if !ok1 || !ok2 || n < 1 || i < 0 || n != math.Trunc(n) || i != math.Trunc(i) {
		return 0, 0, false
	}
	return NodeID(n), int(i), true
}

// IndexRanges renders sorted indices as "4–9, 12".
func IndexRanges(idx []int) string {
	var parts []string
	for i := 0; i < len(idx); {
		j := i
		for j+1 < len(idx) && idx[j+1] == idx[j]+1 {
			j++
		}
		if j > i {
			parts = append(parts, fmt.Sprintf("%d–%d", idx[i], idx[j]))
		} else {
			parts = append(parts, strconv.Itoa(idx[i]))
		}
		i = j + 1
	}
	return strings.Join(parts, ", ")
}

// itemsNode interns the items node of entries over region set src (0: none).
func (b *Builder) itemsNode(entries []any, src NodeID) (NodeID, error) {
	op, ok := b.reg.Lookup(KindNone, ItemsOp)
	if !ok {
		return 0, Errorf(EInternal, "no items op registered")
	}
	vals := map[string]any{"entries": entries}
	var ins []*Node
	if src != 0 {
		n, ok := b.Node(src)
		if !ok {
			return 0, Errorf(EInternal, "unknown region set #%d", src)
		}
		vals["source"] = InputRef{Index: 0}
		ins = []*Node{n}
	}
	return b.internNode(op, Args{op: op, vals: vals, ins: ins}, ins).ID, nil
}

// itemEntries are the entries of an items node (nil for another node).
func itemEntries(n *Node) []any {
	if n.Op.Name != ItemsOp || n.Op.On != KindNone {
		return nil
	}
	l, _ := n.Args.vals["entries"].([]any)
	return l
}

// ItemCount is the number of items of an items node, known when it is
// built (ok false for a detector's Regions node).
func ItemCount(n *Node) (int, bool) {
	if n.Op.Name != ItemsOp || n.Op.On != KindNone {
		return 0, false
	}
	return len(itemEntries(n)), true
}

// ItemTexts are the texts overlay and sheet print for the items of an items
// node, in order: each item's label, else its id, else its position from 1
// (literal items without an id). ok is false for another node (a detector's
// Regions node, whose ids are known only when it is evaluated).
func ItemTexts(n *Node) ([]string, bool) {
	if n.Op.Name != ItemsOp || n.Op.On != KindNone {
		return nil, false
	}
	entries := itemEntries(n)
	out := make([]string, len(entries))
	for i, e := range entries {
		m, _ := e.(map[string]any)
		switch l, id := m["label"], m["id"]; {
		case l != nil && l != "":
			out[i] = fmt.Sprint(l)
		case id != nil:
			out[i] = fmtNum(id.(float64))
		default:
			out[i] = strconv.Itoa(i + 1)
		}
	}
	return out, true
}

// HasPointItems reports whether an items node holds a centre without a
// radius ([x, y] or { cx, cy }).
func HasPointItems(n *Node) bool {
	for _, e := range itemEntries(n) {
		if m, ok := e.(map[string]any); ok && m["shape"] == "point" {
			return true
		}
	}
	return false
}

var (
	rField     = FieldSpec{Name: "r", Type: "number", Unit: DocPx, Optional: true, Doc: "radius of a literal circle"}
	labelField = FieldSpec{Name: "label", Type: "string", Optional: true, Doc: "text printed instead of the id by overlay and sheet labels"}
)

func itemsFields(a Args) []FieldSpec {
	fs := GeometryFields
	if a.HasInputs() && a.Has("source") {
		fs = a.Input("source").Fields
	}
	var disk, label bool
	for _, e := range a.Raw("entries").([]any) {
		m := e.(map[string]any)
		disk = disk || m["shape"] == "disk"
		_, l := m["label"]
		label = label || l
	}
	if _, ok := FindField(fs, "r"); disk && !ok {
		fs = WithFields(fs, rField)
	}
	if _, ok := FindField(fs, "label"); label && !ok {
		fs = WithFields(fs, labelField)
	}
	return fs
}

func runItems(_ context.Context, _ *EvalCtx, _ []Value, a Args) (Value, error) {
	entries := a.Raw("entries").([]any)
	out := &RegionsValue{Scale: 1}
	var src *RegionsValue
	if a.Has("source") {
		src = a.Value("source").(*RegionsValue)
		out.W, out.H, out.Scale, out.Frame = src.W, src.H, src.Scale, src.Frame
	}
	out.Items = make([]*Region, len(entries))
	for k, e := range entries {
		m := e.(map[string]any)
		f := func(key string) float64 { v, _ := m[key].(float64); return v }
		var r *Region
		if idx, linked := m["index"].(float64); linked {
			i := int(idx)
			if src == nil || i >= len(src.Items) {
				return nil, Errorf(EArgType, "item %d links to position %d of a set of %d regions.", k, i, len(src.Items))
			}
			orig := src.Items[i]
			given := [4]float64{f("x"), f("y"), f("w"), f("h")}
			for j := range given {
				if math.Abs(round(orig.Box[j], 2)-given[j]) > 1e-6 {
					return nil, Errorf(EArgType, "item %d was moved: its box [%s, %s, %s, %s] is not its region's [%s, %s, %s, %s].",
						k, fmtNum(given[0]), fmtNum(given[1]), fmtNum(given[2]), fmtNum(given[3]),
						fmtNum(round(orig.Box[0], 2)), fmtNum(round(orig.Box[1], 2)), fmtNum(round(orig.Box[2], 2)), fmtNum(round(orig.Box[3], 2))).
						WithHint("A linked item stands for its region's exact pixels, which an edited box would silently ignore.").
						WithFix(fmt.Sprintf("{ x: %s, y: %s, w: %s, h: %s }   // a plain box uses the new geometry; the item unchanged uses its pixels",
							fmtNum(given[0]), fmtNum(given[1]), fmtNum(given[2]), fmtNum(given[3])))
				}
			}
			c := *orig
			r = &c
		} else {
			r = &Region{ID: k + 1, Measures: map[string]float64{}, Tags: map[string]string{}}
			switch m["shape"] {
			case "box":
				r.Shape = Box
				r.Box = [4]float64{f("x"), f("y"), f("w"), f("h")}
				r.Area = r.Box[2] * r.Box[3]
				r.Centroid = [2]float64{r.Box[0] + r.Box[2]/2, r.Box[1] + r.Box[3]/2}
				lo, hi := math.Min(r.Box[2], r.Box[3]), math.Max(r.Box[2], r.Box[3])
				r.Measures["aspect"] = hi / math.Max(lo, 1e-9)
				if hi == 0 {
					r.Measures["aspect"] = 1
				}
			case "disk":
				r.Shape, r.R = Disk, f("r")
				r.Centroid = [2]float64{f("cx"), f("cy")}
				r.Box = [4]float64{r.Centroid[0] - r.R, r.Centroid[1] - r.R, 2 * r.R, 2 * r.R}
				r.Area = math.Pi * r.R * r.R
				r.Measures["aspect"] = 1
			default:
				r.Shape = Point
				r.Centroid = [2]float64{f("cx"), f("cy")}
				r.Box = [4]float64{r.Centroid[0], r.Centroid[1], 0, 0}
				r.Measures["aspect"] = 1
			}
		}
		if id, ok := m["id"].(float64); ok {
			r.ID = int(id)
		}
		if l, ok := m["label"].(string); ok {
			r.Label = l
		}
		out.Items[k] = r
	}
	return out, nil
}

// ItemsOpDef is the internal op an item array passed to an Items argument
// becomes (registered by the ops package).
func ItemsOpDef() OpDef {
	return OpDef{
		Name: ItemsOp, On: KindNone, Returns: KindRegions, Internal: true, Bespoke: true,
		Args: []ArgSpec{
			{Name: "source", Type: "Regions", Optional: true, Doc: "the region set the linked items come from"},
			{Name: "entries", Type: "any", Doc: "the items, in array order"},
		},
		AnalysisScale: func(Args) float64 { return 1 },
		DocFrame: func(a Args) Frame {
			if a.HasInputs() && a.Has("source") {
				return a.Input("source").Frame
			}
			return Frame{}
		},
		Fields: itemsFields,
		Run:    runItems,
		Doc:    "An array of items passed where regions are accepted: linked region items (their pixels), boxes, circles and points.",
	}
}

// ItemsFrameError checks the frame of items (a Regions value) against the
// frame of the image an op uses them on: literal items have no frame and
// fit any image; a region set must cover the same frame.
func ItemsFrameError(op string, items Frame, image Frame) error {
	if items.Empty() {
		return nil
	}
	return SameFrame(op, items, image)
}
