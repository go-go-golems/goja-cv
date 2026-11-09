package kernels

import "context"

// unionFind is a disjoint-set forest over int32 labels with path halving.
type unionFind struct{ parent []int32 }

func (u *unionFind) make() int32 {
	u.parent = append(u.parent, int32(len(u.parent)))
	return int32(len(u.parent) - 1)
}

func (u *unionFind) find(x int32) int32 {
	for u.parent[x] != x {
		u.parent[x] = u.parent[u.parent[x]]
		x = u.parent[x]
	}
	return x
}

func (u *unionFind) union(a, b int32) {
	ra, rb := u.find(a), u.find(b)
	if ra == rb {
		return
	}
	if ra < rb {
		u.parent[rb] = ra
	} else {
		u.parent[ra] = rb
	}
}

// Components labels the 8-connected components of fg (w×h, row-major) with
// two-pass union-find and returns each component's pixel indices, in raster
// order of the components' first pixels.
func Components(ctx context.Context, fg []bool, w, h int) ([][]int32, error) {
	lab := make([]int32, w*h)
	for i := range lab {
		lab[i] = -1
	}
	uf := &unionFind{}
	for y := 0; y < h; y++ {
		if y%256 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		for x := 0; x < w; x++ {
			i := y*w + x
			if !fg[i] {
				continue
			}
			l := int32(-1)
			// already-visited neighbours: W, NW, N, NE
			for _, d := range [4][2]int{{-1, 0}, {-1, -1}, {0, -1}, {1, -1}} {
				nx, ny := x+d[0], y+d[1]
				if nx < 0 || ny < 0 || nx >= w {
					continue
				}
				nl := lab[ny*w+nx]
				if nl < 0 {
					continue
				}
				if l < 0 {
					l = nl
				} else if nl != l {
					uf.union(l, nl)
				}
			}
			if l < 0 {
				l = uf.make()
			}
			lab[i] = l
		}
	}
	index := map[int32]int{}
	var out [][]int32
	for i, l := range lab {
		if l < 0 {
			continue
		}
		r := uf.find(l)
		k, ok := index[r]
		if !ok {
			k = len(out)
			index[r] = k
			out = append(out, nil)
		}
		out[k] = append(out[k], int32(i))
	}
	return out, nil
}
