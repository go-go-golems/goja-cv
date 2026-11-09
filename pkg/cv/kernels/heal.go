package kernels

import (
	"context"
	"math"

	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
)

// HealOptions configures Heal. Lengths are in buffer px.
type HealOptions struct {
	Margin float64 // grow each target by this before filling
	Method string  // "patch" | "diffuse"
}

// window is a rectangle of the image: [x0, x1) × [y0, y1).
type window struct{ x0, y0, x1, y1 int }

func (w window) W() int { return w.x1 - w.x0 }
func (w window) H() int { return w.y1 - w.y0 }

// Heal replaces every target pixel (target[i] true, grown by Margin) with
// content from its surroundings, one connected target at a time.
// "diffuse" solves Laplace's equation inside the target with the surrounding
// pixels as boundary values (smooth, no grain). "patch" searches nearby for
// the offset whose ring of pixels best matches the target's ring (SSD over
// all channels, sources must not touch any target), copies that patch and
// corrects the seam by adding the harmonic interpolation of the boundary
// difference (membrane cloning), so texture and grain are kept. Linear-light
// op; the input is not modified.
func Heal(ctx context.Context, img *buffer.Buffer, target []bool, o HealOptions) (*buffer.Buffer, error) {
	W, H := img.W, img.H
	hole := target
	if o.Margin > 0 {
		d, err := DistanceToSet(ctx, target, W, H)
		if err != nil {
			return nil, err
		}
		hole = make([]bool, len(target))
		for i, v := range d {
			hole[i] = float64(v) <= o.Margin
		}
	}
	comps, err := Components(ctx, hole, W, H)
	if err != nil {
		return nil, err
	}
	out := img.Clone()
	for _, pix := range comps {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		minX, minY, maxX, maxY := W, H, -1, -1
		for _, p := range pix {
			x, y := int(p)%W, int(p)/W
			minX, maxX, minY, maxY = min(minX, x), max(maxX, x), min(minY, y), max(maxY, y)
		}
		win := window{max(0, minX-2), max(0, minY-2), min(W, maxX+3), min(H, maxY+3)}
		local := make([]bool, win.W()*win.H())
		for _, p := range pix {
			x, y := int(p)%W, int(p)/W
			local[(y-win.y0)*win.W()+x-win.x0] = true
		}
		if o.Method == "patch" {
			if dx, dy, ok := findPatch(img, hole, local, win); ok {
				clonePatch(img, out, local, win, dx, dy)
				continue
			}
		}
		for c := 0; c < img.C; c++ {
			fillHarmonic(out.Plane(c), W, local, win)
		}
	}
	return out, nil
}

// fillHarmonic replaces the unknown pixels of plane inside win (local mask)
// with the solution of Laplace's equation whose boundary values are the
// known pixels: a pull-push initialization, then SOR iterations.
func fillHarmonic(plane []float32, W int, unknown []bool, win window) {
	w, h := win.W(), win.H()
	vals := make([]float32, w*h)
	for y := 0; y < h; y++ {
		copy(vals[y*w:(y+1)*w], plane[(y+win.y0)*W+win.x0:(y+win.y0)*W+win.x1])
	}
	harmonic(vals, unknown, w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if unknown[y*w+x] {
				plane[(y+win.y0)*W+x+win.x0] = vals[y*w+x]
			}
		}
	}
}

// harmonic fills the unknown entries of vals (w×h) in place: pull-push, then
// successive over-relaxation of the 4-neighbour mean (neighbours outside the
// window are ignored: Neumann borders).
func harmonic(vals []float32, unknown []bool, w, h int) {
	pullPush(vals, unknown, w, h)
	idx := make([]int, 0, 64)
	for i, u := range unknown {
		if u {
			idx = append(idx, i)
		}
	}
	if len(idx) == 0 {
		return
	}
	iters := min(3000, 40+8*max(w, h))
	const omega = 1.85
	for it := 0; it < iters; it++ {
		var maxd float32
		for _, i := range idx {
			x, y := i%w, i/w
			var s float32
			n := 0
			if x > 0 {
				s += vals[i-1]
				n++
			}
			if x < w-1 {
				s += vals[i+1]
				n++
			}
			if y > 0 {
				s += vals[i-w]
				n++
			}
			if y < h-1 {
				s += vals[i+w]
				n++
			}
			if n == 0 {
				continue
			}
			d := s/float32(n) - vals[i]
			vals[i] += omega * d
			if d < 0 {
				d = -d
			}
			maxd = max(maxd, d)
		}
		if maxd < 1e-6 {
			break
		}
	}
}

