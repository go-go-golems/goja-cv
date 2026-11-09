package parallel

import (
	"context"
	"sync/atomic"
	"testing"
)

func TestRowsCoversEveryRowOnce(t *testing.T) {
	for _, n := range []int{0, 1, 7, 16, 100, 1037} {
		hits := make([]int32, n)
		if err := Rows(context.Background(), n, func(y0, y1 int) {
			for y := y0; y < y1; y++ {
				atomic.AddInt32(&hits[y], 1)
			}
		}); err != nil {
			t.Fatal(err)
		}
		for y, h := range hits {
			if h != 1 {
				t.Fatalf("n=%d row %d hit %d times", n, y, h)
			}
		}
	}
}

func TestRowsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	err := Rows(ctx, 1000, func(int, int) { called = true })
	if err == nil || called {
		t.Fatalf("err=%v called=%v, want cancellation before any band", err, called)
	}
}
