package ops

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
	"github.com/go-go-golems/goja-cv/pkg/cv/text"
)

// annotate (GCV-002 §7.8) draws marks given as data: circles, points, boxes,
// polylines and text, in absolute doc px. It is a decoration: in a zoomed
// look lines stay `width` image px and text stays readable.

var markShapes = []string{"circle", "point", "box", "polyline", "text"}

var markKeys = []string{"circle", "point", "box", "polyline", "text", "at", "color", "label"}

// markKeySynonyms are words agents use for the mark keys.
var markKeySynonyms = map[string]string{
	"circ": "circle", "ring": "circle", "line": "polyline", "lines": "polyline", "path": "polyline", "poly": "polyline",
	"rect": "box", "rectangle": "box", "bbox": "box", "dot": "point", "marker": "point", "cross": "point", "pt": "point",
	"pos": "at", "position": "at", "xy": "at", "colour": "color", "caption": "label", "name": "label", "string": "text",
}

const annotateFix = `img.annotate([{ circle: [x, y, r], color: "lime", label: "7" }, { point: [x, y] }, { box: [x, y, w, h] }, { polyline: [[x1, y1], [x2, y2]] }, { text: "hair?", at: [x, y] }])`

// mark is one parsed mark in doc px.
type mark struct {
	shape string
	pts   [][2]float64 // circle: centre; point: the point; box: corners; polyline: vertices; text: at
	r     float64      // circle radius
	text  string
	label string
	color [3]float32
}

func markErr(i int, format string, args ...any) *graph.Error {
	return graph.Errorf(graph.EArgType, "annotate marks[%d]: "+format, append([]any{i}, args...)...)
}

func num(v any) (float64, bool) { f, ok := v.(float64); return f, ok }

func pointOf(v any) ([2]float64, bool) {
	switch x := v.(type) {
	case []any:
		if len(x) == 2 {
			a, ok1 := num(x[0])
			b, ok2 := num(x[1])
			return [2]float64{a, b}, ok1 && ok2
		}
	case map[string]any:
		a, ok1 := num(x["cx"])
		b, ok2 := num(x["cy"])
		return [2]float64{a, b}, ok1 && ok2
	}
	return [2]float64{}, false
}

func boxOf(v any) ([4]float64, bool) {
	var raw []any
	switch x := v.(type) {
	case []any:
		raw = x
	case map[string]any:
		raw = []any{x["x"], x["y"], x["w"], x["h"]}
	}
	if len(raw) != 4 {
		return [4]float64{}, false
	}
	var out [4]float64
	for i, e := range raw {
		f, ok := num(e)
		if !ok {
			return out, false
		}
		out[i] = f
	}
	return out, true
}

