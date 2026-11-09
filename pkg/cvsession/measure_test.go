package cvsession_test

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/internal/testimage"
	"github.com/go-go-golems/goja-cv/pkg/cvsession"
)

// Tests of the JavaScript surface of regions (GCV-004 §6.4): awaited
// regions are arrays of flat items linked to their pixels; items pass as
// regions, boxes and points.

func TestRegionItemsAreFlat(t *testing.T) {
	app, sid := acceptanceApp(t)
	out := mustOK(t, eval(t, app, sid, `
const sp = await img.detect.specks()
const it = sp.slice().sort((a, b) => b.area - a.area)[0]
return { array: Array.isArray(sp), keys: Object.keys(it).join(","), json: JSON.stringify(it).slice(0, 20), box: it.box === undefined }`))
	mustContain(t, out, "array: true", "keys: id,x,y,w,h,cx,cy,area,aspect,contrast,localNoise,sharpness,score,meanL,polarity", `json: '{"id":`, "box: true")
	mustNotContain(t, out, "__cvref", "cv.item")
	// a map that builds new objects drops the link: ITEMS names them
	out = mustOK(t, eval(t, app, sid, `
const sp = await img.detect.specks()
return img.overlay(sp.slice(0, 3).map(r => ({ id: r.id, area: r.area, x: r.x, y: r.y, w: r.w, h: r.h }))).look("lost")`))
	mustContain(t, out, "ITEMS line 3: overlay.target: items 0–2 look like region items but lost their link")
	// regions have no methods
	out = text(eval(t, app, sid, `return await img.detect.specks().filter(r => r.score > 0.5)`))
	mustContain(t, out, "code: E_NOT_AWAITED", "fix: (await img.detect.specks()).filter")
	out = text(eval(t, app, sid, `const sp = await img.detect.specks(); return sp.overlay(img)`))
	mustContain(t, out, "sp is plain data here", "fix: img.overlay(sp, …)")
}

func TestRegionItemsAsArguments(t *testing.T) {
	app, sid := acceptanceApp(t)
	out := mustOK(t, eval(t, app, sid, `
const sp = await img.detect.specks()
const items = sp.slice().sort((a, b) => b.area - a.area).slice(0, 4)
const it = items[0]
const without = sp.filter(r => r.id !== items[0].id && r.id !== items[1].id)
const inside = { area: (await img.stats({ box: it })).area }
const crop = await img.crop(it)
const flood = await img.mask.flood(it).coverage()
const marks = await img.annotate([{ box: it }, { point: it }, { circle: [it.cx, it.cy, 3] }])
const masked = await img.stats({ mask: [it] })
return {
  without: without.length - sp.length,
  inside: Math.round(inside.area) === Math.round(it.w * it.h),
  crop: crop.frame.join(",") === [it.x, it.y, it.w, it.h].join(","),
  flood: flood > 0,
  marks: marks.kind,
  masked: Math.round(masked.area) === Math.round(it.area),
  sheet: cv.sheet(items, { views: img, size: 32, zoom: 2 }).look("items"),
}`))
	mustContain(t, out, "without: -2", "inside: true", "crop: true", "flood: true", "marks: Image", "masked: true", "look 1: items")
}

// dotsPNG writes a w×h gray PNG with an isolated bright pixel at every
// other pixel of every other row: (w/2)·(h/2) one-pixel regions.
func dotsPNG(t *testing.T, w, h int) string {
	t.Helper()
	m := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y += 2 {
		for x := 0; x < w; x += 2 {
			m.Pix[y*w+x] = 255
		}
	}
	p := filepath.Join(t.TempDir(), "dots.png")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, m); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAwaitedRegionsHaveEveryItem(t *testing.T) {
	img := dotsPNG(t, 150, 150)
	app, sid := newApp(t, cvsession.Options{Image: img})
	out := mustOK(t, eval(t, app, sid, `
const r = await img.mask.luminance({ above: 0.5 }).components()
return { count: r.length, last: r[r.length - 1].id, all: r }`))
	mustContain(t, out, "count: 5625", "all: |-", " shown. Next: all.slice(", `all: await cv.save("rows.csv", all)`)
	mustNotContain(t, out, "truncated")
}

