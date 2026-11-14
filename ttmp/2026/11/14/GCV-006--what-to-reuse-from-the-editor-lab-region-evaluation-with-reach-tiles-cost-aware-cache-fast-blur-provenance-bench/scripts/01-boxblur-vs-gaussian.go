//go:build ignore

// 01-boxblur-vs-gaussian: time goja-cv's separable Gaussian (kernels.Gaussian,
// 6σ+1 taps per pass) against the Editor lab's three box blurs (boxSizes,
// running sums: cost independent of σ) on a 6000 × 4000 plane, and report the
// approximation error. Run from the goja-cv module root:
//
//	go run ./ttmp/2026/11/14/GCV-006--*/scripts/01-boxblur-vs-gaussian.go
package main

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"sync"
	"time"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cv/kernels"
)

// boxSizes is the Editor lab's engine.js boxSizes (Kovesi 2010): n box widths
// whose successive application has the variance of a Gaussian of σ.
func boxSizes(sigma float64, n int) []int {
	wIdeal := math.Sqrt(12*sigma*sigma/float64(n) + 1)
	wl := int(math.Floor(wIdeal))
	if wl%2 == 0 {
		wl--
	}
	wu := wl + 2
	m := int(math.Round((12*sigma*sigma - float64(n*wl*wl) - float64(4*n*wl) - float64(3*n)) / float64(-4*wl-4)))
	out := make([]int, n)
	for i := range out {
		if i < m {
			out[i] = wl
		} else {
			out[i] = wu
		}
	}
	return out
}

func reflect101(i, n int) int {
	if n == 1 {
		return 0
	}
	p := 2*n - 2
	i %= p
	if i < 0 {
		i += p
	}
	if i >= n {
		i = p - i
	}
	return i
}

// boxLine is one running-sum box pass of radius r over src (stride 1, length n)
// with mirrored borders, as goja-cv's Gaussian mirrors.
func boxLine(src, dst []float32, n, r int) {
	norm := 1 / float32(2*r+1)
	var acc float32
	for j := -r; j <= r; j++ {
		acc += src[reflect101(j, n)]
	}
	for x := 0; x < n; x++ {
		dst[x] = acc * norm
		acc += src[reflect101(x+r+1, n)] - src[reflect101(x-r, n)]
	}
}

func boxBlur3(p []float32, w, h int, sigma float64) []float32 {
	radii := []int{}
	for _, s := range boxSizes(sigma, 3) {
		if r := (s - 1) / 2; r > 0 {
			radii = append(radii, r)
		}
	}
	a := append([]float32(nil), p...)
	b := make([]float32, len(p))
	bands := func(n int, f func(lo, hi int)) { // split [0, n) over the CPUs
		k := runtime.GOMAXPROCS(0)
		var wg sync.WaitGroup
		for i := 0; i < k; i++ {
			lo, hi := n*i/k, n*(i+1)/k
			wg.Add(1)
			go func() { defer wg.Done(); f(lo, hi) }()
		}
		wg.Wait()
	}
	for _, r := range radii { // horizontal passes, rows in parallel
		src, dst := a, b
		bands(h, func(y0, y1 int) {
			for y := y0; y < y1; y++ {
				boxLine(src[y*w:(y+1)*w], dst[y*w:(y+1)*w], w, r)
			}
		})
		a, b = b, a
	}
	for _, r := range radii { // vertical passes: a running sum per column, row-major, column bands in parallel
		src, dst := a, b
		norm := 1 / float32(2*r+1)
		bands(w, func(x0, x1 int) {
			acc := make([]float32, x1-x0)
			for j := -r; j <= r; j++ {
				row := src[reflect101(j, h)*w:]
				for x := x0; x < x1; x++ {
					acc[x-x0] += row[x]
				}
			}
			for y := 0; y < h; y++ {
				out := dst[y*w:]
				add, sub := src[reflect101(y+r+1, h)*w:], src[reflect101(y-r, h)*w:]
				for x := x0; x < x1; x++ {
					out[x] = acc[x-x0] * norm
					acc[x-x0] += add[x] - sub[x]
				}
			}
		})
		a, b = b, a
	}
	return a
}

func main() {
	const w, h = 6000, 4000
	rng := rand.New(rand.NewSource(1))
	in := buffer.New(w, h, 1, 1)
	for i := range in.Pix {
		in.Pix[i] = float32(0.3 + 0.05*rng.NormFloat64())
	}
	for i := 0; i < 400; i++ { // 3 × 3 specks
		x, y := 20+rng.Intn(w-40), 20+rng.Intn(h-40)
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				in.Pix[(y+dy)*w+x+dx] += 0.4
			}
		}
	}
	fmt.Println("sigma | gaussian ms (parallel) | 3-box ms (parallel) | max |diff| | rms diff | max |diff| / max|g - mean|")
	for _, sigma := range []float64{1, 2, 4, 8, 16, 32} {
		t0 := time.Now()
		g, err := kernels.Gaussian(context.Background(), in, sigma)
		if err != nil {
			panic(err)
		}
		tg := time.Since(t0)
		t0 = time.Now()
		bx := boxBlur3(in.Pix, w, h, sigma)
		tb := time.Since(t0)
		var maxd, ss, mean, maxdev float64
		for _, v := range g.Pix {
			mean += float64(v)
		}
		mean /= float64(len(g.Pix))
		for i, v := range g.Pix {
			d := math.Abs(float64(v - bx[i]))
			maxd = math.Max(maxd, d)
			ss += d * d
			maxdev = math.Max(maxdev, math.Abs(float64(v)-mean))
		}
		fmt.Printf("%5g | %8.0f | %8.0f | %.2e | %.2e | %.3f\n", sigma, float64(tg.Milliseconds()), float64(tb.Milliseconds()), maxd, math.Sqrt(ss/float64(len(g.Pix))), maxd/maxdev)
	}
}