// parseMarks reads the marks argument: a list of objects with exactly one
// shape key, an optional color and label.
func parseMarks(v any) ([]mark, error) {
	list, ok := v.([]any)
	if !ok {
		return nil, graph.Errorf(graph.EArgType, "annotate takes a list of marks.").WithFix(annotateFix)
	}
	out := make([]mark, 0, len(list))
	for i, e := range list {
		m, ok := e.(map[string]any)
		if !ok {
			return nil, markErr(i, "a mark is an object such as { circle: [x, y, r] }.").WithFix(annotateFix)
		}
		var shapes []string
		for _, k := range keysOf(m) {
			if contains(markShapes, k) {
				shapes = append(shapes, k)
				continue
			}
			if contains(markKeys, k) {
				continue
			}
			e := markErr(i, "unknown key %q. Marks have one shape (%s) and optionally color, label (and at for text).", k, strings.Join(markShapes, ", ")).
				WithValid(markKeys).WithFix(annotateFix)
			if sug, ok := markKeySynonyms[k]; ok {
				e.Hint = fmt.Sprintf("Did you mean %q?", sug)
			} else if sug := graph.Closest(k, markKeys); sug != "" {
				e.Hint = fmt.Sprintf("Did you mean %q?", sug)
			}
			return nil, e
		}
		if len(shapes) != 1 {
			return nil, markErr(i, "a mark needs exactly one shape key of %s; it has %d.", strings.Join(markShapes, ", "), len(shapes)).
				WithValid(markShapes).WithFix(annotateFix)
		}
		mk := mark{shape: shapes[0], color: [3]float32{1, 0.9, 0.1}}
		if c, ok := m["color"]; ok {
			col, err := parseColor(c, fmt.Sprintf("annotate marks[%d].color", i))
			if err != nil {
				return nil, err
			}
			mk.color = col
		}
		if l, ok := m["label"]; ok {
			s, ok := l.(string)
			if !ok {
				s = fmt.Sprint(l)
			}
			mk.label = s
		}
		val := m[mk.shape]
		switch mk.shape {
		case "circle":
			var c [3]float64
			okc := false
			switch x := val.(type) {
			case []any:
				if len(x) == 3 {
					c[0], okc = num(x[0])
					var ok1, ok2 bool
					c[1], ok1 = num(x[1])
					c[2], ok2 = num(x[2])
					okc = okc && ok1 && ok2
				}
			case map[string]any:
				var ok1, ok2 bool
				c[0], okc = num(x["cx"])
				c[1], ok1 = num(x["cy"])
				c[2], ok2 = num(x["r"])
				okc = okc && ok1 && ok2
			}
			if !okc || c[2] < 0 {
				return nil, markErr(i, "circle is [x, y, r] in doc px (centre and radius).").WithFix(`{ circle: [1337, 131, 4] }`)
			}
			mk.pts, mk.r = [][2]float64{{c[0], c[1]}}, c[2]
		case "point":
			p, ok := pointOf(val)
			if !ok {
				return nil, markErr(i, "point is [x, y] or an object with cx, cy (doc px).").WithFix(`{ point: [1683, 123] }`)
			}
			mk.pts = [][2]float64{p}
		case "box":
			b, ok := boxOf(val)
			if !ok {
				return nil, markErr(i, "box is [x, y, w, h] or an object with x, y, w, h (doc px).").WithFix(`{ box: [1120, 455, 18, 18] }`)
			}
			mk.pts = [][2]float64{{b[0], b[1]}, {b[0] + b[2], b[1] + b[3]}}
		case "polyline":
			l, ok := val.([]any)
			if !ok || len(l) < 2 {
				return nil, markErr(i, "polyline is a list of at least two points [[x, y], …].").WithFix(`{ polyline: [[1475, 395], [1490, 410]] }`)
			}
			for _, p := range l {
				pt, ok := pointOf(p)
				if !ok {
					return nil, markErr(i, "polyline points are [x, y].").WithFix(`{ polyline: [[1475, 395], [1490, 410]] }`)
				}
				mk.pts = append(mk.pts, pt)
			}
		case "text":
			s, ok := val.(string)
			at, ok2 := pointOf(m["at"])
			if !ok || !ok2 {
				return nil, markErr(i, "text is a string with at: [x, y], the top-left corner in doc px.").WithFix(`{ text: "hair?", at: [1500, 380] }`)
			}
			mk.text, mk.pts = s, [][2]float64{at}
		}
		out = append(out, mk)
	}
	return out, nil
}

// pen draws into an RGB buffer with a square brush of w image px.
type pen struct {
	b   *buffer.Buffer
	w   int
	col [3]float32
}

func (p pen) dot(x, y float64) {
	h := float64(p.w) / 2
	for yy := int(math.Floor(y - h + 0.5)); yy < int(math.Floor(y-h+0.5))+p.w; yy++ {
		for xx := int(math.Floor(x - h + 0.5)); xx < int(math.Floor(x-h+0.5))+p.w; xx++ {
			if xx >= 0 && yy >= 0 && xx < p.b.W && yy < p.b.H {
				for c := 0; c < 3; c++ {
					p.b.Plane(c)[yy*p.b.W+xx] = p.col[c]
				}
			}
		}
	}
}

// line draws from (x0, y0) to (x1, y1) (image px, continuous coordinates).
func (p pen) line(x0, y0, x1, y1 float64) {
	n := int(math.Ceil(math.Max(math.Abs(x1-x0), math.Abs(y1-y0)) * 2))
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(max(n, 1))
		p.dot(x0+(x1-x0)*t, y0+(y1-y0)*t)
	}
}

// ring draws a circle of radius r around (cx, cy) whose stroke is w px wide.
func (p pen) ring(cx, cy, r float64) {
	hw := float64(p.w) / 2
	for y := int(math.Floor(cy - r - hw - 1)); y <= int(math.Ceil(cy+r+hw+1)); y++ {
		for x := int(math.Floor(cx - r - hw - 1)); x <= int(math.Ceil(cx+r+hw+1)); x++ {
			d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
			if math.Abs(d-r) < hw && x >= 0 && y >= 0 && x < p.b.W && y < p.b.H {
				for c := 0; c < 3; c++ {
					p.b.Plane(c)[y*p.b.W+x] = p.col[c]
				}
			}
		}
	}
}

