package cvsession_test

import (
	"bytes"
	"image"
	"image/png"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/go-go-golems/go-go-goja/pkg/replsession"
	"github.com/go-go-golems/goja-cv/pkg/cv/docs"
	"github.com/go-go-golems/goja-cv/pkg/cv/ops"
	font "github.com/go-go-golems/goja-cv/pkg/cv/text"
	"github.com/go-go-golems/goja-cv/pkg/cvsession"
)

// Acceptance tests of GCV-002 (guide §13.6): each is the agent's
// JavaScript, run in a session against the planted scan.

func acceptanceApp(t *testing.T) (*cvsession.App, string) {
	t.Helper()
	img := planted(t)
	return newApp(t, cvsession.Options{Image: img, BaseDir: filepath.Dir(img)})
}

func mustOK(t *testing.T, resp *replsession.EvaluateResponse) string {
	t.Helper()
	out := text(resp)
	if resp.Cell.Execution.Status != "ok" {
		t.Fatalf("eval failed:\n%s", out)
	}
	return out
}

func decodePNG(t *testing.T, mime string, data []byte) image.Image {
	t.Helper()
	if mime != "image/png" {
		t.Fatalf("look is %s, want image/png", mime)
	}
	m, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// glyphAt reports which of runes is drawn at (x, y) with font size px in
// pixels for which ink(x, y) holds (0 if none matches).
func glyphAt(m image.Image, x, y, px int, runes string, ink func(r, g, b uint32) bool) rune {
	for _, r := range runes {
		ok := true
		for row := 0; row < font.Height(1) && ok; row++ {
			for col := 0; col < font.Width("0", 1) && ok; col++ {
				cr, cg, cb, _ := m.At(x+col*px+px/2, y+row*px+px/2).RGBA()
				ok = ink(cr>>8, cg>>8, cb>>8) == font.Pixel(r, col, row)
			}
		}
		if ok {
			return r
		}
	}
	return 0
}

func light(r, g, b uint32) bool { return r > 128 && g > 128 && b > 128 }

// numbersIn reads every number drawn at font size 2 in light ink inside the
// rectangle (by sliding a digit matcher over it).
func numbersIn(m image.Image, rect image.Rectangle) []int {
	var out []int
	adv := font.Width("0", 2)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if glyphAt(m, x, y, 2, "0123456789", light) == 0 {
				continue
			}
			s := ""
			for xx := x; ; xx += adv {
				r := glyphAt(m, xx, y, 2, "0123456789", light)
				if r == 0 {
					break
				}
				s += string(r)
			}
			n, _ := strconv.Atoi(s)
			out = append(out, n)
			x += len(s)*adv + 1
		}
	}
	sort.Ints(out)
	uniq := out[:0]
	for i, n := range out {
		if i == 0 || n != out[i-1] {
			uniq = append(uniq, n)
		}
	}
	return uniq
}

func ints(from, to, step int) []int {
	var out []int
	for v := from; v <= to; v += step {
		out = append(out, v)
	}
	return out
}

func fmtInts(xs []int) string {
	s := make([]string, len(xs))
	for i, x := range xs {
		s[i] = strconv.Itoa(x)
	}
	return "[" + strings.Join(s, " ") + "]"
}

// Issue 1 and 23a: §7.1's eval returns three 8× looks of a 64 px window in
// one eval; the speck look's caption names the view and the crop. The
// planted scan is opened with origin [200, 80], so the window at (300, 100)
// is inside it.
func TestAcceptanceZoomedWindowsAndCaption(t *testing.T) {
	app, sid := acceptanceApp(t)
	resp := eval(t, app, sid, `img = cv.open("planted-256x192-16bit.tif", { origin: [200, 80] })
const box = [300, 100, 64, 64]
const spots = img.view("speck").mask.luminance({ above: 0.5 }).components({ minArea: 4 })
return {
  plain: img.crop(box).levels({ auto: true }).look({ zoom: 8, ruler: 8 }),
  speck: img.view("speck").crop(box).look({ zoom: 8, ruler: 8, label: "speck 8×" }),
  marked: img.overlay(spots, { labels: "id" }).crop(box).look({ zoom: 8 }),
}`)
	out := mustOK(t, resp)
	if len(resp.Content) != 3 {
		t.Fatalf("%d looks:\n%s", len(resp.Content), out)
	}
	for _, c := range resp.Content {
		if c.MIMEType != "image/png" {
			t.Fatalf("zoomed look %q is %s", c.Caption, c.MIMEType)
		}
	}
	mustContain(t, out, "view(speck) → crop[300,100,64,64]", "speck 8×", "1 image px = 0.125 doc px", "doc x 300–364, y 100–164")
	if m := decodePNG(t, resp.Content[2].MIMEType, resp.Content[2].Data); m.Bounds().Dx() != 512 || m.Bounds().Dy() != 512 {
		t.Fatalf("marked look %v", m.Bounds())
	}

	// Issue 2: the ruler labels of the crop at (300, 100) read 300 … 364 and 100 … 164.
	m := decodePNG(t, resp.Content[1].MIMEType, resp.Content[1].Data)
	b := m.Bounds()
	if got := numbersIn(m, image.Rect(28, 0, b.Dx()-14, 24)); fmtInts(got) != fmtInts(ints(300, 364, 8)) {
		t.Fatalf("x labels %v", got)
	}
	if got := numbersIn(m, image.Rect(0, 28, 28, b.Dy()-28)); fmtInts(got) != fmtInts(ints(100, 164, 8)) {
		t.Fatalf("y labels %v", got)
	}
}

