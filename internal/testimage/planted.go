package testimage

import (
	"context"
	"math"
	"math/rand"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
)

// Size of the planted-fault scan.
const (
	PlantedW = 256
	PlantedH = 192
	// PlantedTIFF is the file name WritePlanted uses.
	PlantedTIFF = "planted-256x192-16bit.tif"
	// PlantedNoise is the standard deviation of the added noise, in
	// perceptual (sRGB-encoded) units.
	PlantedNoise = 0.004
)

// Speck is one planted dust speck (doc px, perceptual contrast).
type Speck struct {
	X, Y     float64 // centre
	RX, RY   float64 // radii (RX ≠ RY for elongated specks)
	Contrast float64 // perceptual amplitude (positive)
	Dark     bool
}

// Star is the planted star: round, soft (Gaussian), bright.
type Star struct {
	X, Y, Sigma, Peak float64
}

// Planted describes the planted-fault image.
type Planted struct {
	Specks []Speck
	Star   Star
}

// PlantedBackground is the noise-free background in perceptual units at
// (x, y): a smooth diagonal gradient with a gentle bump, slightly warm.
func PlantedBackground(x, y float64) [3]float64 {
	v := 0.28 + 0.32*(x/PlantedW) + 0.12*(y/PlantedH) + 0.04*math.Sin(x/37)*math.Cos(y/29)
	return [3]float64{v * 1.04, v, v * 0.93}
}

// coverage is the 4×4-supersampled area of pixel (px, py) inside the
// ellipse s.
func (s Speck) coverage(px, py int) float64 {
	n := 0
	for j := 0; j < 4; j++ {
		for i := 0; i < 4; i++ {
			dx := (float64(px) + (float64(i)+0.5)/4 - s.X) / s.RX
			dy := (float64(py) + (float64(j)+0.5)/4 - s.Y) / s.RY
			if dx*dx+dy*dy <= 1 {
				n++
			}
		}
	}
	return float64(n) / 16
}

// PlantedScan builds the 256×192 planted-fault scan in linear light: a smooth
// gradient with Gaussian noise (σ = PlantedNoise perceptual), 40 hard-edged
// dust specks at known positions (30 dark, 10 bright; radii 1–3.5 px, some
// elongated; contrast 0.07–0.3) and one soft, round, bright star that a
// good detector must not report as dust. The result is deterministic.
func PlantedScan() (*buffer.Buffer, Planted) {
	rng := rand.New(rand.NewSource(20261108))
	p := Planted{Star: Star{X: 200.5, Y: 46.5, Sigma: 2.2, Peak: 0.45}}
	far := func(x, y, d float64) bool {
		if math.Hypot(x-p.Star.X, y-p.Star.Y) < d+12 {
			return false
		}
		for _, s := range p.Specks {
			if math.Hypot(x-s.X, y-s.Y) < d+math.Max(s.RX, s.RY)+6 {
				return false
			}
		}
		return true
	}
	for len(p.Specks) < 40 {
		r := 1 + rng.Float64()*2.5
		rx, ry := r, r
		if len(p.Specks)%7 == 3 { // elongated (fibre-like)
			rx, ry = r*2.2, math.Max(1, r*0.6)
		}
		x := 10 + rng.Float64()*(PlantedW-20)
		y := 10 + rng.Float64()*(PlantedH-20)
		if !far(x, y, math.Max(rx, ry)) {
			continue
		}
		p.Specks = append(p.Specks, Speck{X: x, Y: y, RX: rx, RY: ry,
			Contrast: 0.07 + rng.Float64()*0.23, Dark: len(p.Specks)%4 != 3})
	}
	b := buffer.New(PlantedW, PlantedH, 3, 1)
	noise := rand.New(rand.NewSource(7))
	for y := 0; y < PlantedH; y++ {
		for x := 0; x < PlantedW; x++ {
			bg := PlantedBackground(float64(x)+0.5, float64(y)+0.5)
			d := PlantedNoise * noise.NormFloat64()
			dx, dy := float64(x)+0.5-p.Star.X, float64(y)+0.5-p.Star.Y
			d += p.Star.Peak * math.Exp(-(dx*dx+dy*dy)/(2*p.Star.Sigma*p.Star.Sigma))
			for _, s := range p.Specks {
				if math.Abs(float64(x)-s.X) > s.RX+2 || math.Abs(float64(y)-s.Y) > s.RY+2 {
					continue
				}
				c := s.coverage(x, y) * s.Contrast
				if s.Dark {
					c = -c
				}
				d += c
			}
			i := y*PlantedW + x
			for ch := 0; ch < 3; ch++ {
				b.Pix[ch*PlantedW*PlantedH+i] = buffer.ToLinear(float32(math.Max(0, math.Min(1, bg[ch]+d))))
			}
		}
	}
	return b, p
}

// WritePlanted writes the planted scan as a 16-bit TIFF to path.
func WritePlanted(ctx context.Context, path string) (Planted, error) {
	b, p := PlantedScan()
	return p, buffer.WriteFile(ctx, path, b, "tiff16")
}
