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

// GCV-003 tests of the JavaScript surface: flat region items, removed field
// names, items passed as boxes, points and ids.

func TestRegionItemsAreFlat(t *testing.T) {
	app, sid := acceptanceApp(t)
	out := mustOK(t, eval(t, app, sid, `
const specks = img.detect.specks()
const it = (await specks.sortBy("area").top(1)).items[0]
return { keys: Object.keys(it).join(","), json: JSON.stringify(it).slice(0, 20), has: "box" in it, own: Object.prototype.hasOwnProperty.call(it, "box") }`))
	mustContain(t, out, "keys: id,x,y,w,h,cx,cy,area,aspect,contrast,localNoise,sharpness,score,meanL,polarity", `json: '{"id":`, "has: true", "own: false")

	for _, code := range []string{
		`return (await img.detect.specks()).items[0].box`,
		`const c = (await img.detect.specks()).items[0].centroid; return c`,
		`return await img.detect.specks().filter(r => r.box[2] > 3).count()`,
	} {
		out := text(eval(t, app, sid, code))
		mustContain(t, out, "code: E_NO_FIELD", "pass the item itself")
	}
	out = text(eval(t, app, sid, `return await img.mask.luminance({ above: 0.75 }).components().where({ score: { gte: 0.5 } })`))
	mustContain(t, out, "code: E_UNKNOWN_OPTION", "id, x, y, w, h, cx, cy, area, aspect", "score comes from img.detect.specks()")
}

func TestRegionItemsAsArguments(t *testing.T) {
	app, sid := acceptanceApp(t)
	out := mustOK(t, eval(t, app, sid, `
const specks = img.detect.specks().sortBy("area")
const items = (await specks.top(4)).items
const it = items[0]
const only = await specks.only(it)
const without = await specks.without(items.slice(0, 2))
const inside = await img.stats({ box: it }).pick("area")
const crop = await img.crop(it)
const flood = await img.mask.flood(it).coverage()
const marks = await img.annotate([{ box: it }, { point: it }, { circle: [it.cx, it.cy, 3] }])
return {
  only: only.count + " " + (only.items[0].id === it.id),
  without: without.count - (await specks).count,
  inside: Math.round(inside.area) === Math.round(it.w * it.h),
  crop: crop.frame.join(",") === [it.x, it.y, it.w, it.h].join(","),
  flood: flood > 0,
  marks: marks.kind,
  sheet: cv.sheet(items, { views: img, size: 32, zoom: 2 }).look("items"),
}`))
	mustContain(t, out, "only: 1 true", "without: -2", "inside: true", "crop: true", "flood: true", "marks: Image", "look 1: items")
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
	app, sid := newApp(t, cvsession.Options{Image: img, BaseDir: filepath.Dir(img)})
	out := mustOK(t, eval(t, app, sid, `
const r = await img.mask.luminance({ above: 0.5 }).components()
return { count: r.count, items: r.items.length, last: r.items[r.items.length - 1].id, all: r.items }`))
	mustContain(t, out, "count: 5625", "items: 5625", "more (return cv.json(x) for all)")
	mustNotContain(t, out, "truncated")
}

// TestMeasureMatchesTheHelper: img.measure under the mapping of guide §13.2
// returns what the colleague's mem.probe computed with masks and stats
// (testdata/probe.js): step, channel steps and stepChroma with gray
// "average", spread with the default rec709.
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
  dChroma = Math.max(dChroma, Math.abs(a.stepChroma - h.chroma))
  dSpread = Math.max(dSpread, Math.abs(l.spread - h.ringSpread))
  dSnr = Math.max(dSnr, Math.abs(l.step / Math.max(l.spread, 1e-3) - l.snr))
}
return { n: pts.length, ok: [dStep, dCh, dChroma, dSpread, dSnr].every(d => d < 1e-6), d: [dStep, dCh, dChroma, dSpread, dSnr] }`, strings.Join(pts, ", "))))
	mustContain(t, out, "n: 15", "ok: true")
}

// TestMeasurementOps: sample at radius 0 is the pixel, rows follow the
// schema order, points outside the frame warn OUTSIDE, Regions are refused
// with the fix, and the space option is respected.
func TestMeasurementOps(t *testing.T) {
	app, sid := acceptanceApp(t)
	out := mustOK(t, eval(t, app, sid, `
const [s] = await img.sample([[200.5, 46.5]])
const [e] = await img.sample([[200.5, 46.5]], { space: "encoded" })
const inside = await img.stats({ box: [200, 46, 1, 1] })
const rows = await img.measure([[200.5, 46.5], [-20, 4]], { coreRadius: 2.5 })
const p = await img.profile([190.5, 46.5], [210.5, 46.5], { width: 3 })
return {
  keys: Object.keys(rows[0]).join(","),
  pixel: Math.abs(s.meanR - inside.perChannel.r.mean) < 1e-9 && s.area === 1,
  encoded: e.meanB > s.meanB,
  star: rows[0].step > 0 && rows[0].snr > 1,
  outside: rows[1].outside + " " + rows[1].step,
  profile: p.t.length + " " + p.meanL.length + " peak " + (p.meanL.indexOf(Math.max(...p.meanL)) === 10),
}`))
	mustContain(t, out, "keys: cx,cy,coreMean,ringMean,step,spread,snr,stepR,stepG,stepB,stepChroma,ringCoverage", "pixel: true", "encoded: true",
		"star: true", "outside: true null", "profile: 21 21 peak true", "OUTSIDE: measure: 1 of 2 points lie outside")
	out = text(eval(t, app, sid, `return await img.measure(img.detect.specks())`))
	mustContain(t, out, "code: E_WRONG_KIND", "regions.measure(img, { ring: true })")
	out = text(eval(t, app, sid, `return await img.measure([[10, 10]], { coreRadius: 4, ringRadii: [3, 9] })`))
	mustContain(t, out, "code: E_ARG_RANGE", "ringRadii: [7, 13]")
	out = text(eval(t, app, sid, `return await img.profile([0, 0], [255, 190], { step: 0.05 })`))
	mustContain(t, out, "code: E_ARG_RANGE", "at most 4096")
}
