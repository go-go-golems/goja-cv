package look

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"strconv"

	"github.com/go-go-golems/goja-cv/pkg/cv/text"
)

// Rulers and grid (GCV-002 §7.7). Rulers are drawn into margins around the
// image: 28 px on the top and the left, and on the bottom and the right when
// the image is at least 256 px on that side. The side margins are widened to
// fit the longest y label. Tick labels are absolute doc px (from the
// frame): the tick at doc x = t is at the left edge of doc pixel t, image x =
// margin + (t − origin) · scale. Labels are thinned so that adjacent labels
// are at least their width + 4 px apart.

const (
	rulerMargin   = 28  // top and bottom margins, the minimum of the side margins
	rulerBothSide = 256 // an image side at least this long gets rulers on both edges
	tickLen       = 6   // tick mark length in image px
	labelPx       = 2   // font size of tick labels
	labelGap      = 4   // minimum gap between adjacent labels
	autoTickPx    = 40  // ruler: true picks the smallest power-of-2 spacing at least this many image px apart
	gridOpacity   = 0.35
)

var (
	rulerBG   = color.Gray{Y: 24}
	rulerInk  = color.Gray{Y: 225}
	gridLight = color.Gray{Y: 255}
	gridDark  = color.Gray{Y: 0}
)

// axis describes one image axis in doc px.
type axis struct {
	origin float64 // doc px at image px 0
	scale  float64 // image px per doc px
	n      int     // image px
}

func (a axis) px(doc float64) int { return int(math.Round((doc - a.origin) * a.scale)) }

// tick is one ruler tick: its doc coordinate, image px offset and whether it
// is labelled.
type tick struct {
	doc     int
	px      int
	labeled bool
}

// autoTick is the tick spacing of ruler: true at scale s: 1, 2, 4, 8, … doc
// px, the first at least autoTickPx image px apart.
func autoTick(s float64) float64 {
	t := 1.0
	for t*s < autoTickPx && t < 1<<20 {
		t *= 2
	}
	return t
}

// ticks lays out the ticks of spacing t (doc px) on axis a; labels are
// thinned to every k-th tick, k in 1, 2, 5, 10, 20, 50, …, so that
// labelled ticks are at least need image px apart.
func ticks(a axis, t float64, need int) []tick {
	if t <= 0 {
		return nil
	}
	k := 1
	for i := 1; float64(k)*t*a.scale < float64(need) && k < 1<<20; i++ {
		k = []int{1, 2, 5}[i%3] * int(math.Pow(10, float64(i/3)))
	}
	start := math.Ceil(a.origin - 1e-9) // ticks count from the first whole doc px of the axis
	end := a.origin + float64(a.n)/a.scale
	var out []tick
	for i := 0; start+float64(i)*t <= end+1e-9; i++ {
		d := start + float64(i)*t
		px := a.px(d)
		if px < 0 || px > a.n {
			continue
		}
		out = append(out, tick{doc: int(math.Round(d)), px: px, labeled: i%k == 0})
	}
	return out
}

func maxLabelWidth(ts []tick) int {
	w := 0
	for _, t := range ts {
		w = max(w, text.Width(strconv.Itoa(t.doc), labelPx))
	}
	return w
}

// rulerLayout is where the rulers go around a w × h image.
type rulerLayout struct {
	x, y                     axis
	xt, yt                   []tick
	top, bottom, left, right int
}

func layoutRulers(x, y axis, spacing float64) rulerLayout {
	l := rulerLayout{x: x, y: y, top: rulerMargin}
	if spacing < 0 {
		spacing = autoTick(min(x.scale, y.scale))
	}
	// x labels: width of the widest label + gap; y labels: height + gap
	probe := ticks(x, spacing, 1)
	l.xt = ticks(x, spacing, maxLabelWidth(probe)+labelGap)
	l.yt = ticks(y, spacing, text.Height(labelPx)+labelGap)
	l.left = max(rulerMargin, maxLabelWidth(l.yt)+tickLen+6)
	if y.n >= rulerBothSide {
		l.bottom = rulerMargin
	}
	if x.n >= rulerBothSide {
		l.right = l.left
	}
	return l
}

