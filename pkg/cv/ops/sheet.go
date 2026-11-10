package ops

import (
	"context"
	"fmt"
	goimage "image"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
	"github.com/go-go-golems/goja-cv/pkg/cv/text"
)

// cv.sheet (GCV-002 §7.9) is a layout op: one picture of many magnified
// crops, a cell (or a group of cells, one per view) per item, each labelled
// with its id and centre. Its size is fixed when it is built (a static max),
// it is evaluated at scale 1, and every cell is rendered through
// Evaluator.Render with the cell's window, the path a look takes, so views
// that end in a decoration draw thin outlines in cells too.

const (
	sheetLong      = MaxLookSide // a sheet fits a look
	sheetGap       = 6           // px between cells (groups)
	sheetInnerGap  = 2           // px between the cells of one group
	sheetLabelPad  = 2
	sheetEdgeGrey  = 0.08 // encoded grey of window parts outside the image
	sheetLabelBack = 0.05
)

type sheetLayout struct {
	cell, group  int // cell side and group width in px
	cols, rows   int // groups per row and rows that fit
	views, cells int // views per item, items laid out
}

func (l sheetLayout) fit() int { return l.cols * l.rows }

func (l sheetLayout) frame() graph.Frame {
	used := min(l.cols, l.cells)
	rows := (l.cells + l.cols - 1) / l.cols
	return graph.Frame{W: used*l.group + (used-1)*sheetGap, H: rows*l.cell + (rows-1)*sheetGap, Picture: true}
}

// viewCount is the number of views in a bound views argument (one node or
// a list of nodes).
func viewCount(v any) int {
	if l, ok := v.([]any); ok {
		return len(l)
	}
	return 1
}

// itemsBound is the item count of the sheet's items when it is known at
// build time (an array), or -1 (a detector's Regions node).
func itemsBound(a graph.Args) int {
	if !a.HasInputs() {
		return -1
	}
	if n, ok := graph.ItemCount(a.Input("items")); ok {
		return n
	}
	return -1
}

// layoutSheet lays out a sheet: cells for every item of an array, or as
// many as fit for a Regions node; bound is the item count when known (-1
// unknown).
func layoutSheet(a graph.Args, bound int) (sheetLayout, error) {
	size, zoom := a.DocLength("size"), a.Number("zoom")
	l := sheetLayout{cell: int(math.Round(size * zoom)), views: viewCount(a.Raw("views"))}
	l.group = l.views*l.cell + (l.views-1)*sheetInnerGap
	l.cols = (sheetLong + sheetGap) / (l.group + sheetGap)
	l.rows = (sheetLong + sheetGap) / (l.cell + sheetGap)
	if l.fit() == 0 {
		return l, graph.Errorf(graph.EArgRange, "sheet cells of %d px (size %g × zoom %g, %d views) do not fit a %d px look.", l.cell, size, zoom, l.views, sheetLong).
			WithFix(fmt.Sprintf("cv.sheet(items, { views, size: %g, zoom: %g })", size, math.Max(1, math.Floor(float64(sheetLong)/float64(l.views)/size))))
	}
	l.cells = l.fit()
	if bound >= 0 {
		l.cells = bound
	}
	if a.Has("max") {
		m := a.Int("max")
		if m > l.fit() {
			return l, graph.Errorf(graph.EArgRange, "sheet max %d: only %d cells of %d px fit a %d px look (%d per row, %d rows).", m, l.fit(), l.cell, sheetLong, l.cols, l.rows).
				WithFix(fmt.Sprintf("cv.sheet(items, { views, max: %d })", l.fit()))
		}
		l.cells = min(l.cells, m)
	}
	if l.cells > l.fit() {
		return l, graph.Errorf(graph.EArgRange, "sheet of %d items: only %d cells of %d px fit a %d px look.", l.cells, l.fit(), l.cell, sheetLong).
			WithFix(fmt.Sprintf("cv.sheet(items.slice(0, %d), { views })   // or a smaller size or zoom", l.fit()))
	}
	if l.cells == 0 {
		return l, graph.Errorf(graph.EArgType, "cv.sheet needs at least one item.").WithFix(`cv.sheet([[120, 80], [200, 40]], { views: img })`)
	}
	return l, nil
}

// cellLabel fits the label into a cell: font size 2, else 1, else the id
// alone.
func cellLabel(full, id string, cell int) (string, int) {
	for _, px := range []int{2, 1} {
		if text.Width(full, px)+2*sheetLabelPad <= cell-4 {
			return full, px
		}
	}
	if text.Width(id, 2)+2*sheetLabelPad <= cell-4 {
		return id, 2
	}
	return id, 1
}

// sheetBadgeDigits is the look number the sheet keeps room for: the badge a
// look burns into its top-left corner is as wide as the number's digits.
const sheetBadgeDigits = 999

