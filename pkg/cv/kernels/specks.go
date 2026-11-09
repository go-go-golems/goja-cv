package kernels

import (
	"context"
	"math"
	"sort"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
)

// SpeckOptions configures DetectSpecks. Lengths and areas are in buffer px.
type SpeckOptions struct {
	MaxArea     float64 // largest speck in px²
	MinContrast float64 // smallest peak contrast (perceptual units)
	Polarity    string  // "dark" | "bright" | "both"
	NoiseK      float64 // detection threshold in noise σ (default 4)
}

// Speck is one detected speck: its pixels and measures.
type Speck struct {
	Pix        []int32
	Dark       bool
	Contrast   float64 // peak |residual| (perceptual)
	LocalNoise float64 // robust σ of the residual in a ring around the speck
	Sharpness  float64 // fill factor Σ|residual| / (peak · area): ~0.5–0.9 for hard-edged dust, ~0.3 for a soft Gaussian blob (a star)
	Score      float64 // [0, 1]: signal-to-noise × sharpness
	MeanL      float64 // mean linear luminance of the speck's pixels
}

// SpeckResult is the output of DetectSpecks.
type SpeckResult struct {
	Specks []Speck
	Noise  float64 // global robust residual σ
	Thresh float64 // high detection threshold used
}

// median of a copy of xs.
func medianOf(xs []float32) float64 {
	if len(xs) == 0 {
		return 0
	}
	c := append([]float32(nil), xs...)
	return float64(selectK(c, len(c)/2))
}

// MAD is the median absolute deviation from the median, scaled to σ for
// normal data (×1.4826).
func MAD(xs []float32) (med, sigma float64) {
	med = medianOf(xs)
	d := make([]float32, len(xs))
	for i, x := range xs {
		d[i] = float32(math.Abs(float64(x) - med))
	}
	return med, 1.4826 * medianOf(d)
}

// oddPad extends a w×h plane by p px on every side with point reflection
// about the edge samples (v(−k) = 2·v(0) − v(k)), which continues linear
// trends, so min/max filters near the border are not biased by the clipped
// window. Reflection indices are clamped for planes smaller than p.
func oddPad(src []float32, w, h, p int) ([]float32, int, int) {
	pw, ph := w+2*p, h+2*p
	out := make([]float32, pw*ph)
	refl := func(i, n int) (int, int, bool) { // index, mirror source, reflected?
		switch {
		case i < 0:
			return 0, min(-i, n-1), true
		case i >= n:
			return n - 1, max(2*(n-1)-i, 0), true
		}
		return i, i, false
	}
	for y := 0; y < h; y++ {
		row := src[y*w : (y+1)*w]
		for x := -p; x < w+p; x++ {
			e, m, r := refl(x, w)
			v := row[e]
			if r {
				v = 2*row[e] - row[m]
			}
			out[(y+p)*pw+x+p] = v
		}
	}
	for y := -p; y < h+p; y++ {
		e, m, r := refl(y, h)
		if !r {
			continue
		}
		for x := 0; x < pw; x++ {
			out[(y+p)*pw+x] = 2*out[(e+p)*pw+x] - out[(m+p)*pw+x]
		}
	}
	return out, pw, ph
}

// SpeckBackground estimates the speck-free background of L. The plane is
// denoised with a small Gaussian (σ = 1 px, so the min/max filters do not
// pick up noise extremes) and detrended by a wide Gaussian (σ = 2r): on a
// slope, a flat-structuring-element opening rebuilds each value from the
// uphill erosion, and a speck's erosion "shadow" would drag the background
// down around it. The detrended plane is padded by odd reflection, opened
// (removes bright specks) and closed (removes dark specks) with a (2r+1)²
// square and smoothed with a box mean of radius r/2; the trend is added back.
// This runs twice: the second trend is taken from the first background, so
// it no longer contains the specks' own dips.
func SpeckBackground(ctx context.Context, L []float32, w, h, r int) ([]float32, error) {
	g, err := Gaussian(ctx, &buffer.Buffer{W: w, H: h, C: 1, Pix: L, Scale: 1}, 1)
	if err != nil {
		return nil, err
	}
	src := g
	var bg []float32
	for pass := 0; pass < 2; pass++ {
		trend, err := Gaussian(ctx, src, float64(2*r))
		if err != nil {
			return nil, err
		}
		if bg, err = flatBackground(ctx, g.Pix, trend.Pix, w, h, r); err != nil {
			return nil, err
		}
		src = &buffer.Buffer{W: w, H: h, C: 1, Pix: bg, Scale: 1}
	}
	return bg, nil
}

