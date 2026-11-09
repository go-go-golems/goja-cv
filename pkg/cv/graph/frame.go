package graph

import (
	"fmt"
	"math"
)

// Frame is the rectangle an image covers in doc px: an integer origin (file
// pixel positions) and an integer size. Coordinates are absolute: a window
// read from a large scan keeps the scan's coordinates.
type Frame struct {
	X, Y int // origin in doc px
	W, H int // size in doc px
}

// Empty reports whether the frame has no area (pure data nodes).
func (f Frame) Empty() bool { return f.W <= 0 || f.H <= 0 }

// String renders the frame as a box: "[1536, 1536, 1024, 1024]".
func (f Frame) String() string { return fmt.Sprintf("[%d, %d, %d, %d]", f.X, f.Y, f.W, f.H) }

// Data is the frame as plain data [x, y, w, h].
func (f Frame) Data() []any { return []any{float64(f.X), float64(f.Y), float64(f.W), float64(f.H)} }

// Size is "W×H".
func (f Frame) Size() string { return fmt.Sprintf("%d×%d", f.W, f.H) }

// ContainsPoint reports whether the doc-px point (x, y) lies inside f.
func (f Frame) ContainsPoint(x, y float64) bool {
	return x >= float64(f.X) && y >= float64(f.Y) && x < float64(f.X+f.W) && y < float64(f.Y+f.H)
}

// OverlapsBox reports whether the doc-px box [x, y, w, h] intersects f. A
// box without area overlaps when its corner lies in the closed frame.
func (f Frame) OverlapsBox(x, y, w, h float64) bool {
	x0, y0, x1, y1 := float64(f.X), float64(f.Y), float64(f.X+f.W), float64(f.Y+f.H)
	if w <= 0 || h <= 0 {
		return x >= x0 && y >= y0 && x <= x1 && y <= y1
	}
	return x < x1 && y < y1 && x+w > x0 && y+h > y0
}

// NearestBox returns the box of size (w, h) inside f closest to (x, y)
// (clamped; a box larger than the frame starts at its origin).
func (f Frame) NearestBox(x, y, w, h float64) [4]float64 {
	cx := math.Max(float64(f.X), math.Min(x, float64(f.X+f.W)-w))
	cy := math.Max(float64(f.Y), math.Min(y, float64(f.Y+f.H)-h))
	return [4]float64{cx, cy, w, h}
}

// NearestPoint returns the point inside f closest to (x, y).
func (f Frame) NearestPoint(x, y float64) [2]float64 {
	return [2]float64{
		math.Max(float64(f.X), math.Min(x, float64(f.X+f.W-1))),
		math.Max(float64(f.Y), math.Min(y, float64(f.Y+f.H-1))),
	}
}

// frameError is E_FRAME: two images of one op cover different frames.
func frameError(op string, a, b Frame) *Error {
	return Errorf(EFrame, "%s: the inputs cover different frames (%s and %s, [x, y, w, h] in doc px).", op, a, b).
		WithHint("Images, masks and regions combined by one op must come from the same frame: open the same window, or crop both alike.")
}

// SameFrame returns E_FRAME when frames a and b differ.
func SameFrame(op string, a, b Frame) error {
	if a != b {
		return frameError(op, a, b)
	}
	return nil
}