// TestMeasureMatchesTheHelper: img.measure under the mapping of guide §13.2
// returns what the colleague's mem.probe computed with masks and stats
// (testdata/probe.js): step, channel steps and stepChroma with gray
// "average" (stepChroma only where |snr| ≥ 1, null elsewhere), spread with
// the default rec709.
func TestMeasureMatchesTheHelper(t *testing.T) {
	app, sid := acceptanceApp(t)
	probe, err := os.ReadFile("testdata/probe.js")
	if err != nil {
		t.Fatal(err)
	}
	mustOK(t, eval(t, app, sid, string(probe)))
	_, p := testimage.PlantedScan()
	var pts []string
	for _, s := range p.Specks[:12] {
		pts = append(pts, fmt.Sprintf("[%d, %d, %d]", int(s.X), int(s.Y), 2+len(pts)%3))
	}
	pts = append(pts, "[200, 46, 3]", "[1, 2, 2]", "[254, 190, 4]") // the star; two next to corners
	out := mustOK(t, eval(t, app, sid, fmt.Sprintf(`
const pts = [%s]
let dStep = 0, dCh = 0, dChroma = 0, dSpread = 0, dSnr = 0
for (const [x, y, r] of pts) {
  const h = await mem.probe(x, y, r)
  const opts = { coreRadius: r + 0.5, ringRadii: [r + 3.5, r + 9.5] }
  const [a] = await img.measure([[x + 0.5, y + 0.5]], { ...opts, gray: "average" })
  const [l] = await img.measure([[x + 0.5, y + 0.5]], opts)
  dStep = Math.max(dStep, Math.abs(a.step - h.step))
  dCh = Math.max(dCh, Math.abs(a.stepR - h.ch[0]), Math.abs(a.stepG - h.ch[1]), Math.abs(a.stepB - h.ch[2]))
  // stepChroma is null below |snr| 1 (the helper divides regardless)
  dChroma = Math.max(dChroma, Math.abs(a.snr) < 1 ? (a.stepChroma === null ? 0 : 1) : Math.abs(a.stepChroma - h.chroma))
  dSpread = Math.max(dSpread, Math.abs(l.spread - h.ringSpread))
  dSnr = Math.max(dSnr, Math.abs(l.step / Math.max(l.spread, 1e-3) - l.snr))
}
return { n: pts.length, ok: [dStep, dCh, dChroma, dSpread, dSnr].every(d => d < 1e-6), d: [dStep, dCh, dChroma, dSpread, dSnr] }`, strings.Join(pts, ", "))))
	mustContain(t, out, "n: 15", "ok: true")
}

// TestMeasurementOps: sample at radius 0 is the pixel, items follow the
// schema order, points outside the frame warn OUTSIDE, a point needs
// coreRadius, and the receiver's space is respected.
func TestMeasurementOps(t *testing.T) {
	app, sid := acceptanceApp(t)
	out := mustOK(t, eval(t, app, sid, `
const [s] = await img.sample([[200.5, 46.5]])
const [l] = await img.as("linear").sample([[200.5, 46.5]])
const inside = await img.stats({ box: [200, 46, 1, 1] })
const rows = await img.measure([[200.5, 46.5], [-20, 4]], { coreRadius: 2.5 })
const p = await img.profile([190.5, 46.5], [210.5, 46.5], { width: 3 })
return {
  keys: Object.keys(rows[0]).join(","),
  pixel: Math.abs(s.meanR - inside.perChannel.r.mean) < 1e-9 && s.area === 1,
  encoded: s.meanB > l.meanB,
  star: rows[0].step > 0 && rows[0].snr > 1,
  outside: rows[1].step,
  profile: p.rows.length + " peak " + (p.rows.map(r => r.meanL).indexOf(Math.max(...p.rows.map(r => r.meanL))) === 10),
}`))
	mustContain(t, out, "keys: id,x,y,w,h,cx,cy,area,aspect,coreMean,ringMean,step,spread,snr,stepR,stepG,stepB,stepChroma,ringCoverage", "pixel: true", "encoded: true",
		"star: true", "outside: null", "profile: 21 peak true", "OUTSIDE: measure: 1 of 2 items lie outside")
	out = text(eval(t, app, sid, `return await img.measure([[10, 10]])`))
	mustContain(t, out, "code: E_ARG_TYPE", "fix: 'img.measure(points, { coreRadius: 3 })'")
	out = text(eval(t, app, sid, `return await img.measure([[10, 10]], { coreRadius: 4, ringRadii: [3, 9] })`))
	mustContain(t, out, "code: E_ARG_RANGE", "ringRadii: [6, 10]")
	out = text(eval(t, app, sid, `return await img.profile([0, 0], [255, 190], { step: 0.05 })`))
	mustContain(t, out, "code: E_ARG_RANGE", "at most 4096")
}

// TestProfileLookIsALineChart: look() of a profile is a PNG line chart of
// the chart size, captioned as a chart; a zoom is refused.
func TestProfileLookIsALineChart(t *testing.T) {
	app, sid := acceptanceApp(t)
	resp := eval(t, app, sid, `return img.profile([180.5, 46.5], [220.5, 46.5], { width: 3 }).look("across the star")`)
	out := mustOK(t, resp)
	mustContain(t, out, "look 1: across the star", "chart)")
	if len(resp.Content) != 1 {
		t.Fatalf("%d content items", len(resp.Content))
	}
	m := decodePNG(t, resp.Content[0].MIMEType, resp.Content[0].Data)
	if b := m.Bounds(); b.Dx() != 512 || b.Dy() != 204 {
		t.Fatalf("chart is %v", b)
	}
	dark := 0
	for y := 0; y < 204; y++ {
		r, g, b, _ := m.At(300, y).RGBA()
		if r>>8 < 100 && g>>8 < 100 && b>>8 < 100 {
			dark++
		}
	}
	if dark == 0 {
		t.Fatal("no meanL line in column 300")
	}
	out = text(eval(t, app, sid, `return img.profile([180.5, 46.5], [220.5, 46.5]).look({ zoom: 2 })`))
	mustContain(t, out, "code: E_ARG_TYPE", "charts have no doc px")
}
