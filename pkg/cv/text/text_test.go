package text

import (
	"image"
	"image/color"
	"testing"
)

// required is the character set the guide asks for (GCV-002 §7.12).
const required = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz[](),.:-+×→%?/_ "

func TestRequiredGlyphsExist(t *testing.T) {
	for _, r := range required {
		if !Has(r) {
			t.Errorf("no glyph for %q", r)
		}
	}
	if Has('€') {
		t.Fatal("€ should not have a glyph")
	}
}

func TestEveryGlyphRendersInsideItsBox(t *testing.T) {
	runes := []rune(required)
	for r := range misc5x7 {
		runes = append(runes, r)
	}
	for _, r := range runes {
		for _, px := range []int{1, 2, 3} {
			lit := 0
			Walk(10, 20, string(r), px, func(x, y int) {
				lit++
				if x < 10 || x >= 10+Width(string(r), px) || y < 20 || y >= 20+Height(px) {
					t.Fatalf("%q at px %d: pixel (%d, %d) outside its %d×%d box", r, px, x, y, Width(string(r), px), Height(px))
				}
			})
			if r != ' ' && lit == 0 {
				t.Errorf("%q at px %d draws nothing", r, px)
			}
			if lit%(px*px) != 0 {
				t.Errorf("%q at px %d: %d lit pixels is not a whole number of font pixels", r, px, lit)
			}
		}
	}
}

func TestWidthIsTheAdvance(t *testing.T) {
	for _, tc := range []struct {
		s    string
		px   int
		want int
	}{{"", 2, 0}, {"7", 1, 5}, {"300", 2, 30}, {"×1.28", 2, 50}, {"7 (1337, 131)", 2, 130}} {
		if got := Width(tc.s, tc.px); got != tc.want {
			t.Errorf("Width(%q, %d) = %d, want %d", tc.s, tc.px, got, tc.want)
		}
	}
	// the second glyph starts one advance to the right
	var xs []int
	Walk(0, 0, "11", 1, func(x, y int) {
		if y == 0 {
			xs = append(xs, x)
		}
	})
	if len(xs) != 2 || xs[1]-xs[0] != glyphW {
		t.Fatalf("top-row pixels of \"11\": %v, want two %d apart", xs, glyphW)
	}
}

// readBack decodes the font pixels of one glyph drawn at (x, y) with size px
// and returns the rune whose glyph matches, or 0.
func readBack(m *image.Gray, x, y, px int, candidates string) rune {
	for _, r := range candidates {
		ok := true
		for row := 0; row < glyphH && ok; row++ {
			for col := 0; col < glyphW && ok; col++ {
				on := m.GrayAt(x+col*px+px/2, y+row*px+px/2).Y > 128
				ok = on == Pixel(r, col, row)
			}
		}
		if ok {
			return r
		}
	}
	return 0
}

func TestDrawReadsBack(t *testing.T) {
	m := image.NewGray(image.Rect(0, 0, 200, 40))
	Draw(m, 3, 4, "x 300-364", 2, color.White, color.Black)
	got := ""
	for i := range []rune("x 300-364") {
		got += string(readBack(m, 3+i*glyphW*2, 4, 2, "0123456789-x "))
	}
	if got != "x 300-364" {
		t.Fatalf("read back %q", got)
	}
}

func TestBurnNumberReadsBack(t *testing.T) {
	m := image.NewGray(image.Rect(0, 0, 64, 48))
	for i := range m.Pix {
		m.Pix[i] = 100
	}
	BurnNumber(m, 27)
	s := NumberScale(64, 48)
	box := NumberBox(27, 64, 48)
	if got := string(readBack(m, s, s, s, "0123456789")) + string(readBack(m, s+glyphW*s, s, s, "0123456789")); got != "27" {
		t.Fatalf("burned number reads %q", got)
	}
	for _, p := range []image.Point{{box.Max.X, 0}, {0, box.Max.Y}, {63, 47}} {
		if v := m.GrayAt(p.X, p.Y).Y; v != 100 {
			t.Fatalf("pixel %v outside the box = %d, want unchanged", p, v)
		}
	}
	if v := m.GrayAt(0, 0).Y; v != 0 {
		t.Fatalf("box corner %d, want black", v)
	}
	BurnNumber(m, 0) // no-op
}