// Issue 3: cv.sheet of 36 regions at the defaults is one look ≤ 1568 px
// with 36 labelled cells whose ids match the regions.
func TestAcceptanceSheetOf36(t *testing.T) {
	app, sid := acceptanceApp(t)
	resp := eval(t, app, sid, `const top = img.detect.specks().sortBy("area").top(36)
return { ids: (await top).items.map(r => r.id), sheet: cv.sheet(top, { views: img.view("speck") }).look("candidates") }`)
	out := mustOK(t, resp)
	if len(resp.Content) != 1 {
		t.Fatalf("looks:\n%s", out)
	}
	m := decodePNG(t, resp.Content[0].MIMEType, resp.Content[0].Data)
	if b := m.Bounds(); b.Dx() != 1566 || b.Dy() != 1566 {
		t.Fatalf("sheet %v", b)
	}
	i := strings.Index(out, "ids: [")
	ids := strings.Split(out[i+6:i+strings.Index(out[i:], "]")], ", ")
	if len(ids) != 36 {
		t.Fatalf("ids %v", ids)
	}
	for k, id := range ids {
		if k == 0 {
			continue // the look number covers the first label's corner
		}
		gx, gy := (k%6)*262, (k/6)*262
		got := ""
		for x := gx + 2; ; x += font.Width("0", 2) {
			r := glyphAt(m, x, gy+2, 2, "0123456789", light)
			if r == 0 {
				break
			}
			got += string(r)
		}
		if got != id {
			t.Fatalf("cell %d label %q, want region id %s", k, got, id)
		}
	}
}

// Issue 4: a.sub(b) keeps negative values and .add(0.5) displays zero as
// grey 128.
func TestAcceptanceSignedDifference(t *testing.T) {
	app, sid := acceptanceApp(t)
	resp := eval(t, app, sid, `const d = img.sub(img.gaussian(3, { space: "encoded" }))
const zero = img.sub(img).add(0.5)
return { p01: (await d.stats()).p01, zero: zero.crop([0, 0, 64, 64]).look() }`)
	out := mustOK(t, resp)
	mustContain(t, out, "p01: -")
	m := decodePNG(t, resp.Content[0].MIMEType, resp.Content[0].Data)
	if r, g, b, _ := m.At(50, 50).RGBA(); r>>8 != 128 || g>>8 != 128 || b>>8 != 128 {
		t.Fatalf("zero shows as %d %d %d", r>>8, g>>8, b>>8)
	}
}

// Issues 5 and 22: highpass and dog exist; every preset has a paragraph in
// the views topic and renders.
func TestAcceptanceViewsAndTopic(t *testing.T) {
	app, sid := acceptanceApp(t)
	var body string
	for _, tp := range docs.Topics() {
		if tp.ID == "views" {
			body = tp.Body
		}
	}
	mustContain(t, body, "img.highpass(sigma)", "img.dog(sigma1, sigma2)", "localContrast")
	var looks []string
	for _, p := range ops.Presets {
		if !strings.Contains(body, "| `"+p.Name+"` |") {
			t.Errorf("the views topic has no row for preset %q", p.Name)
		}
		looks = append(looks, p.Name+`: img.view("`+p.Name+`").crop([100, 40, 48, 48]).look({ zoom: 2 })`)
	}
	out := mustOK(t, eval(t, app, sid, `return { hp: (await img.highpass(3).stats()).p99 > 0, dog: (await img.dog(1, 4).stats()).p99 > 0 }`))
	mustContain(t, out, "hp: true", "dog: true")
	for i := 0; i < len(looks); i += 4 {
		resp := eval(t, app, sid, "return { "+strings.Join(looks[i:min(i+4, len(looks))], ", ")+" }")
		mustOK(t, resp)
		if len(resp.Content) != min(4, len(looks)-i) {
			t.Fatalf("preset looks: %d", len(resp.Content))
		}
	}
}

