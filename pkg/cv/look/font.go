package look

import (
	"image"
	"image/color"
	"image/draw"
	"strconv"
)

// glyphs is a 5×7 bitmap font for the digits; each row uses the low 5 bits,
// most significant bit on the left.
var glyphs = [10][7]uint8{
	{0x0E, 0x11, 0x13, 0x15, 0x19, 0x11, 0x0E}, // 0
	{0x04, 0x0C, 0x04, 0x04, 0x04, 0x04, 0x0E}, // 1
	{0x0E, 0x11, 0x01, 0x02, 0x04, 0x08, 0x1F}, // 2
	{0x1F, 0x02, 0x04, 0x02, 0x01, 0x11, 0x0E}, // 3
	{0x02, 0x06, 0x0A, 0x12, 0x1F, 0x02, 0x02}, // 4
	{0x1F, 0x10, 0x1E, 0x01, 0x01, 0x11, 0x0E}, // 5
	{0x06, 0x08, 0x10, 0x1E, 0x11, 0x11, 0x0E}, // 6
	{0x1F, 0x01, 0x02, 0x04, 0x08, 0x08, 0x08}, // 7
	{0x0E, 0x11, 0x11, 0x0E, 0x11, 0x11, 0x0E}, // 8
	{0x0E, 0x11, 0x11, 0x0F, 0x01, 0x02, 0x0C}, // 9
}

const (
	glyphW = 5
	glyphH = 7
)

// GlyphScale is the size of one font pixel for an image of the given size:
// about 1/160 of the long side, between 2 and 8 px.
func GlyphScale(w, h int) int {
	return min(max(max(w, h)/160, 2), 8)
}

// NumberBox is the rectangle BurnNumber paints for n on a w×h image
// (clipped to the image).
func NumberBox(n, w, h int) image.Rectangle {
	if n <= 0 {
		return image.Rectangle{}
	}
	s := GlyphScale(w, h)
	digits := len(strconv.Itoa(n))
	bw := 2*s + digits*glyphW*s + (digits-1)*s
	bh := 2*s + glyphH*s
	return image.Rect(0, 0, bw, bh).Intersect(image.Rect(0, 0, w, h))
}

// BurnNumber paints n (> 0) in white on a black box in the top-left corner
// of m. Pixels outside NumberBox are unchanged.
func BurnNumber(m draw.Image, n int) {
	if n <= 0 {
		return
	}
	b := m.Bounds()
	w, h := b.Dx(), b.Dy()
	box := NumberBox(n, w, h).Add(b.Min)
	draw.Draw(m, box, image.NewUniform(color.Black), image.Point{}, draw.Src)
	s := GlyphScale(w, h)
	x0 := b.Min.X + s
	for _, ch := range strconv.Itoa(n) {
		g := glyphs[ch-'0']
		for row := 0; row < glyphH; row++ {
			for col := 0; col < glyphW; col++ {
				if g[row]&(1<<(glyphW-1-col)) == 0 {
					continue
				}
				px := image.Rect(x0+col*s, b.Min.Y+s+row*s, x0+(col+1)*s, b.Min.Y+s+(row+1)*s).Intersect(box)
				draw.Draw(m, px, image.NewUniform(color.White), image.Point{}, draw.Src)
			}
		}
		x0 += (glyphW + 1) * s
	}
}

// GlyphPixel reports whether font pixel (col, row) of digit d is set; for
// tests that read a burned-in number back.
func GlyphPixel(d, col, row int) bool {
	return glyphs[d][row]&(1<<(glyphW-1-col)) != 0
}
