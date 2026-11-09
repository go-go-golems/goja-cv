// Package parallel provides band-parallel loops for kernels.
package parallel

import (
	"context"
	"runtime"

	"golang.org/x/sync/errgroup"
)

// minParallelRows is the row count below which Rows runs inline.
const minParallelRows = 16

// Rows splits [0, n) into bands and calls fn(y0, y1) for each band on a
// bounded number of goroutines (GOMAXPROCS). The context is checked before
// each band starts; a cancelled context stops scheduling new bands and Rows
// returns ctx.Err(). fn must only write to rows in [y0, y1).
func Rows(ctx context.Context, n int, fn func(y0, y1 int)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if n <= 0 {
		return nil
	}
	workers := runtime.GOMAXPROCS(0)
	if n < minParallelRows || workers == 1 {
		fn(0, n)
		return ctx.Err()
	}
	bands := workers * 4
	if bands > n {
		bands = n
	}
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(workers)
	for i := 0; i < bands; i++ {
		if gctx.Err() != nil {
			break
		}
		y0, y1 := i*n/bands, (i+1)*n/bands
		g.Go(func() error {
			if err := gctx.Err(); err != nil {
				return err
			}
			fn(y0, y1)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	return ctx.Err()
}
