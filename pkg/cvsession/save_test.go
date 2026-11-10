package cvsession_test

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cvsession"
)

// saveApp is a session whose workspace holds a copy of the planted scan
// (opened as img: a file the session opened).
func saveApp(t *testing.T) (*cvsession.App, string, string) {
	t.Helper()
	ws := t.TempDir()
	src, err := os.Open(planted(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Close() }()
	dst, err := os.Create(filepath.Join(ws, "scan.tif"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		t.Fatal(err)
	}
	_ = dst.Close()
	app, sid := newApp(t, cvsession.Options{Image: filepath.Join(ws, "scan.tif"), Root: ws, Read: []string{}})
	return app, sid, ws
}

// TestSaveImagesReopenWithEncodingAndFrame: saved TIFF and PNG files carry
// their encoding and frame origin; NPY reopens as linear at (0, 0) with a
// note; masks are saved linear; cv.open(saved.path) reopens.
func TestSaveImagesReopenWithEncodingAndFrame(t *testing.T) {
	app, sid, ws := saveApp(t)
	res := mustOK(t, eval(t, app, sid, `
const scan = cv.open("scan.tif", { origin: [200, 80] })
const v = scan.view("speck")
const lin = scan.exposure(-0.5)
const a = await cv.save("views/speck.tif", v)
const b = await cv.save("lin.png", lin, { depth: 16 })
const c = await cv.save("lin.npy", lin)
const m = await cv.save("mask.tif", scan.mask.luminance({ above: 0.75 }), { depth: 8 })
const ra = await cv.open(a.path), rb = await cv.open(b.path), rc = await cv.open(c.path), rm = await cv.open(m.path)
const s0 = await lin.stats(), s1 = await cv.open(b.path).stats()
return {
  a: [a.path, a.format, a.depth, a.encoding, a.space, a.frame.join(","), a.replaced].join(" "),
  ra: [ra.encoding, ra.frame.join(","), ra.bitDepth].join(" "),
  rb: [b.encoding, rb.encoding, rb.frame.join(",")].join(" "),
  rc: [rc.encoding, rc.frame.join(","), c.note.includes("linear with origin (0, 0)")].join(" "),
  rm: [m.encoding, rm.encoding, rm.fileChannels].join(" "),
  same: Math.abs(s0.meanL - s1.meanL) < 1e-4,
}`))
	mustContain(t, res, "a: views/speck.tif tiff 16 linear linear 200,80,256,192 false", "ra: linear 200,80,256,192 16", "rb: srgb srgb 200,80,256,192",
		"rc: linear 0,0,256,192 true", "rm: linear linear 1", "same: true")
	if _, err := os.Stat(filepath.Join(ws, "views", "speck.tif")); err != nil {
		t.Fatal(err)
	}
	// the next eval may save the same path again: replaced
	mustContain(t, mustOK(t, eval(t, app, sid, `return (await cv.save("views/speck.tif", img.view("speck"))).replaced`)), "true")
}

// TestSaveData: JSON for any data, CSV for a list of rows; awaited items
// are written without their links; profiles and histograms keep their
// table in rows.
func TestSaveData(t *testing.T) {
	app, sid, ws := saveApp(t)
	res := mustOK(t, eval(t, app, sid, `
const sp = await img.detect.specks()
const a = await cv.save("specks.csv", img.detect.specks())
const b = await cv.save("items.json", sp.slice(0, 3).map(r => ({ ...r, note: "x" })))
const c = await cv.save("rows.csv", sp)
const p = await img.profile([190.5, 46.5], [195.5, 46.5])
const d = await cv.save("profile.csv", p.rows)
const h = await cv.save("hist.json", img.histogram({ bins: 4 }))
return { a: [a.format, a.rows].join(" "), b: b.rows, c: c.rows, n: sp.length, d: d.rows, h: h.format, keys: Object.keys(p).join(",") }`))
	mustContain(t, res, "a: csv 41", "b: 3", "c: 41", "n: 41", "d: 6", "h: json", "keys: 'from,to,step,width,rows'")
	csv, _ := os.ReadFile(filepath.Join(ws, "specks.csv"))
	if lines := strings.Split(strings.TrimSpace(string(csv)), "\n"); len(lines) != 42 || lines[0] != "id,x,y,w,h,cx,cy,area,aspect,contrast,localNoise,sharpness,score,meanL,polarity" {
		t.Fatalf("specks.csv:\n%s", csv)
	}
	items, _ := os.ReadFile(filepath.Join(ws, "items.json"))
	var got []map[string]any
	if err := json.Unmarshal(items, &got); err != nil || len(got) != 3 || got[0]["note"] != "x" || strings.Contains(string(items), "cvref") || strings.Contains(string(items), "cv.item") {
		t.Fatalf("items.json %v: %s", err, items)
	}
	if p, _ := os.ReadFile(filepath.Join(ws, "profile.csv")); !strings.HasPrefix(string(p), "t,cx,cy,meanL,meanR,meanG,meanB\n0,190.5,46.5,") {
		t.Fatalf("profile.csv:\n%s", p)
	}
	if h, _ := os.ReadFile(filepath.Join(ws, "hist.json")); !strings.Contains(string(h), `"rows":[{"lo":0,"hi":0.25,"count":`) {
		t.Fatalf("hist.json:\n%s", h)
	}
}

// TestSaveRefusals: the path, the extension against what is saved, CSV of
// a non-list, duplicates in one eval, symlinks, the opened image.
func TestSaveRefusals(t *testing.T) {
	app, sid, ws := saveApp(t)
	for _, tc := range []struct{ code, want string }{
		{`return await cv.save("../escape.tif", img)`, "code: E_ACCESS"},
		{`return await cv.save("/etc/x.tif", img)`, "outside the workspace"},
		{`return await cv.save("speck.gif", img)`, "code: E_ARG_TYPE"},
		{`return await cv.save("x.png", img, { depth: 32 })`, "code: E_ARG_RANGE"},
		{`return await cv.save("x.jpg", img.exposure(0.2).toGray().encode("linear"))`, `fix: cv.save("x.png", x)`},
		{`return await cv.save("s.tif", img.detect.specks())`, `fix: cv.save("s.json", x)`},
		{`return await cv.save("i.json", img)`, `fix: cv.save("i.tif", x)`},
		{`return await cv.save("o.csv", { a: 1 })`, "CSV is a list of rows (objects); this is an object"},
		{`return await cv.save("st.csv", img.stats())`, `fix: cv.save("st.json", x)`},
		{`return await cv.save("p.csv", img.profile([190.5, 46.5], [195.5, 46.5]))`, `fix: cv.save("p.csv", (await x).rows)`},
		{`return await cv.save("d.json", { a: 1 }, { depth: 8 })`, "code: E_UNKNOWN_OPTION"},
		{`await cv.save("twice.json", 1); return await cv.save("twice.json", 2)`, "code: E_DUPLICATE"},
		{`return await cv.save("scan.tif", img.exposure(0.2))`, "which this session opened"},
	} {
		mustContain(t, text(eval(t, app, sid, tc.code)), tc.want)
	}
	if b, _ := os.ReadFile(filepath.Join(ws, "twice.json")); string(b) != "1\n" {
		t.Fatalf("twice.json %q", b)
	}
	// a symlink inside the workspace: saves do not follow it
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(ws, "link")); err != nil {
		t.Fatal(err)
	}
	mustContain(t, text(eval(t, app, sid, `return await cv.save("link/x.tif", img)`)), "code: E_ACCESS")
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatalf("wrote through the symlink: %v", entries)
	}
	// CLAMP: a signed image to an integer format warns; float TIFF keeps the values
	res := mustOK(t, eval(t, app, sid, `const d = img.sub(img.gaussian(3)); await cv.save("d16.tif", d); const f = await cv.save("d32.tif", d, { depth: 32 }); return f.depth`))
	mustContain(t, res, `CLAMP line 1: cv.save("d16.tif")`, `depth: 32 })`)
	if strings.Count(res, "CLAMP") != 1 {
		t.Fatalf("CLAMP for the float save too:\n%s", res)
	}
	// a failed save leaves the old file
	mustOK(t, eval(t, app, sid, `return (await cv.save("keep.tif", img)).bytes`))
	before, _ := os.ReadFile(filepath.Join(ws, "keep.tif"))
	mustContain(t, text(eval(t, app, sid, `return await cv.save("keep.tif", img.levels({ black: 0.5, white: 0.2 }))`)), "code: E_ARG_RANGE")
	if after, _ := os.ReadFile(filepath.Join(ws, "keep.tif")); string(after) != string(before) {
		t.Fatal("a failed save changed the file")
	}
}