// addRulers returns m inside margins with ticks and absolute labels, and
// the grid drawn over the image area; spacing < 0 picks the tick spacing,
// 0 draws no ruler; grid 0 draws no grid.
func addRulers(m draw.Image, x, y axis, spacing, grid float64) draw.Image {
	if grid > 0 {
		drawGrid(m, x, y, grid)
	}
	if spacing == 0 {
		return m
	}
	l := layoutRulers(x, y, spacing)
	b := m.Bounds()
	out := newLike(m, b.Dx()+l.left+l.right, b.Dy()+l.top+l.bottom)
	draw.Draw(out, out.Bounds(), image.NewUniform(rulerBG), image.Point{}, draw.Src)
	draw.Draw(out, image.Rect(l.left, l.top, l.left+b.Dx(), l.top+b.Dy()), m, b.Min, draw.Src)
	lh := text.Height(labelPx)
	for _, t := range l.xt {
		px := l.left + t.px
		vline(out, px, l.top-tickLen, l.top-1)
		if l.bottom > 0 {
			vline(out, px, l.top+b.Dy(), l.top+b.Dy()+tickLen-1)
		}
		if !t.labeled {
			continue
		}
		s := strconv.Itoa(t.doc)
		w := text.Width(s, labelPx)
		x0 := min(max(px-w/2, l.left), out.Bounds().Dx()-w) // the top-left corner keeps the look number
		text.Draw(out, x0, l.top-tickLen-2-lh, s, labelPx, rulerInk, nil)
		if l.bottom > 0 {
			text.Draw(out, x0, l.top+b.Dy()+tickLen+2, s, labelPx, rulerInk, nil)
		}
	}
	for _, t := range l.yt {
		py := l.top + t.px
		hline(out, l.left-tickLen, l.left-1, py)
		if l.right > 0 {
			hline(out, l.left+b.Dx(), l.left+b.Dx()+tickLen-1, py)
		}
		if !t.labeled {
			continue
		}
		s := strconv.Itoa(t.doc)
		w := text.Width(s, labelPx)
		y0 := min(max(py-lh/2, l.top), out.Bounds().Dy()-lh)
		text.Draw(out, l.left-tickLen-2-w, y0, s, labelPx, rulerInk, nil)
		if l.right > 0 {
			text.Draw(out, l.left+b.Dx()+tickLen+2, y0, s, labelPx, rulerInk, nil)
		}
	}
	return out
}

func vline(m draw.Image, x, y0, y1 int) {
	for y := y0; y <= y1; y++ {
		m.Set(x, y, rulerInk)
	}
}

func hline(m draw.Image, x0, x1, y int) {
	for x := x0; x <= x1; x++ {
		m.Set(x, y, rulerInk)
	}
}

// drawGrid draws 1 px lines every grid doc px over the image at 35 %
// opacity: white over dark pixels, black over light ones.
func drawGrid(m draw.Image, x, y axis, grid float64) {
	b := m.Bounds()
	mix := func(px, py int) {
		r, g, bl, _ := m.At(px, py).RGBA()
		lum := (299*r + 587*g + 114*bl) / 1000 >> 8
		ink := gridLight
		if lum >= 128 {
			ink = gridDark
		}
		m.Set(px, py, blend(m.At(px, py), ink, gridOpacity))
	}
	for _, t := range ticks(x, grid, 0) {
		if px := b.Min.X + t.px; px < b.Max.X {
			for py := b.Min.Y; py < b.Max.Y; py++ {
				mix(px, py)
			}
		}
	}
	for _, t := range ticks(y, grid, 0) {
		if py := b.Min.Y + t.px; py < b.Max.Y {
			for px := b.Min.X; px < b.Max.X; px++ {
				mix(px, py)
			}
		}
	}
}

func blend(under color.Color, ink color.Gray, a float64) color.Color {
	r, g, b, _ := under.RGBA()
	f := func(v uint32) uint8 {
		return uint8(math.Round(float64(v>>8)*(1-a) + float64(ink.Y)*a)) // #nosec G115 -- a blend of two 8-bit values
	}
	if _, gray := under.(color.Gray); gray {
		return color.Gray{Y: f(r)}
	}
	return color.RGBA{R: f(r), G: f(g), B: f(b), A: 255}
}

// newLike allocates an image of the same type as m (Gray stays Gray).
func newLike(m image.Image, w, h int) draw.Image {
	if _, ok := m.(*image.Gray); ok {
		return image.NewGray(image.Rect(0, 0, w, h))
	}
	return image.NewNRGBA(image.Rect(0, 0, w, h))
}
