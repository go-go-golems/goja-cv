// Package text is goja-cv's one bitmap font: the X11 misc-fixed 5×7 font
// (printable ASCII and ×, plus a hand-drawn →), used for look numbers, ruler
// labels, region ids, annotation text and sheet labels.
//
// A glyph is a cell of 5 × 7 font pixels whose last column (and, for most
// glyphs, last row) is blank, so glyphs set side by side are spaced. px is the
// size of one font pixel in image px: text at px 2 is 10 × 14 image px per
// character.
package text

import (
	"image"
	"image/color"
	"image/draw"
	"strconv"
)

const (
	glyphW = 5 // cell width in font px (the advance)
	glyphH = 7 // cell height in font px
)

// extra are glyphs the 5×7 font does not have.
var extra = map[rune][glyphH]uint8{
	'→': {0x00, 0x04, 0x02, 0x1E, 0x02, 0x04, 0x00},
}

// glyph returns the bitmap of r; runes without a glyph are drawn as '?'.
func glyph(r rune) [glyphH]uint8 {
	if g, ok := misc5x7[r]; ok {
		return g
	}
	if g, ok := extra[r]; ok {
		return g
	}
	return misc5x7['?']
}

// Has reports whether r has a glyph of its own.
func Has(r rune) bool {
	_, a := misc5x7[r]
	_, b := extra[r]
	return a || b
}

// Pixel reports whether font pixel (col, row) of r's glyph is set (for tests
// that read text back from an image).
func Pixel(r rune, col, row int) bool {
	if col < 0 || col >= glyphW || row < 0 || row >= glyphH {
		return false
	}
	return glyph(r)[row]&(1<<(glyphW-1-col)) != 0
}

// Width is the width of s in image px at font size px.
func Width(s string, px int) int { return len([]rune(s)) * glyphW * px }

// Height is the height of one line of text in image px at font size px.
func Height(px int) int { return glyphH * px }

// Walk calls fn for every set image pixel of s drawn with its top-left at
// (x, y) at font size px. Drawing into float buffers (overlay, annotate)
// goes through Walk.
func Walk(x, y int, s string, px int, fn func(x, y int)) {
	cx := x
	for _, r := range s {
		g := glyph(r)
		for row := 0; row < glyphH; row++ {
			for col := 0; col < glyphW; col++ {
				if g[row]&(1<<(glyphW-1-col)) == 0 {
					continue
				}
				for dy := 0; dy < px; dy++ {
					for dx := 0; dx < px; dx++ {
						fn(cx+col*px+dx, y+row*px+dy)
					}
				}
			}
		}
		cx += glyphW * px
	}
}

// Draw draws s in fg with its top-left at (x, y), at font size px, over a
// one-pixel shadow (offset right and down) when shadow is not nil.
func Draw(dst draw.Image, x, y int, s string, px int, fg, shadow color.Color) {
	b := dst.Bounds()
	set := func(c color.Color) func(int, int) {
		return func(x, y int) {
			if image.Pt(x, y).In(b) {
				dst.Set(x, y, c)
			}
		}
	}
	if shadow != nil {
		Walk(x+1, y+1, s, px, set(shadow))
	}
	Walk(x, y, s, px, set(fg))
}

// NumberScale is the font size of the look number on a w × h image: about
// 1/160 of the long side, between 2 and 8 px.
func NumberScale(w, h int) int {
	return min(max(max(w, h)/160, 2), 8)
}

// NumberBox is the rectangle BurnNumber paints for n on a w × h image
// (clipped to the image): one font px of margin around the digits.
func NumberBox(n, w, h int) image.Rectangle {
	if n <= 0 {
		return image.Rectangle{}
	}
	s := NumberScale(w, h)
	digits := len(strconv.Itoa(n))
	return image.Rect(0, 0, 2*s+digits*glyphW*s, 2*s+glyphH*s).Intersect(image.Rect(0, 0, w, h))
}

// BurnNumber paints n (> 0) in white on a black box in the top-left corner
// of m. Pixels outside NumberBox are unchanged.
func BurnNumber(m draw.Image, n int) {
	if n <= 0 {
		return
	}
	b := m.Bounds()
	box := NumberBox(n, b.Dx(), b.Dy()).Add(b.Min)
	draw.Draw(m, box, image.NewUniform(color.Black), image.Point{}, draw.Src)
	s := NumberScale(b.Dx(), b.Dy())
	Walk(b.Min.X+s, b.Min.Y+s, strconv.Itoa(n), s, func(x, y int) {
		if image.Pt(x, y).In(box) {
			m.Set(x, y, color.White)
		}
	})
}
