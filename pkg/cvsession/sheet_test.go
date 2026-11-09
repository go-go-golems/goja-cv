package cvsession_test

import (
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cvsession"
)

// TestSheetCaptionListsCells: a sheet of an array names its cells (label,
// else id, else position) in its caption, in cell order, cut after 40 with
// the count; a sheet of a Regions node says "cells in region order";
// labels: "none" still lists the cells; "index" is not a label mode.
func TestSheetCaptionListsCells(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: planted(t)})
	out := mustOK(t, eval(t, app, sid, `const sp = (await img.detect.specks()).sort((a, b) => b.contrast - a.contrast).slice(0, 4)
const rows = [...sp, { cx: 30, cy: 30, label: "big one" }, [60, 60]]
return {
  a: cv.sheet(rows, { views: img, size: 16, zoom: 2 }).look("a"),
  ids: sp.map(r => r.id).join(" "),
  b: cv.sheet(img.detect.specks(), { views: img, max: 6 }).look("b"),
  c: cv.sheet(rows.slice(0, 2), { views: img, labels: "none" }).look("c"),
}`))
	mustContain(t, out, "ids: 9 11 14 40", `a — sheet(6 items; cells: 9 11 14 40 "big one" 6) (`, "b — sheet(cells in region order) (", "c — sheet(2 items; cells: 9 11) (")
	out = mustOK(t, eval(t, app, sid, `return cv.sheet(Array.from({ length: 45 }, (_, i) => ({ cx: 10 + i, cy: 10, id: 100 + i })), { views: img, size: 8, zoom: 1 }).look()`))
	mustContain(t, out, "sheet(45 items; cells: 100 101 ", " 139 … (5 more))")
	mustContain(t, text(eval(t, app, sid, `return cv.sheet([[10, 10], [20, 20]], { views: img, labels: "index" }).look()`)), "code: E_ARG_TYPE", "index")
}