func drawMarks(ctx context.Context, img *graph.ImageValue, marks []mark, width int) (*buffer.Buffer, error) {
	out := kernels.ToRGB(img.Buf).Clone()
	t := graph.RasterOf(img)
	ix := func(x float64) float64 { return (x - t.OX) * t.SX }
	iy := func(y float64) float64 { return (y - t.OY) * t.SY }
	const px = 2
	for i, m := range marks {
		if i%256 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		p := pen{b: out, w: width, col: m.color}
		var lx, ly float64 // where the label goes: right of the shape's top
		switch m.shape {
		case "circle":
			cx, cy, r := ix(m.pts[0][0]), iy(m.pts[0][1]), math.Max(m.r*t.SX, 2)
			p.ring(cx, cy, r)
			lx, ly = cx+r+3, cy-r-float64(text.Height(px))
		case "point":
			x, y := ix(m.pts[0][0]), iy(m.pts[0][1])
			for _, d := range [][4]float64{{-6, 0, -2, 0}, {2, 0, 6, 0}, {0, -6, 0, -2}, {0, 2, 0, 6}} {
				p.line(x+d[0], y+d[1], x+d[2], y+d[3])
			}
			lx, ly = x+5, y-5-float64(text.Height(px))
		case "box":
			x0, y0 := int(math.Floor(ix(m.pts[0][0]))), int(math.Floor(iy(m.pts[0][1])))
			x1, y1 := int(math.Ceil(ix(m.pts[1][0])))-1, int(math.Ceil(iy(m.pts[1][1])))-1
			kernels.DrawRect(out, x0-width, y0-width, x1+width, y1+width, width, m.color, 1)
			lx, ly = float64(x1+width+2), float64(y0-width-text.Height(px)-1)
		case "polyline":
			for k := 1; k < len(m.pts); k++ {
				p.line(ix(m.pts[k-1][0]), iy(m.pts[k-1][1]), ix(m.pts[k][0]), iy(m.pts[k][1]))
			}
			last := m.pts[len(m.pts)-1]
			lx, ly = ix(last[0])+3, iy(last[1])+3
		case "text":
			kernels.DrawText(out, int(math.Round(ix(m.pts[0][0]))), int(math.Round(iy(m.pts[0][1]))), m.text, px, m.color)
			continue
		}
		if m.label != "" {
			kernels.DrawText(out, int(math.Round(lx)), int(math.Max(0, math.Round(ly))), m.label, px, m.color)
		}
	}
	return out, nil
}

func annotateOp() graph.OpDef {
	return graph.OpDef{
		Name: "annotate", On: graph.KindImage, Returns: graph.KindImage, Class: graph.Decoration,
		Space: graph.SpaceRule{Default: buffer.SpaceEncoded},
		Args: []graph.ArgSpec{
			{Name: "marks", Type: "any", Positional: true, Static: true,
				Doc: "list of marks in absolute doc px: { circle: [x, y, r] }, { point: [x, y] }, { box: [x, y, w, h] }, { polyline: [[x, y], …] }, { text, at: [x, y] }; each may have color (display colour) and label"},
			{Name: "width", Type: "number", Default: 1.0, Min: graph.F(1), Max: graph.F(8), Doc: "line width in image px (stays thin at any zoom)"},
		},
		Validate: func(a graph.Args) error {
			_, err := parseMarks(a.Raw("marks"))
			return err
		},
		Run: func(ctx context.Context, _ *graph.EvalCtx, in []graph.Value, a graph.Args) (graph.Value, error) {
			marks, err := parseMarks(a.Raw("marks"))
			if err != nil {
				return nil, err
			}
			out, err := drawMarks(ctx, image(in[0]), marks, a.Int("width"))
			if err != nil {
				return nil, err
			}
			return graph.NewImage(out), nil
		},
		Doc: "Draw marks given as data (circles, points, boxes, polylines, text) in absolute doc px, for looking. A decoration: in a zoomed look lines stay width image px and text stays readable; marks outside a crop are skipped. Colours are display colours.",
		See: []string{"overlay", "crop", "sheet"},
	}
}
