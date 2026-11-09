package look

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"strconv"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/cv/text"
)

// chartSize is the size of a histogram chart for a look of long side maxLong.
func chartSize(maxLong int) (int, int) {
	w := min(maxLong, 512)
	return w, max(w*2/5, MinSide)
}

// column is one field of a value's rows ({ rows: [{ key: v }, …] }) as a
// list, nil where a row lacks it.
func column(m map[string]any, key string) []any {
	rows, _ := m["rows"].([]any)
	if len(rows) == 0 {
		return nil
	}
	out := make([]any, len(rows))
	for i, r := range rows {
		if row, ok := r.(map[string]any); ok {
			out[i] = row[key]
		}
	}
	return out
}

// renderChart draws a histogram's rows (count per bin) as a bar chart on
// white.
func renderChart(data any, w, h int) (draw.Image, error) {
	m, _ := data.(map[string]any)
	counts := column(m, "count")
	if len(counts) == 0 {
		return nil, graph.Errorf(graph.EWrongKind, "look(): this Value has no histogram counts to chart.")
	}
	vals := make([]float64, len(counts))
	peak := 0.0
	for i, c := range counts {
		f, _ := c.(float64)
		vals[i] = f
		peak = max(peak, f)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	if peak <= 0 {
		return img, nil
	}
	bar := color.RGBA{R: 40, G: 40, B: 40, A: 255}
	axis := color.RGBA{R: 180, G: 180, B: 180, A: 255}
	plotH := h - 4
	for x := 0; x < w; x++ {
		img.Set(x, h-2, axis)
		// bins covering column x (columns may span several bins or part of one)
		b0 := x * len(vals) / w
		b1 := max((x+1)*len(vals)/w, b0+1)
		v := 0.0
		for b := b0; b < b1 && b < len(vals); b++ {
			v = max(v, vals[b])
		}
		top := h - 3 - int(v/peak*float64(plotH-1))
		for y := top; y <= h-3; y++ {
			img.Set(x, y, bar)
		}
	}
	return img, nil
}

// lineSeries are the profile series a line chart draws, with their colours.
var lineSeries = []struct {
	key string
	c   color.RGBA
}{
	{"meanR", color.RGBA{R: 200, G: 40, B: 40, A: 255}},
	{"meanG", color.RGBA{R: 30, G: 150, B: 40, A: 255}},
	{"meanB", color.RGBA{R: 40, G: 70, B: 210, A: 255}},
	{"meanL", color.RGBA{R: 20, G: 20, B: 20, A: 255}},
}

// chartLabelPx is the font size of chart axis labels.
const chartLabelPx = 2

// renderLines draws a profile's rows ({ t, meanL, meanR, … }) as a line chart on
// white: t (doc px) on x, one line per channel and meanL in black, gaps
// where samples are null; axis labels give the t and value ranges.
func renderLines(data any, w, h int) (draw.Image, error) {
	rm, _ := data.(map[string]any)
	m := map[string]any{"t": column(rm, "t")}
	for _, s := range lineSeries {
		m[s.key] = column(rm, s.key)
	}
	ts, ok := m["t"].([]any)
	if !ok || len(ts) == 0 {
		return nil, graph.Errorf(graph.EWrongKind, "look(): this Value has no profile samples (t) to chart.")
	}
	num := func(v any) (float64, bool) {
		f, ok := v.(float64)
		return f, ok && !math.IsNaN(f)
	}
	t0, _ := num(ts[0])
	t1, _ := num(ts[len(ts)-1])
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, s := range lineSeries {
		vals, _ := m[s.key].([]any)
		for _, v := range vals {
			if f, ok := num(v); ok {
				lo, hi = math.Min(lo, f), math.Max(hi, f)
			}
		}
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	if math.IsInf(lo, 1) {
		text.Draw(img, 8, h/2, "no samples inside the image", chartLabelPx, color.Black, nil)
		return img, nil
	}
	if hi-lo < 1e-9 {
		lo, hi = lo-0.5e-3, hi+0.5e-3
	}
	pad := (hi - lo) * 0.05
	lo, hi = lo-pad, hi+pad
	label := func(f float64) string { return strconv.FormatFloat(f, 'g', 4, 64) }
	axis := color.RGBA{R: 160, G: 160, B: 160, A: 255}
	ink := color.RGBA{R: 60, G: 60, B: 60, A: 255}
	left := max(text.Width(label(hi), chartLabelPx), text.Width(label(lo), chartLabelPx)) + 6
	bottom := text.Height(chartLabelPx) + 6
	top := 4
	plot := image.Rect(left, top, w-6, h-bottom)
	if plot.Dx() < 16 || plot.Dy() < 16 {
		plot = image.Rect(0, 0, w, h)
	}
	for x := plot.Min.X; x < plot.Max.X; x++ {
		img.Set(x, plot.Max.Y, axis)
	}
	for y := plot.Min.Y; y <= plot.Max.Y; y++ {
		img.Set(plot.Min.X-1, y, axis)
	}
	text.Draw(img, 2, plot.Min.Y, label(hi), chartLabelPx, ink, nil)
	text.Draw(img, 2, plot.Max.Y-text.Height(chartLabelPx), label(lo), chartLabelPx, ink, nil)
	tl, tr := label(t0), label(t1)+" doc px"
	text.Draw(img, plot.Min.X, plot.Max.Y+3, tl, chartLabelPx, ink, nil)
	text.Draw(img, plot.Max.X-text.Width(tr, chartLabelPx), plot.Max.Y+3, tr, chartLabelPx, ink, nil)
	span := t1 - t0
	if span <= 0 {
		span = 1
	}
	px := func(t float64) int { return plot.Min.X + int(math.Round((t-t0)/span*float64(plot.Dx()-1))) }
	py := func(v float64) int { return plot.Max.Y - 1 - int(math.Round((v-lo)/(hi-lo)*float64(plot.Dy()-2))) }
	for _, s := range lineSeries {
		vals, ok := m[s.key].([]any)
		if !ok {
			continue
		}
		prevOK := false
		var x0, y0 int
		for i, v := range vals {
			f, ok := num(v)
			tv, okT := num(ts[min(i, len(ts)-1)])
			if !ok || !okT {
				prevOK = false
				continue
			}
			x, y := px(tv), py(f)
			if prevOK {
				line(img, x0, y0, x, y, s.c)
			} else {
				img.Set(x, y, s.c)
			}
			x0, y0, prevOK = x, y, true
		}
	}
	return img, nil
}

// line draws a 1 px line from (x0, y0) to (x1, y1) (Bresenham).
func line(img draw.Image, x0, y0, x1, y1 int, c color.Color) {
	dx, dy := iabs(x1-x0), -iabs(y1-y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	e := dx + dy
	for {
		img.Set(x0, y0, c)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * e
		if e2 >= dy {
			e += dy
			x0 += sx
		}
		if e2 <= dx {
			e += dx
			y0 += sy
		}
	}
}

func iabs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