// Issue 8: img.crop(box).levels({ auto: true }) maps the crop's p01/p99 to 0/1.
func TestAcceptancePerCropStretch(t *testing.T) {
	app, sid := acceptanceApp(t)
	out := mustOK(t, eval(t, app, sid, `const c = img.crop([150, 20, 64, 64])
const s = await c.stats()
const a = await c.levels({ auto: true }).stats()
const b = await c.levels({ black: s.p01, white: s.p99 }).stats()
return { same: Math.abs(a.meanL - b.meanL) < 1e-6, p99: a.p99 }`))
	mustContain(t, out, "same: true", "p99: 1")
}

// Issue 14: a circle annotation is 1 image px wide at zoom 1 and at zoom 8,
// and its label glyphs are present at both.
func TestAcceptanceAnnotateAtZoom(t *testing.T) {
	app, sid := acceptanceApp(t)
	resp := eval(t, app, sid, `const a = img.annotate([{ circle: [132, 72, 6], color: "lime", label: "7" }])
return { z1: a.crop([112, 52, 40, 40]).look({ zoom: 1 }), z8: a.crop([112, 52, 40, 40]).look({ zoom: 8 }) }`)
	mustOK(t, resp)
	lime := func(r, g, b uint32) bool { return r < 40 && g > 200 && b < 40 }
	for k, c := range resp.Content {
		m := decodePNG(t, c.MIMEType, c.Data)
		b := m.Bounds()
		shortest := 0
		found := false
		for y := 0; y < b.Dy(); y++ {
			run := 0
			for x := 0; x <= b.Dx(); x++ {
				on := false
				if x < b.Dx() {
					r, g, bl, _ := m.At(x, y).RGBA()
					on = lime(r>>8, g>>8, bl>>8)
				}
				if on {
					run++
					continue
				}
				if run > 0 && (shortest == 0 || run < shortest) {
					shortest = run
				}
				run = 0
				if !found && x+10 < b.Dx() && y+14 < b.Dy() && glyphAt(m, x, y, 2, "7", lime) == '7' {
					found = true
				}
			}
		}
		if shortest != 1 || !found {
			t.Fatalf("look %d: stroke %d px, label found %v", k+1, shortest, found)
		}
	}
}

// Issue 15: format png gives a PNG colour look; a zoomed colour look is PNG
// by default.
func TestAcceptanceLossless(t *testing.T) {
	app, sid := acceptanceApp(t)
	resp := eval(t, app, sid, `return { a: img.look({ format: "png" }), b: img.crop([0, 0, 100, 80]).look({ zoom: 4 }), c: img.look({ format: "jpeg" }) }`)
	mustOK(t, resp)
	if resp.Content[0].MIMEType != "image/png" || resp.Content[1].MIMEType != "image/png" || resp.Content[2].MIMEType != "image/jpeg" {
		t.Fatalf("mimes %s %s %s", resp.Content[0].MIMEType, resp.Content[1].MIMEType, resp.Content[2].MIMEType)
	}
}

// Names agents reach for: zoom as a method, cv.view, cv.grid, img.compare,
// subtract.
func TestAcceptanceRedirects(t *testing.T) {
	app, sid := acceptanceApp(t)
	for code, want := range map[string]string{
		`return img.zoom(8)`:                            "fix: 'node.crop([x, y, w, h]).look({ zoom: 8 })'",
		`return cv.view("speck")`:                       "fix: img.view(…)",
		`return cv.grid(img.detect.specks())`:           "fix: cv.sheet(…)",
		`return img.subtract(img)`:                      "fix: img.sub(…)",
		`return img.compare([img])`:                     `fix: 'cv.compare([a, b], { mode: "side" })'`,
		`return img.look({ box: [0, 0, 8, 8] })`:        "fix: node.crop([x, y, w, h]).look()",
		`return img.blend(img, { mode: "difference" })`: "fix: a.sub(b).abs()",
	} {
		mustContain(t, text(eval(t, app, sid, code)), want)
	}
}
