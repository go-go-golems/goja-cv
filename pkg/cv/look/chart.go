package look

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// chartSize is the size of a histogram chart for a look of long side maxLong.
func chartSize(maxLong int) (int, int) {
	w := min(maxLong, 512)
	return w, max(w*2/5, MinSide)
}

// renderChart draws data["counts"] (a histogram) as a bar chart on white.
func renderChart(data any, w, h int) (draw.Image, error) {
	m, _ := data.(map[string]any)
	counts, ok := m["counts"].([]any)
	if !ok || len(counts) == 0 {
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