// badgeZone is the top-left rectangle the look-number badge covers on a
// look of the w × h sheet at scale 1 (text.BurnNumber).
func badgeZone(w, h int) goimage.Rectangle { return text.NumberBox(sheetBadgeDigits, w, h) }

// placeLabel moves a cell label (lw × lh at the cell's top-left corner) off
// the badge zone: below it when the label still fits in the cell, else to
// its right. Labels clear of the zone stay where they are.
func placeLabel(gx, gy, lw, lh, cell int, badge goimage.Rectangle) (int, int) {
	if !goimage.Rect(gx, gy, gx+lw, gy+lh).Overlaps(badge) {
		return gx, gy
	}
	if badge.Max.Y+lh <= gy+cell {
		return gx, badge.Max.Y
	}
	return badge.Max.X, gy
}

func fill(b *buffer.Buffer, x0, y0, w, h int, v float32) {
	for c := 0; c < b.C; c++ {
		p := b.Plane(c)
		for y := max(0, y0); y < min(b.H, y0+h); y++ {
			for x := max(0, x0); x < min(b.W, x0+w); x++ {
				p[y*b.W+x] = v
			}
		}
	}
}

func runSheet(ctx context.Context, ec *graph.EvalCtx, _ []graph.Value, a graph.Args) (graph.Value, error) {
	l, err := layoutSheet(a, itemsBound(a))
	if err != nil {
		return nil, err
	}
	warn := func(w graph.Warning) { ec.Warn(w.Code, w.Message, "") }
	v, err := ec.Eval.Render(ctx, a.Input("items").ID, graph.RenderOptions{Scale: 1, OnWarn: warn})
	if err != nil {
		return nil, err
	}
	items := v.(*graph.RegionsValue).Items
	labels := a.String("labels")
	if len(items) > l.cells {
		ec.Warn(WSheetTruncated, fmt.Sprintf("%d items, %d shown (in list order).", len(items), l.cells),
			fmt.Sprintf("cv.sheet((await regions).sort((a, b) => b.contrast - a.contrast).slice(0, %d), { views })", l.cells))
		items = items[:l.cells]
	}
	views := a.Inputs("views") // in list order, repetitions kept
	f := ec.Node.Frame
	out := buffer.New(f.W, f.H, 3, 1)
	for i := range out.Pix {
		out.Pix[i] = compareBackground
	}
	size := a.DocLength("size")
	badge := badgeZone(f.W, f.H)
	for i, it := range items {
		gx := (i % l.cols) * (l.group + sheetGap)
		gy := (i / l.cols) * (l.cell + sheetGap)
		cx, cy := it.Centroid[0], it.Centroid[1]
		side := int(math.Ceil(math.Max(size, math.Max(it.Box[2], it.Box[3]))))
		win := graph.Frame{X: int(math.Round(cx - float64(side)/2)), Y: int(math.Round(cy - float64(side)/2)), W: side, H: side}
		scale := float64(l.cell) / float64(side)
		edge := false
		for k, vn := range views {
			cx := gx + k*(l.cell+sheetInnerGap)
			fill(out, cx, gy, l.cell, l.cell, sheetEdgeGrey)
			inter := graph.Intersect(win, vn.Frame)
			if inter != win {
				edge = true
			}
			if inter.Empty() {
				continue
			}
			v, err := ec.Eval.Render(ctx, vn.ID, graph.RenderOptions{Scale: scale, Window: &win, OnWarn: warn})
			if err != nil {
				return nil, err
			}
			cb := kernels.ToRGB(v.(*graph.ImageValue).Buf)
			if a.String("stretch") == "cell" {
				lum := cb.Luminance()
				lo, hi := kernels.Percentile(lum, 0.01), kernels.Percentile(cb.Luminance(), 0.99)
				if hi > lo {
					if cb, err = kernels.Levels(ctx, cb, lo, hi, 1); err != nil {
						return nil, err
					}
				}
			}
			ox := int(math.Round(float64(inter.X-win.X) * scale))
			oy := int(math.Round(float64(inter.Y-win.Y) * scale))
			kernels.Paste(out, cb, cx+ox, gy+oy) // clipped to the sheet; cells never overlap
		}
		if labels == "none" {
			continue
		}
		id := it.Text()
		full := fmt.Sprintf("%s (%d, %d)", id, int(math.Round(cx)), int(math.Round(cy)))
		if math.Abs(scale-a.Number("zoom")) > 1e-9 {
			full += " ×" + strconv.FormatFloat(scale, 'g', 3, 64)
		}
		if edge {
			full += " (edge)"
		}
		s, px := cellLabel(full, id, l.cell)
		lw, lh := text.Width(s, px)+2*sheetLabelPad, text.Height(px)+2*sheetLabelPad
		lx, ly := placeLabel(gx, gy, lw, lh, l.cell, badge) // the first cell's label clears the look number
		fill(out, lx, ly, lw, lh, sheetLabelBack)
		kernels.DrawText(out, lx+sheetLabelPad, ly+sheetLabelPad, s, px, [3]float32{0.95, 0.95, 0.95})
	}
	return graph.NewImage(out), nil
}

