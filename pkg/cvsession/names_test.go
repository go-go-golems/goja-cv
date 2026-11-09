package cvsession_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/go-go-golems/goja-cv/internal/testimage"
	"github.com/go-go-golems/goja-cv/pkg/cv/buffer"
	"github.com/go-go-golems/goja-cv/pkg/cvsession"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNamedImagesPersistAndAreListed(t *testing.T) {
	dir := t.TempDir()
	app, sid := newApp(t, cvsession.Options{Image: tiffPath, Read: []string{filepath.Dir(tiffPath), dir}})
	src, _ := testimage.PlantedScan()
	npy := filepath.Join(dir, "tile.npy")
	if err := testimage.WriteFile(npy, src, 32, buffer.Meta{}); err != nil {
		t.Fatal(err)
	}
	dng := filepath.Join(dir, "raw.dng")
	if err := testimage.WriteDNG(dng, src, testimage.DNGOptions{}); err != nil {
		t.Fatal(err)
	}

	out := text(eval(t, app, sid, `cv.open(`+quote(pngPath)+`, { name: "ref" })
cv.open(`+quote(dng)+`, { name: "scan", box: [10, 20, 64, 48] })
cv.open(`+quote(npy)+`)
return { ref: (await ref).file, scan: (await scan).frame }`))
	mustContain(t, out, "ref: gradient-64x48-8bit.png", "scan: [10, 20, 64, 48]",
		"images: img = gradient-64x48-16bit.tif (64×48, 16-bit TIFF, srgb), ref = gradient-64x48-8bit.png (64×48, 8-bit PNG, srgb), scan = raw.dng [10,20,64,48] (16-bit DNG, linear), (unnamed) tile.npy (256×192, float32 NPY, linear)")
	mustNotContain(t, out, "UNUSED_NODE", "error")

	// names persist across evals; same-frame ops across names work
	out = text(eval(t, app, sid, `return { both: (await img.blend(ref, { opacity: 0.5 })).w, scanEnc: (await scan).encoding }`))
	mustContain(t, out, "both: 64", "scanEnc: linear")

	// rebinding a name replaces it; img = … still works
	out = text(eval(t, app, sid, `cv.open(`+quote(tiffPath)+`, { name: "ref" }); img = cv.open(`+quote(pngPath)+`); return (await ref).file`))
	mustContain(t, out, "gradient-64x48-16bit.tif", "images: img = gradient-64x48-8bit.png (64×48, 8-bit PNG, srgb), ref = gradient-64x48-16bit.tif")

	// synonyms of as
	out = text(eval(t, app, sid, `cv.open(`+quote(pngPath)+`, { name: "third" }); return (await third).file`))
	mustContain(t, out, "gradient-64x48-8bit.png", "third = gradient-64x48-8bit.png")
}

func TestBadNamesAreENAME(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	for code, want := range map[string][]string{
		`cv.open(` + quote(pngPath) + `, { name: "mem" })`:       {"code: E_NAME", `"mem" is a session global`, `fix: '{ name: "mem2" }'`},
		`cv.open(` + quote(pngPath) + `, { name: "2nd" })`:       {"code: E_NAME", "must be an identifier", "fix:"},
		`cv.open(` + quote(pngPath) + `, { name: "Math" })`:      {"code: E_NAME", "fix:"},
		`cv.open(` + quote(pngPath) + `, { name: "class" })`:     {"code: E_NAME"},
		`cv.open(` + quote(pngPath) + `, { name: "__cv_run" })`:  {"code: E_NAME", "already a global"},
		`cv.open(` + quote(pngPath) + `, { name: 3 })`:           {"code: E_NAME", "must be a string"},
		`cv.open(` + quote(pngPath) + `, { encoding: "gamma" })`: {"code: E_ARG_TYPE", `cv.open.encoding = "gamma" is not one of "srgb", "linear".`},
	} {
		out := text(eval(t, app, sid, code+"\nreturn 1"))
		mustContain(t, out, want...)
	}
	// a failed name binds nothing
	out := text(eval(t, app, sid, `return typeof mem`))
	mustContain(t, out, "object")
}

func TestMCPOpenToolWithNameAndEncoding(t *testing.T) {
	dir := t.TempDir()
	cs := connect(t, cvsession.Options{Image: tiffPath, Read: []string{filepath.Dir(tiffPath), dir}})
	lin := filepath.Join(dir, "lin.tif")
	src, _ := testimage.PlantedScan()
	if err := testimage.WriteFile(lin, src, 16, buffer.Meta{}); err != nil {
		t.Fatal(err)
	}
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "open",
		Arguments: map[string]any{"path": lin, "name": "v", "encoding": "linear"}})
	if err != nil {
		t.Fatal(err)
	}
	out, images, isErr := splitResult(t, res)
	if isErr || len(images) != 1 {
		t.Fatalf("open: isErr=%v images=%d\n%s", isErr, len(images), out)
	}
	mustContain(t, out, "encoding: linear", "images: img = gradient-64x48-16bit.tif (64×48, 16-bit TIFF, srgb), v = lin.tif (256×192, 16-bit TIFF, linear)")
	out, _, _ = callEval(t, cs, `return (await v.stats()).w`)
	mustContain(t, out, "256")
}

// TestSpaceVocabulary: synonyms, value synonyms and help entries for the
// GCV-001 vocabulary.
func TestSpaceVocabulary(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	for code, want := range map[string][]string{
		`return cv.open(` + quote(pngPath) + `, { encodng: "linear" })`:  {"code: E_UNKNOWN_OPTION", `Did you mean "encoding"?`, `fix: 'cv.open(path, { encoding: … })'`},
		`return img.as("display")`:                                       {"code: E_ARG_TYPE", "valid: [linear, encoded]"},
		`return cv.open(` + quote(pngPath) + `, { encoding: "linera" })`: {"code: E_ARG_TYPE", `fix: 'cv.open(path, { encoding: "linear" })'`},
		`return img.tolab()`:                                             {"code: E_UNKNOWN_METHOD", `Did you mean "toLab"?`},
		`return img.colormp()`:                                           {"code: E_UNKNOWN_METHOD", `Did you mean "colormap"?`},
	} {
		out := text(eval(t, app, sid, code))
		mustContain(t, out, want...)
	}
	out := text(eval(t, app, sid, `return [cv.help("space"), cv.help("encoding"), cv.help("frame"), cv.help("names")].map(h => h.found !== false)`))
	mustContain(t, out, "[true, true, true, true]")
	out = text(eval(t, app, sid, `return cv.help("space")`))
	mustContain(t, out, "encodings-and-spaces")
}
