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
	"math"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/go-go-golems/go-go-goja/pkg/resultcontent"
	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/text"
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
	Label   string        // shown in the caption; default: the node's label
	Zoom    float64       // render scale in [1, 16]; 0 = fit to MaxSide
	Interp  buffer.Interp // upsampling when zoomed
	MaxSide int           // long side in image px when fitting; 0 = the engine's look size (1568)
}

// OptionsFrom reads the look options bound by graph.BindOptions against the
// look function's ArgSpecs (ops.LookArgs).
func OptionsFrom(a graph.Args) (Options, error) {
	var o Options
	if a.Has("label") {
		o.Label = a.String("label")
	}
	if a.Has("zoom") {
		o.Zoom = a.Number("zoom")
	}
	if a.Has("interp") {
		o.Interp, _ = buffer.ParseInterp(a.String("interp"))
	}
	if a.Has("maxSide") {
		f := a.Number("maxSide")
		if f != float64(int(f)) {
			return o, graph.Errorf(graph.EArgType, "look maxSide must be an integer number of image px; got %g.", f).
				WithFix(fmt.Sprintf("node.look({ maxSide: %d })", int(f)))
		}
		o.MaxSide = int(f)
	}
	if o.Zoom != 0 && o.MaxSide != 0 {
		return o, graph.Errorf(graph.EArgType, "look takes zoom or maxSide, not both: zoom sets the scale; maxSide fits it.").
			WithFix(fmt.Sprintf("node.look({ zoom: %s })", fmtZoom(o.Zoom)))
	}
	return o, nil
}

func fmtZoom(z float64) string { return strconv.FormatFloat(z, 'g', 4, 64) }

// Look is a lazily rendered image of one node.
type Look struct {
	eng    *graph.Engine
	node   graph.NodeID
	root   graph.NodeID // what is rendered: node, or convert(node → encoded)
	kind   graph.Kind
	label  string
	scale  float64 // render scale: image px per doc px; 1 for charts
	interp buffer.Interp
	w, h   int  // rendered size
	chart  bool // a Value node rendered as a chart
	n      atomic.Int64

	mu    sync.Mutex
	warns []graph.Warning // evaluation-time warnings of the render
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
	l := &Look{eng: eng, node: id, kind: n.Kind, label: strings.TrimSpace(opt.Label), interp: opt.Interp}
	if l.label == "" {
		l.label = graph.Label(n)
	}
	l.root = id
	switch n.Kind {
	case graph.KindImage, graph.KindMask:
		root, err := eng.Display(id)
		if err != nil {
			return nil, err
		}
		l.root = root
		l.scale = eng.ScaleForLong(id, maxLong)
		if opt.Zoom != 0 {
			l.scale = graph.QuantizeScale(opt.Zoom)
		}
		l.w, l.h = buffer.ScaledSize(n.Frame.W, n.Frame.H, l.scale)
		if limit := eng.Options().LookLong; max(l.w, l.h) > limit {
			return nil, zoomTooLarge(n, opt.Zoom, limit)
		}
	case graph.KindValue:
		if n.Op.Name != "histogram" {
			return nil, lookKindError(n)
		}
		if opt.Zoom != 0 {
			return nil, graph.Errorf(graph.EArgType, "look of a chart takes no zoom: charts have no doc px.").WithFix("node.look()")
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

// zoomTooLarge is E_ARG_RANGE for a zoomed look above the size limit, with
// both fixes: a smaller crop at the same zoom, or the same image at a lower
// zoom.
func zoomTooLarge(n *graph.Node, zoom float64, limit int) error {
	side := int(float64(limit) / zoom)
	long := max(n.Frame.W, n.Frame.H)
	fix := fmt.Sprintf("node.crop([x, y, %d, %d]).look({ zoom: %s })", min(side, n.Frame.W), min(side, n.Frame.H), fmtZoom(zoom))
	if z := float64(limit) / float64(long); z >= 1 {
		fix += fmt.Sprintf("   // or node.look({ zoom: %s })", fmtZoom(math.Floor(z*100)/100))
	} else {
		fix += "   // or node.look() to fit the whole image"
	}
	return graph.Errorf(graph.EArgRange, "look({ zoom: %s }) of %s (%s doc px) would be %d px on its long side; looks are at most %d px.",
		fmtZoom(zoom), graph.Label(n), n.Frame.Size(), int(math.Round(float64(long)*zoom)), limit).
		WithHint("Zoom a crop: at zoom z a look shows at most " + strconv.Itoa(limit) + "/z doc px per side.").WithFix(fix)
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

// DocPxPerImagePx formats 1/scale for captions: "4.0", "6.13", "0.125".
func DocPxPerImagePx(scale float64) string {
	if scale <= 0 {
		return "?"
	}
	if scale > 1 {
		return strconv.FormatFloat(1/scale, 'g', 3, 64)
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
	caption := l.Caption()
	if ws := l.Warnings(); len(ws) > 0 {
		parts := make([]string, len(ws))
		for i, w := range ws {
			parts[i] = w.Code + ": " + w.Message
		}
		caption += " — warnings: " + strings.Join(parts, "; ")
	}
	return []resultcontent.Content{{MIMEType: mime, Data: data, Caption: caption}}, nil
}

// Warnings are the evaluation-time warnings of the last render (SHEET_TRUNCATED).
func (l *Look) Warnings() []graph.Warning {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]graph.Warning(nil), l.warns...)
}

func (l *Look) warn(w graph.Warning) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, x := range l.warns {
		if x == w {
			return
		}
	}
	l.warns = append(l.warns, w)
}

// Render evaluates and encodes the look, returning the bytes and MIME type.
func (l *Look) Render(ctx context.Context) ([]byte, string, error) {
	m, err := l.image(ctx)
	if err != nil {
		return nil, "", err
	}
	text.BurnNumber(m, l.Number())
	return Encode(m, l.chart)
}

// image evaluates the look's node and returns its 8-bit image, before the
// look number is burned in and before encoding. A look shows encoded values:
// images are rendered through convert(node → encoded) (no node when the
// values already are encoded), masks and derived images as they are.
func (l *Look) image(ctx context.Context) (draw.Image, error) {
	v, err := l.eng.Render(ctx, l.root, graph.RenderOptions{Scale: l.scale, Interp: l.interp, OnWarn: l.warn})
	if err != nil {
		return nil, err
	}
	switch x := v.(type) {
	case *graph.ImageValue:
		img, err := buffer.Quantize(ctx, x.Buf, 8)
		if err != nil {
			return nil, errors.Wrap(err, "look: convert")
		}
		dm, ok := img.(draw.Image)
		if !ok {
			return nil, errors.Errorf("look: unexpected image type %T", img)
		}
		return dm, nil
	case *graph.DataValue:
		return renderChart(x.Data, l.w, l.h)
	}
	return nil, errors.Errorf("look: unexpected value %T", v)
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
