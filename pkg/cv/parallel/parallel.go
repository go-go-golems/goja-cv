// Package parallel provides band-parallel loops for kernels.
package parallel

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"

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
		g.Go(func() (err error) { //nolint:nonamedreturns // recover needs named results
			defer Recover(&err)
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

// PanicError is a panic recovered on a goroutine goja-cv started (a kernel
// band, a decoder strip): the goroutine cannot re-panic on its caller's
// stack, so the panic travels back as this error, which the graph reports as
// E_INTERNAL.
type PanicError struct {
	Value any
	Stack string
}

func (e *PanicError) Error() string { return fmt.Sprintf("panic: %v", e.Value) }

// Recover turns a panic into a *PanicError in *err. Use it in a deferred call
// at the top of every goroutine: defer parallel.Recover(&err).
func Recover(err *error) {
	if p := recover(); p != nil {
		*err = &PanicError{Value: p, Stack: string(debug.Stack())}
	}
}