// TestUnawaitedSaves: every save of an eval completes before it returns;
// an unawaited save that fails fails the eval; a handled failure does not;
// other unhandled rejections are UNHANDLED warnings.
func TestUnawaitedSaves(t *testing.T) {
	app, sid, ws := saveApp(t)
	mustOK(t, eval(t, app, sid, `cv.save("u.json", { a: 1 }); cv.save("u.tif", img.view("speck")); return 1`))
	for _, f := range []string{"u.json", "u.tif"} {
		if st, err := os.Stat(filepath.Join(ws, f)); err != nil || st.Size() == 0 {
			t.Fatalf("%s: %v", f, err)
		}
	}
	out := text(eval(t, app, sid, "cv.save(\"bad.tif\", img.levels({ black: 0.5, white: 0.2 }))\nreturn 1"))
	mustContain(t, out, "code: E_ARG_RANGE", `cv.save("bad.tif") was not awaited and failed`, "where: 'line 1: cv.save(")
	mustContain(t, mustOK(t, eval(t, app, sid, `try { await cv.save("bad2.tif", img.levels({ black: 0.5, white: 0.2 })) } catch (e) { return e.code }`)), "E_ARG_RANGE")
	// A save started by another save's continuation also completes before
	// the eval returns (review of PR #6).
	mustOK(t, eval(t, app, sid, `cv.save("c1.tif", img.view("speck")).then(() => cv.save("c2.tif", img.view("detail"))).then(() => cv.save("c3.json", { c: 3 })); return 1`))
	for _, f := range []string{"c1.tif", "c2.tif", "c3.json"} {
		if st, err := os.Stat(filepath.Join(ws, f)); err != nil || st.Size() == 0 {
			t.Fatalf("chained save %s: %v", f, err)
		}
	}
	out = mustOK(t, eval(t, app, sid, `Promise.reject(new Error("boom")); return 1`))
	mustContain(t, out, "UNHANDLED", "boom")
}

