// Package look implements looks: images an evaluation returns to the agent.
//
// A Look names a graph node and the size to render it at. It implements
// resultcontent.Provider, so a look returned from an eval (alone, or inside
// an object or array) is delivered as image content by go-go-goja's result
// content machinery. Rendering is lazy and happens off the runtime owner
// goroutine: the node is evaluated at look scale, the look number is burned
// into the top-left corner, and the result is encoded as JPEG (q85) for
// colour images and PNG for single-channel images, masks and charts.
//
// A Look never holds pixels or encoded bytes in exported fields: hosts may
// json.Marshal values that contain looks (persisted bindings).
package look

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/go-go-golems/go-go-goja/pkg/resultcontent"
	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/pkg/errors"
)

const (
	// JPEGQuality is the quality of colour looks.
	JPEGQuality = 85
	// MinSide is the smallest maxSide a look accepts.
	MinSide = 16
)

// Options configure a look.
type Options struct {
	Label   string // shown in the caption; default: the node's label
	MaxSide int    // long side in image px; 0 = the engine's look size (1568)
}

// Look is a lazily rendered image of one node.
type Look struct {
	eng   *graph.Engine
	node  graph.NodeID
	kind  graph.Kind
	label string
	scale float64 // buffer px per doc px; 1 for charts
	w, h  int     // rendered size
	chart bool    // a Value node rendered as a chart
	n     atomic.Int64
}

var (
	_ resultcontent.Provider  = &Look{}
	_ resultcontent.Describer = &Look{}
)

// New creates a look of node id. Image and Mask nodes render as images;
// Value nodes whose data has a numeric "counts" array (histogram) render as a
// bar chart. Other kinds are E_WRONG_KIND. It does not evaluate anything.
func New(eng *graph.Engine, id graph.NodeID, opt Options) (*Look, error) {
	n, ok := eng.Node(id)
	if !ok {
		return nil, graph.Errorf(graph.EInternal, "unknown node #%d", id)
	}
	maxLong := eng.Options().LookLong
	if opt.MaxSide != 0 {
		if opt.MaxSide < MinSide || opt.MaxSide > maxLong {
			return nil, graph.Errorf(graph.EArgRange, "look maxSide must be in [%d, %d]; got %d.", MinSide, maxLong, opt.MaxSide).
				WithFix(fmt.Sprintf("node.look({ maxSide: %d })", min(max(opt.MaxSide, MinSide), maxLong)))
		}
		maxLong = opt.MaxSide
	}
	l := &Look{eng: eng, node: id, kind: n.Kind, label: strings.TrimSpace(opt.Label)}
	if l.label == "" {
		l.label = graph.Label(n)
	}
	switch n.Kind {
	case graph.KindImage, graph.KindMask:
		l.scale = eng.ScaleForLong(id, maxLong)
		l.w, l.h = buffer.ScaledSize(n.DocW, n.DocH, l.scale)
	case graph.KindValue:
		if n.Op.Name != "histogram" {
			return nil, lookKindError(n)
		}
		l.chart, l.scale = true, 1
		l.w, l.h = chartSize(maxLong)
	case graph.KindRegions, graph.KindParam, graph.KindNone:
		return nil, lookKindError(n)
	default:
		return nil, lookKindError(n)
	}
	return l, nil
}

func lookKindError(n *graph.Node) error {
	e := graph.Errorf(graph.EWrongKind, "look() renders Image and Mask nodes (and histograms); %s is %s node.", graph.Label(n), graph.A(string(n.Kind)))
	e.Hint = "Await a Value node to read its data; look at the image it was computed from."
	switch n.Kind {
	case graph.KindValue:
		e.Fix = "return { value: await node, look: img.look() }"
	case graph.KindRegions:
		e.Hint = "Regions are areas, not an image: draw them as a mask over the image."
		e.Fix = "img.overlay(regions).look(\"regions\")"
	case graph.KindImage, graph.KindMask, graph.KindParam, graph.KindNone:
		// the generic hint applies
	}
	return e
}

