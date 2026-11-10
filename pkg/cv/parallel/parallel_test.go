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

func TestEachCoversEveryIndexOnce(t *testing.T) {
	for _, n := range []int{0, 1, 3, 100} {
		hits := make([]int32, n)
		if err := Each(context.Background(), n, func(i int) error {
			atomic.AddInt32(&hits[i], 1)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		for i, h := range hits {
			if h != 1 {
				t.Fatalf("n=%d index %d hit %d times", n, i, h)
			}
		}
	}
	if err := Each(context.Background(), 4, func(i int) error {
		if i == 2 {
			panic("boom")
		}
		return nil
	}); err == nil {
		t.Fatal("a panic in a task did not become an error")
	}
}
