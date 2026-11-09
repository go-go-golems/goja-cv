package cvsession_test

import (
	"testing"
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