// Node is the node the look renders.
func (l *Look) Node() graph.NodeID { return l.node }

// Label is the look's label.
func (l *Look) Label() string { return l.label }

// Size is the rendered size in image px.
func (l *Look) Size() (int, int) { return l.w, l.h }

// Scale is buffer px per doc px (1 for charts).
func (l *Look) Scale() float64 { return l.scale }

// Number is the look number (0 until the session assigns one).
func (l *Look) Number() int { return int(l.n.Load()) }

// AssignNumber sets the look number if none is set yet and returns the
// look's number.
func (l *Look) AssignNumber(n int) int {
	l.n.CompareAndSwap(0, int64(n))
	return l.Number()
}

// DocPxPerImagePx formats 1/scale for captions: "4.0", "6.13".
func DocPxPerImagePx(scale float64) string {
	if scale <= 0 {
		return "?"
	}
	s := strconv.FormatFloat(1/scale, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	if strings.HasSuffix(s, ".") {
		s += "0"
	}
	return s
}

// Caption is "look N: label (W×H, 1 image px = S doc px)"; "look: …" before
// a number is assigned. Charts say "chart" instead of the scale.
func (l *Look) Caption() string {
	head := "look"
	if n := l.Number(); n > 0 {
		head = fmt.Sprintf("look %d", n)
	}
	if l.chart {
		return fmt.Sprintf("%s: %s (%d×%d chart)", head, l.label, l.w, l.h)
	}
	return fmt.Sprintf("%s: %s (%d×%d, 1 image px = %s doc px)", head, l.label, l.w, l.h, DocPxPerImagePx(l.scale))
}

// ResultPlaceholder implements resultcontent.Describer.
func (l *Look) ResultPlaceholder() string { return l.Caption() }

// String is the caption in brackets.
func (l *Look) String() string { return "[" + l.Caption() + "]" }

// ResultContent implements resultcontent.Provider: it evaluates the node at
// look scale and encodes it. It runs off the runtime owner goroutine.
func (l *Look) ResultContent(ctx context.Context) ([]resultcontent.Content, error) {
	data, mime, err := l.Render(ctx)
	if err != nil {
		return nil, err
	}
	return []resultcontent.Content{{MIMEType: mime, Data: data, Caption: l.Caption()}}, nil
}

// Render evaluates and encodes the look, returning the bytes and MIME type.
func (l *Look) Render(ctx context.Context) ([]byte, string, error) {
	v, err := l.eng.Evaluate(ctx, l.node, graph.EvalOptions{Scale: l.scale})
	if err != nil {
		return nil, "", err
	}
	var m draw.Image
	switch x := v.(type) {
	case *graph.ImageValue:
		img, err := buffer.ToImage(ctx, x.Buf, 8)
		if err != nil {
			return nil, "", errors.Wrap(err, "look: convert")
		}
		dm, ok := img.(draw.Image)
		if !ok {
			return nil, "", errors.Errorf("look: unexpected image type %T", img)
		}
		m = dm
	case *graph.DataValue:
		cm, err := renderChart(x.Data, l.w, l.h)
		if err != nil {
			return nil, "", err
		}
		m = cm
	default:
		return nil, "", errors.Errorf("look: unexpected value %T", v)
	}
	BurnNumber(m, l.Number())
	return Encode(m, l.chart)
}

// Encode writes m as JPEG q85, or PNG for single-channel images and when
// forcePNG is set (charts).
func Encode(m image.Image, forcePNG bool) ([]byte, string, error) {
	var buf bytes.Buffer
	if _, gray := m.(*image.Gray); gray || forcePNG {
		if err := png.Encode(&buf, m); err != nil {
			return nil, "", errors.Wrap(err, "look: encode png")
		}
		return buf.Bytes(), "image/png", nil
	}
	if err := jpeg.Encode(&buf, m, &jpeg.Options{Quality: JPEGQuality}); err != nil {
		return nil, "", errors.Wrap(err, "look: encode jpeg")
	}
	return buf.Bytes(), "image/jpeg", nil
}