// flatBackground is trend + box(close(open(oddPad(g − trend)))).
func flatBackground(ctx context.Context, g, trend []float32, w, h, r int) ([]float32, error) {
	d := make([]float32, w*h)
	for i := range d {
		d[i] = g[i] - trend[i]
	}
	p := 4*r + r/2 + 2
	padded, pw, ph := oddPad(d, w, h, p)
	o, err := Opening(ctx, padded, pw, ph, r)
	if err != nil {
		return nil, err
	}
	c, err := Closing(ctx, o, pw, ph, r)
	if err != nil {
		return nil, err
	}
	bm, err := BoxMean(ctx, c, pw, ph, max(1, r/2))
	if err != nil {
		return nil, err
	}
	out := make([]float32, w*h)
	for y := 0; y < h; y++ {
		copy(out[y*w:(y+1)*w], bm[(y+p)*pw+p:(y+p)*pw+p+w])
	}
	for i := range out {
		out[i] += trend[i]
	}
	return out, nil
}

// DetectSpecks finds small dark and/or bright specks in b: the residual of
// perceptual luminance against a morphological background (SpeckBackground)
// is thresholded with hysteresis at a noise-aware level (high: max(minContrast,
// k·σ), low: half of it but at least 2.5σ, σ from the MAD of the residual),
// labelled with union-find, and kept when no larger than MaxArea. Each speck
// gets a contrast (peak residual), a local noise (MAD in a 3 px ring), a
// sharpness (fill factor: soft round blobs such as stars score ~0.3, dust
// ≥ 0.5) and a score = smoothstep(k−1, k+4, contrast/localNoise) ·
// smoothstep(0.33, 0.45, sharpness). Specks are returned in raster order of
// their first pixel.
func DetectSpecks(ctx context.Context, b *buffer.Buffer, o SpeckOptions) (*SpeckResult, error) {
	if o.NoiseK <= 0 {
		o.NoiseK = 4
	}
	w, h := b.W, b.H
	L, err := PerceptualLuminance(ctx, b)
	if err != nil {
		return nil, err
	}
	lin := b.Luminance()
	diam := 2 * math.Sqrt(math.Max(o.MaxArea, 1)/math.Pi)
	r := max(2, int(math.Ceil(diam/2))+2)
	bg, err := SpeckBackground(ctx, L, w, h, r)
	if err != nil {
		return nil, err
	}
	res := make([]float32, w*h)
	for i := range res {
		res[i] = L[i] - bg[i]
	}
	med, sigma := MAD(res)
	for i := range res {
		res[i] -= float32(med)
	}
	sigma = math.Max(sigma, 1e-5)
	hi := math.Max(o.MinContrast, o.NoiseK*sigma)
	lo := math.Max(hi/2, 2.5*sigma)
	out := &SpeckResult{Noise: sigma, Thresh: hi}

	type cand struct {
		pix  []int32
		dark bool
	}
	var cands []cand
	for _, dark := range []bool{true, false} {
		if (dark && o.Polarity == "bright") || (!dark && o.Polarity == "dark") {
			continue
		}
		sign := float32(1)
		if dark {
			sign = -1
		}
		fg := make([]bool, w*h)
		for i, v := range res {
			fg[i] = sign*v >= float32(lo)
		}
		comps, err := Components(ctx, fg, w, h)
		if err != nil {
			return nil, err
		}
		for _, pix := range comps {
			if float64(len(pix)) > o.MaxArea {
				continue
			}
			peak := float32(0)
			for _, p := range pix {
				peak = max(peak, sign*res[p])
			}
			if float64(peak) < hi {
				continue
			}
			cands = append(cands, cand{pix, dark})
		}
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].pix[0] < cands[j].pix[0] })

	// all candidate pixels, excluded from the local-noise rings
	inSpeck := make([]bool, w*h)
	for _, c := range cands {
		for _, p := range c.pix {
			inSpeck[p] = true
		}
	}
	for _, c := range cands {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		sign := float32(1)
		if c.dark {
			sign = -1
		}
		var peak float32
		var sl float64
		minX, minY, maxX, maxY := w, h, 0, 0
		for _, p := range c.pix {
			peak = max(peak, sign*res[p])
			sl += float64(lin[p])
			x, y := int(p)%w, int(p)/w
			minX, maxX, minY, maxY = min(minX, x), max(maxX, x), min(minY, y), max(maxY, y)
		}
		var sum float64
		for _, p := range c.pix {
			sum += float64(sign * res[p])
		}
		ring := make([]float32, 0, 64)
		m := 3
		for y := max(0, minY-m); y <= min(h-1, maxY+m); y++ {
			for x := max(0, minX-m); x <= min(w-1, maxX+m); x++ {
				if !inSpeck[y*w+x] {
					ring = append(ring, res[y*w+x])
				}
			}
		}
		local := sigma
		if len(ring) >= 12 {
			_, s := MAD(ring)
			local = math.Max(s, sigma/2)
		}
		s := Speck{Pix: c.pix, Dark: c.dark, Contrast: float64(peak), LocalNoise: local,
			Sharpness: sum / float64(peak) / float64(len(c.pix)), MeanL: sl / float64(len(c.pix))}
		snr := s.Contrast / math.Max(local, 1e-5)
		s.Score = smoothstep(o.NoiseK-1, o.NoiseK+4, snr) * smoothstep(0.33, 0.45, s.Sharpness)
		out.Specks = append(out.Specks, s)
	}
	return out, nil
}