// TestSavedFilesReopenAndChangedFiles: a saved file reopens by its path and
// may be saved again; a file that changes before its first decode is
// refused with "open it again".
func TestSavedFilesReopenAndChangedFiles(t *testing.T) {
	app, sid, ws := saveApp(t)
	mustOK(t, eval(t, app, sid, `mem.s = await cv.save("out.tif", img.exposure(0.3)); return 1`))
	mustContain(t, mustOK(t, eval(t, app, sid, `
const again = cv.open(mem.s.path)
const t = await cv.save("out.tif", again.gaussian(1))
return [(await again).w, t.replaced].join(" ")`)), "256 true")
	mustOK(t, eval(t, app, sid, `mem.c = cv.open("out.tif"); return 1`))
	if err := os.WriteFile(filepath.Join(ws, "out.tif"), []byte("not a tiff any more"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustContain(t, text(eval(t, app, sid, `return await mem.c.stats()`)), "code: E_EVAL", "changed since it was opened", `fix: img = cv.open("out.tif")`)
}

// TestRequireLoadsNoFiles: require serves only the built-in modules; a file
// outside (or inside) the workspace cannot be loaded with it (a pi agent
// used require("/abs/lib/tile.js") to bypass --root/--read).
func TestRequireLoadsNoFiles(t *testing.T) {
	app, sid, _ := saveApp(t)
	outside := filepath.Join(t.TempDir(), "evil.js")
	if err := os.WriteFile(outside, []byte(`module.exports = { pwned: true }`), 0o600); err != nil {
		t.Fatal(err)
	}
	out := text(eval(t, app, sid, `try { return require(`+strconvQuote(outside)+`) } catch (e) { return "refused: " + e.message }`))
	mustContain(t, out, "refused:", "use await cv.load(path)")
	if strings.Contains(out, "pwned") {
		t.Fatalf("require loaded a file: %s", out)
	}
	mustContain(t, mustOK(t, eval(t, app, sid, `return typeof require("cv").open`)), "function")
}

func strconvQuote(s string) string { b, _ := json.Marshal(s); return string(b) }