// sheetCaptionCells is the number of cells a sheet's caption lists.
const sheetCaptionCells = 40

// sheetCaption names the sheet's cells in its caption, so a model maps cells
// to items from text (GCV-005 §6.7, D11): for an array the label (else id) of
// each cell in cell order, "sheet(15 items; cells: 14 13 26 …)"; for a
// Regions node, whose ids are known only when it is evaluated, "sheet(cells
// in region order)". A label with whitespace is quoted.
func sheetCaption(a graph.Args) string {
	if !a.HasInputs() {
		return "sheet"
	}
	texts, ok := graph.ItemTexts(a.Input("items"))
	if !ok {
		return "sheet(cells in region order)"
	}
	cells := len(texts)
	if a.Has("max") {
		cells = min(cells, a.Int("max"))
	}
	list := make([]string, 0, min(cells, sheetCaptionCells))
	for _, t := range texts[:min(cells, sheetCaptionCells)] {
		if t == "" || strings.ContainsFunc(t, unicode.IsSpace) {
			t = strconv.Quote(t)
		}
		list = append(list, t)
	}
	s := fmt.Sprintf("sheet(%d items; cells: %s", len(texts), strings.Join(list, " "))
	if cells > sheetCaptionCells {
		s += fmt.Sprintf(" … (%d more)", cells-sheetCaptionCells)
	}
	return s + ")"
}

// WSheetTruncated: a sheet got more items than it has cells.
const WSheetTruncated = "SHEET_TRUNCATED"

func sheetOp() graph.OpDef {
	return graph.OpDef{
		Name: "sheet", On: graph.KindNone, Returns: graph.KindImage, Result: graph.DisplayEnc, Lossless: true, Channels: graph.Three,
		Space: graph.Display, // cells show what looks show
		Args: []graph.ArgSpec{
			itemsArg("items", graph.ItemsType+"|point[]",
				"a Regions node or an array of items: region items (centred on their centre, sized by their box), boxes [x, y, w, h] or { x, y, w, h }, circles { cx, cy, r }, points [x, y] or { cx, cy }; doc px"),
			{Name: "views", Type: "Image|Image[]", Doc: "what each cell shows: an image node, or a list (one cell per view, side by side, for each item)"},
			{Name: "size", Type: "number", Unit: graph.DocPx, Default: 64.0, Min: graph.F(4), Max: graph.F(1024),
				Doc: "side of the square window around each item, doc px (a larger box widens its window and lowers its scale)"},
			{Name: "zoom", Type: "number", Default: 4.0, Min: graph.F(1), Max: graph.F(graph.MaxRenderScale),
				Doc: "render scale of each cell: cells are size × zoom px"},
			{Name: "max", Type: "number", Optional: true, Min: graph.F(1),
				Doc: "cells laid out (default: as many as fit 1568 px; a longer region list is cut with SHEET_TRUNCATED)"},
			{Name: "labels", Type: "enum:id,none", Default: "id", Doc: "cell label: the item's label, else its id (ids may repeat: they are data), with the centre in doc px; none draws no labels (the caption still lists the cells)"},
			{Name: "stretch", Type: "enum:none,cell", Default: "none", Doc: "cell: stretch each cell by its own p01/p99 (levels({ auto: true }) per cell)"},
		},
		Validate: func(a graph.Args) error {
			if !a.Has("views") {
				return graph.Errorf(graph.EArgType, "cv.sheet needs views: the image (or images) each cell shows.").
					WithFix(`cv.sheet(regions, { views: img.view("speck") })`)
			}
			_, err := layoutSheet(a, itemsBound(a))
			return err
		},
		DocFrame: func(a graph.Args) graph.Frame {
			l, err := layoutSheet(a, itemsBound(a))
			if err != nil {
				return graph.Frame{Picture: true}
			}
			return l.frame()
		},
		AnalysisScale: func(graph.Args) float64 { return 1 },
		Short:         sheetCaption,
		Lazy:          true, // cells are rendered by Run through Evaluator.Render, window by window
		Run:           runSheet,
		Doc:           "A contact sheet: one picture of magnified crops around many items (a Regions node or an array of region items, boxes, circles or points), labelled with label (else id) and centre; cv.sheet(regions, { views: img.view(\"speck\"), size: 64, zoom: 4 }). 36 cells fit at the defaults (18 items with two views). The sheet is a picture: its pixels are not doc px. Its caption lists the cells' labels (else ids) in cell order for an array (sheet(15 items; cells: 14 13 26 …)), \"cells in region order\" for a Regions node: map cells to items from that text.",
		See:           []string{"crop", "view", "compare", "overlay"},
	}
}