// pullPush gives unknown entries a smooth first guess: known values are
// averaged down a 2×2 pyramid until every cell has data, then pushed back up,
// each level filling its unknown cells from the coarser one.
func pullPush(vals []float32, unknown []bool, w, h int) {
	type level struct {
		v, wt []float32
		w, h  int
	}
	l0 := level{v: make([]float32, w*h), wt: make([]float32, w*h), w: w, h: h}
	has := false
	for i := range vals {
		if !unknown[i] {
			l0.v[i], l0.wt[i] = vals[i], 1
			has = true
		}
	}
	if !has {
		return
	}
	levels := []level{l0}
	for {
		cur := levels[len(levels)-1]
		full := true
		for _, x := range cur.wt {
			if x == 0 {
				full = false
				break
			}
		}
		if full || (cur.w == 1 && cur.h == 1) {
			break
		}
		nw, nh := (cur.w+1)/2, (cur.h+1)/2
		nx := level{v: make([]float32, nw*nh), wt: make([]float32, nw*nh), w: nw, h: nh}
		for y := 0; y < cur.h; y++ {
			for x := 0; x < cur.w; x++ {
				i, j := y*cur.w+x, (y/2)*nw+x/2
				nx.v[j] += cur.v[i] * cur.wt[i]
				nx.wt[j] += cur.wt[i]
			}
		}
		for j := range nx.v {
			if nx.wt[j] > 0 {
				nx.v[j] /= nx.wt[j]
				nx.wt[j] = min(1, nx.wt[j])
			}
		}
		levels = append(levels, nx)
	}
	for k := len(levels) - 2; k >= 0; k-- {
		cur, up := levels[k], levels[k+1]
		for y := 0; y < cur.h; y++ {
			for x := 0; x < cur.w; x++ {
				i := y*cur.w + x
				if cur.wt[i] >= 1 {
					continue
				}
				u := up.v[(y/2)*up.w+x/2]
				cur.v[i] = cur.v[i]*cur.wt[i] + u*(1-cur.wt[i])
				cur.wt[i] = 1
			}
		}
	}
	for i := range vals {
		if unknown[i] {
			vals[i] = l0.v[i]
		}
	}
}

// ring returns the window-local indices of pixels within 2 px (Chebyshev)
// of the target but outside it.
func ring(local []bool, w, h int) []int {
	var out []int
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if local[y*w+x] {
				continue
			}
			near := false
			for dy := -2; dy <= 2 && !near; dy++ {
				for dx := -2; dx <= 2; dx++ {
					xx, yy := x+dx, y+dy
					if xx >= 0 && yy >= 0 && xx < w && yy < h && local[yy*w+xx] {
						near = true
						break
					}
				}
			}
			if near {
				out = append(out, y*w+x)
			}
		}
	}
	return out
}

// findPatch searches offsets (dx, dy) around the target for the source whose
// ring best matches the target's ring. Every source pixel (target and ring
// shifted) must be inside the image and outside every target.
func findPatch(img *buffer.Buffer, hole, local []bool, win window) (int, int, bool) {
	W, H := img.W, img.H
	w, h := win.W(), win.H()
	rg := ring(local, w, h)
	if len(rg) < 4 {
		return 0, 0, false
	}
	var all []int // target + ring, window-local
	for i, u := range local {
		if u {
			all = append(all, i)
		}
	}
	all = append(all, rg...)
	size := max(w, h)
	S := max(10, 2*size)
	step := 1
	if size > 12 {
		step = 2
	}
	best, bdx, bdy := math.Inf(1), 0, 0
	for dy := -S; dy <= S; dy += step {
		for dx := -S; dx <= S; dx += step {
			if max(abs(dx), abs(dy)) < size/2+2 {
				continue // overlapping the target itself
			}
			ok := true
			for _, i := range all {
				x, y := i%w+win.x0+dx, i/w+win.y0+dy
				if x < 0 || y < 0 || x >= W || y >= H || hole[y*W+x] {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			var cost float64
			for c := 0; c < img.C && cost < best; c++ {
				pl := img.Plane(c)
				for _, i := range rg {
					x, y := i%w+win.x0, i/w+win.y0
					d := float64(pl[(y+dy)*W+x+dx] - pl[y*W+x])
					cost += d * d
				}
			}
			// prefer near sources when costs tie
			cost *= 1 + 0.002*math.Hypot(float64(dx), float64(dy))
			if cost < best {
				best, bdx, bdy = cost, dx, dy
			}
		}
	}
	return bdx, bdy, !math.IsInf(best, 1)
}

// clonePatch copies the source at (dx, dy) into the target and corrects the
// seam: the difference original − source on the ring is interpolated
// harmonically over the target and added.
func clonePatch(img, out *buffer.Buffer, local []bool, win window, dx, dy int) {
	W := img.W
	w, h := win.W(), win.H()
	for c := 0; c < img.C; c++ {
		src, dst := img.Plane(c), out.Plane(c)
		diff := make([]float32, w*h)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				gx, gy := x+win.x0, y+win.y0
				sx, sy := gx+dx, gy+dy
				var s float32
				if sx >= 0 && sy >= 0 && sx < W && sy < img.H {
					s = src[sy*W+sx]
				}
				diff[y*w+x] = src[gy*W+gx] - s
			}
		}
		harmonic(diff, local, w, h)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if local[y*w+x] {
					gx, gy := x+win.x0, y+win.y0
					dst[gy*W+gx] = src[(gy+dy)*W+gx+dx] + diff[y*w+x]
				}
			}
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
