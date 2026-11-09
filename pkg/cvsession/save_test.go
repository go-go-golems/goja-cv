package cvsession_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cvsession"
)

func saveApp(t *testing.T) (*cvsession.App, string, string) {
	t.Helper()
	img := planted(t)
	out := t.TempDir()
	app, sid := newApp(t, cvsession.Options{Image: img, BaseDir: filepath.Dir(img), OutRoot: out})
	return app, sid, out
}

// TestSaveImagesReopenWithEncodingAndFrame: saved TIFF and PNG files carry
// their encoding and frame origin; NPY reopens as linear at (0, 0) with a
// note; masks are saved linear.
func TestSaveImagesReopenWithEncodingAndFrame(t *testing.T) {
	app, sid, out := saveApp(t)
	res := mustOK(t, eval(t, app, sid, `
const scan = cv.open((await img).file, { origin: [200, 80] })
const v = scan.view("speck")
const lin = scan.exposure(-0.5)
const a = await v.save("views/speck.tif")
const b = await lin.save("lin.png", { depth: 16 })
const c = await lin.save("lin.npy")
const m = await scan.mask.luminance({ above: 0.75 }).save("mask.tif", { depth: 8 })
const ra = await cv.open(a.path), rb = await cv.open(b.path), rc = await cv.open(c.path), rm = await cv.open(m.path)
const s0 = await lin.stats(), s1 = await cv.open(b.path).stats()
return {
  a: [a.format, a.depth, a.encoding, a.frame.join(","), a.replaced].join(" "),
  ra: [ra.encoding, ra.frame.join(","), ra.bitDepth].join(" "),
  rb: [b.encoding, rb.encoding, rb.frame.join(",")].join(" "),
  rc: [rc.encoding, rc.frame.join(","), c.note.includes("linear with origin (0, 0)")].join(" "),
  rm: [m.encoding, rm.encoding, rm.fileChannels].join(" "),
  same: Math.abs(s0.meanL - s1.meanL) < 1e-4,
  again: (await v.save("views/speck.tif")).replaced,
}`))
	mustContain(t, res, "a: tiff 16 linear 200,80,256,192 false", "ra: linear 200,80,256,192 16", "rb: srgb srgb 200,80,256,192",
		"rc: linear 0,0,256,192 true", "rm: linear linear 1", "same: true", "again: true")
	if _, err := os.Stat(filepath.Join(out, "views", "speck.tif")); err != nil {
		t.Fatal(err)
	}
}

func TestSaveRefusals(t *testing.T) {
	app, sid, out := saveApp(t)
	for _, tc := range []struct{ code, want string }{
		{`return await img.save("../escape.tif")`, "outside the out-root"},
		{`return await img.save("/etc/x.tif")`, "outside the out-root"},
		{`return await img.save("speck.gif")`, "code: E_ARG_TYPE"},
		{`return await img.save("x.png", { depth: 32 })`, "code: E_ARG_RANGE"},
		{`return await img.exposure(0.2).toGray().encode("linear").save("x.jpg")`, "fix: node.save(\"x.png\")"},
		{`return await img.detect.specks().save("s.csv")`, "cv.json(x).save(\"s.json\")"},
	} {
		mustContain(t, text(eval(t, app, sid, tc.code)), tc.want)
	}
	// a symlink inside the out-root that points outside it
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(out, "link")); err != nil {
		t.Fatal(err)
	}
	mustContain(t, text(eval(t, app, sid, `return await img.save("link/x.tif")`)), "code: E_ACCESS", "outside the out-root")
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatalf("wrote through the symlink: %v", entries)
	}
	// CLAMP: a signed image to an integer format warns; float TIFF keeps the values
	res := mustOK(t, eval(t, app, sid, `const d = img.sub(img.gaussian(3)); await d.save("d16.tif"); const f = await d.save("d32.tif", { depth: 32 }); return f.depth`))
	mustContain(t, res, "CLAMP line 1: save(\"d16.tif\")", `depth: 32 })`)
	if strings.Count(res, "CLAMP") != 1 {
		t.Fatalf("CLAMP for the float save too:\n%s", res)
	}
	// no out-root: saving is disabled
	img := planted(t)
	app2, sid2 := newApp(t, cvsession.Options{Image: img, BaseDir: filepath.Dir(img)})
	mustContain(t, text(eval(t, app2, sid2, `return await img.save("x.tif")`)), "code: E_ACCESS", "saving is disabled", "--out-root DIR")
}
