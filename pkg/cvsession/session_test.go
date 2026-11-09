package cvsession_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-go-golems/go-go-goja/pkg/replsession"
	"github.com/go-go-golems/goja-cv/pkg/cvsession"
)

var (
	tiffPath, _ = filepath.Abs(filepath.Join("..", "..", "testdata", "gradient-64x48-16bit.tif"))
	pngPath, _  = filepath.Abs(filepath.Join("..", "..", "testdata", "gradient-64x48-8bit.png"))
)

func newApp(t *testing.T, opts cvsession.Options) (*cvsession.App, string) {
	t.Helper()
	ctx := context.Background()
	app, err := cvsession.New(ctx, opts)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	t.Cleanup(func() { _ = app.Close(context.Background()) })
	s, err := app.CreateSession(ctx)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return app, s.ID
}

func eval(t *testing.T, app *cvsession.App, sid, code string) *replsession.EvaluateResponse {
	t.Helper()
	resp, err := app.Evaluate(context.Background(), sid, code)
	if err != nil {
		t.Fatalf("evaluate %q: %v", code, err)
	}
	return resp
}

func text(resp *replsession.EvaluateResponse) string { return resp.Cell.Execution.Result }

func mustContain(t *testing.T, s string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(s, n) {
			t.Fatalf("text does not contain %q:\n%s", n, s)
		}
	}
}

func mustNotContain(t *testing.T, s string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if strings.Contains(s, n) {
			t.Fatalf("text contains %q:\n%s", n, s)
		}
	}
}

func TestStatsAndLookReturnYAMLAndOneImage(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	resp := eval(t, app, sid, `return { stats: await img.stats(), after: img.exposure(0.5).look("after") }`)
	out := text(resp)
	t.Logf("result:\n%s", out)
	if resp.Cell.Execution.Status != "ok" {
		t.Fatalf("status %s: %s", resp.Cell.Execution.Status, resp.Cell.Execution.Error)
	}
	mustContain(t, out, "stats:", "meanL: 0.29507", "after: '[look 1: after — open(gradient-64x48-16bit.tif) → exposure(0.5) (64×48, 1 image px = 1.0 doc px)]'",
		"context:", "images: img = gradient-64x48-16bit.tif (64×48, 16-bit TIFF, srgb)")
	mustNotContain(t, out, "UNUSED_NODE")
	if len(resp.Content) != 1 || resp.Content[0].MIMEType != "image/png" || len(resp.Content[0].Data) == 0 {
		t.Fatalf("content %+v", resp.Content)
	}
	if got := resp.Content[0].Caption; got != "look 1: after — open(gradient-64x48-16bit.tif) → exposure(0.5) (64×48, 1 image px = 1.0 doc px)" {
		t.Fatalf("caption %q", got)
	}
	// numbers continue across evals; a look delivered again keeps its number
	resp = eval(t, app, sid, `mem.b = img.look("before"); return [mem.b, img.toGray().look("gray")]`)
	mustContain(t, text(resp), "look 2: before", "look 3: gray")
	if len(resp.Content) != 2 || resp.Content[1].MIMEType != "image/png" {
		t.Fatalf("content %+v", resp.Content)
	}
	resp = eval(t, app, sid, `return mem.b`)
	mustContain(t, text(resp), "look 2: before")
	mustContain(t, text(resp), "mem: b (look)")
}

func TestLookBeforeThrowStillDeliversTheImage(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	resp := eval(t, app, sid, "img.exposure(-1).look(\"darker\")\nthrow new Error(\"boom\")")
	out := text(resp)
	t.Logf("result:\n%s", out)
	if resp.Cell.Execution.Status == "ok" {
		t.Fatalf("status ok, want an error")
	}
	mustContain(t, out, "error:", "code: Error", "message: boom", "where: 'line 2: throw new Error(\"boom\")'")
	if len(resp.Content) != 1 || resp.Content[0].MIMEType != "image/png" || !strings.Contains(resp.Content[0].Caption, "look 1: darker") {
		t.Fatalf("content %+v", resp.Content)
	}
	// a returned look is also queued: delivered once; an equal look is the same look
	resp = eval(t, app, sid, `const l = img.look("x"); img.look("x"); return { l }`)
	mustContain(t, text(resp), "l: '[look 2: x")
	if len(resp.Content) != 1 {
		t.Fatalf("content %d items, want 1", len(resp.Content))
	}
}

func TestUnusedNodeWarning(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	resp := eval(t, app, sid, "const s = await img.stats()\nimg.exposure(0.3).gaussian(2)\nreturn s.meanL")
	out := text(resp)
	t.Logf("result:\n%s", out)
	mustContain(t, out, "warnings:", "UNUSED_NODE line 2: img.exposure(0.3).gaussian(2) builds a node and discards it. Nodes are lazy: nothing ran.")
	if strings.Count(out, "UNUSED_NODE") != 1 {
		t.Fatalf("want one UNUSED_NODE for the discarded chain:\n%s", out)
	}
	for _, code := range []string{
		`const a = img.exposure(0.3); return a.look()`,
		`const a = img.exposure(0.3); return { m: (await a.stats()).meanL }`,
		`mem.a = img.exposure(0.3); return 1`,
		`const a = img.exposure(0.3); a.look(); return 1`,
		`const s = await img.stats(); const f = img.levels({ black: s.p01, white: s.p99 }); return f.look()`,
		`return img.exposure(0.4)`,
	} {
		mustNotContain(t, text(eval(t, app, sid, code)), "UNUSED_NODE")
	}
}

func TestRangeWarningHasLine(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	out := text(eval(t, app, sid, "const x = 1\nconst a = img.exposure(5)\nreturn a.look()"))
	mustContain(t, out, "RANGE line 2: ", "outside the usual range")
}

func TestErrorYAMLHasCodeFixAndLine(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	resp := eval(t, app, sid, "const x = 1\nreturn img.brightness(0.3)")
	out := text(resp)
	t.Logf("result:\n%s", out)
	mustContain(t, out, "error:\n  code: E_UNKNOWN_METHOD", "message: Image has no method \"brightness\".",
		"where: 'line 2: return img.brightness(0.3)'", "path: open(gradient-64x48-16bit.tif) → ✗ brightness",
		"hint: Did you mean \"exposure\"?", "fix: img.exposure(…)")

	out = text(eval(t, app, sid, "const a = 1\nreturn img.levels({ blak: 0.02 })"))
	mustContain(t, out, "code: E_UNKNOWN_OPTION", "where: 'line 2: ", "fix: 'levels({ black: 0.02 })'", "valid: [black, white, gamma, auto, space]")

	out = text(eval(t, app, sid, "if (img.stats().meanL > 0.5) return 1"))
	mustContain(t, out, "code: E_NOT_AWAITED", "where: 'line 1: ", "fix: (await img.stats()).meanL")
	out = text(eval(t, app, sid, "if (img.stats() > 0.5) return 1"))
	mustContain(t, out, "code: E_NOT_AWAITED", "fix: (await img.stats()).p99")
	out = text(eval(t, app, sid, "const d = img.dog(1, 4)\nreturn d.levels({ black: 0, white: d.stats() })"))
	mustContain(t, out, "code: E_ARG_TYPE", "where: 'line 2: ", "fix: '{ white: (await node.stats()).p99 }'")

	out = text(eval(t, app, sid, "const a = 1\nreturn a +* 2"))
	mustContain(t, out, "code: SyntaxError", "where: 'line 2: return a +* 2'")

	out = text(eval(t, app, sid, "return mem.nothing.x"))
	mustContain(t, out, "code: TypeError", "where: 'line 1: return mem.nothing.x'", "keep it in mem")

}

func TestNoImageThenOpenPersists(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{})
	out := text(eval(t, app, sid, "return await img.stats()"))
	mustContain(t, out, "code: E_NO_IMAGE", "fix: img = cv.open(\"scan.tif\")")
	out = text(eval(t, app, sid, "img = cv.open("+quote(pngPath)+")\nreturn await img"))
	mustContain(t, out, "kind: Image", "images: img = gradient-64x48-8bit.png (64×48, 8-bit PNG, srgb)")
	out = text(eval(t, app, sid, "mem.n = 3; return (await img.stats()).w"))
	mustContain(t, out, "64", "mem: n (number)")
}

func TestRootsRestrictOpen(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Roots: []string{t.TempDir()}})
	out := text(eval(t, app, sid, "img = cv.open("+quote(tiffPath)+")"))
	mustContain(t, out, "code: E_ACCESS")
	app2, sid2 := newApp(t, cvsession.Options{Roots: []string{filepath.Dir(tiffPath)}})
	out = text(eval(t, app2, sid2, "img = cv.open("+quote(tiffPath)+"); return await img"))
	mustContain(t, out, "kind: Image")
}

func TestLongResultsAreCut(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	out := text(eval(t, app, sid, `return { xs: Array.from({ length: 200 }, (_, i) => i), rows: Array.from({ length: 40 }, (_, i) => ({ i, s: "x".repeat(400) })) }`))
	if len(out) > 9000 {
		t.Fatalf("len %d > 9000", len(out))
	}
	mustContain(t, out, "# … 152 more", "# CUT: the result is")
	out = text(eval(t, app, sid, `return [img.stats(), Promise.resolve(1), () => 1]`))
	mustContain(t, out, "[Value #", "stats()] (lazy", "[Promise] (not awaited", "[Function]")
	out = text(eval(t, app, sid, `const x = 1`))
	mustContain(t, out, "# nothing returned")
}

func TestConsoleIsCaptured(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	resp := eval(t, app, sid, `console.log("hello", 1); return 2`)
	if len(resp.Cell.Execution.Console) != 1 || !strings.Contains(resp.Cell.Execution.Console[0].Message, "hello") {
		t.Fatalf("console %+v", resp.Cell.Execution.Console)
	}
}

func quote(s string) string { return "\"" + strings.ReplaceAll(s, "\"", "\\\"") + "\"" }

func TestTimeoutIsAStructuredError(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath, EvalTimeout: 300 * time.Millisecond})
	resp := eval(t, app, sid, "img.look()\nwhile (true) {}")
	out := text(resp)
	t.Logf("result:\n%s\nerror: %s", out, resp.Cell.Execution.Error)
	mustContain(t, out, "code: E_TIMEOUT", "300ms")
	if len(resp.Content) != 1 {
		t.Fatalf("content %d, want the look shown before the timeout", len(resp.Content))
	}
	// the session is still usable
	resp = eval(t, app, sid, "return 1 + 1")
	if resp.Cell.Execution.Status != "ok" {
		t.Fatalf("after timeout: %s %s\n%s", resp.Cell.Execution.Status, resp.Cell.Execution.Error, text(resp))
	}
}

// TestTimeoutInsideANativeFunction: a native function that runs agent
// JavaScript (cv.json reads a getter) is interrupted by the watchdog; the
// panic boundary lets goja's interrupt through, so the result is E_TIMEOUT,
// not E_INTERNAL (GCV-004 §7.11).
func TestTimeoutInsideANativeFunction(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath, EvalTimeout: 300 * time.Millisecond})
	resp := eval(t, app, sid, "return cv.json({ get x() { while (true) {} } })")
	out := text(resp)
	mustContain(t, out, "code: E_TIMEOUT")
	mustNotContain(t, out, "E_INTERNAL")
}

func TestInterruptInAsyncContinuationIsReported(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath, EvalTimeout: 300 * time.Millisecond})
	out := text(eval(t, app, sid, "await img.stats()\nwhile (true) {}"))
	mustContain(t, out, "code: E_TIMEOUT", "Start a new session")
	start := time.Now()
	resp := eval(t, app, sid, "return 1")
	if resp.Cell.Execution.Status == "ok" || !strings.Contains(resp.Cell.Execution.Error, "Start a new session") || time.Since(start) > 200*time.Millisecond {
		t.Fatalf("broken session should fail fast: %s %q after %s", resp.Cell.Execution.Status, resp.Cell.Execution.Error, time.Since(start))
	}
}

// TestM3KindsInTheSession covers masks, builder lambdas, regions and data
// lambdas through the session (M3 landed on main before the M2 rebase).
func TestM3KindsInTheSession(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	resp := eval(t, app, sid, `const sky = img.mask.gradient({ from: "top", to: 0.45 })
const darker = img.within(sky, i => i.exposure(-0.4))
return { cov: await sky.coverage(), sky: sky.look("sky"), darker: darker.look("darker") }`)
	out := text(resp)
	mustContain(t, out, "cov: 0.22492", "look 1: sky", "look 2: darker")
	mustNotContain(t, out, "UNUSED_NODE")
	if len(resp.Content) != 2 || resp.Content[0].MIMEType != "image/png" || resp.Content[1].MIMEType != "image/png" {
		t.Fatalf("content %+v", resp.Content)
	}
	resp = eval(t, app, sid, `const m = img.mask.luminance({ above: 0.5 })
const big = m.components().filter(x => x.area > 10)
return { n: await big.count(), sel: m.overlay(img).look("sel") }`)
	mustContain(t, text(resp), "n: ", "look 3: sel")
	mustNotContain(t, text(resp), "UNUSED_NODE", "error")
	out = text(eval(t, app, sid, `return img.mask.luminance({ above: 0.5 }).components().look()`))
	mustContain(t, out, "code: E_WRONG_KIND", "img.overlay(regions)")
	resp = eval(t, app, sid, `const r = img.mask.luminance({ above: 0.5 }).components()
return img.overlay(r, { labels: "id" }).look("regions")`)
	if len(resp.Content) != 1 || strings.Contains(text(resp), "UNUSED_NODE") {
		t.Fatalf("regions overlay look:\n%s", text(resp))
	}
	out = text(eval(t, app, sid, "img.within(img.mask.box({ box: [0, 0, 10, 10] }), i => i.exposure(1))\nreturn 1"))
	mustContain(t, out, "UNUSED_NODE line 1: img.within(")
	if strings.Count(out, "UNUSED_NODE") != 1 {
		t.Fatalf("want one UNUSED_NODE:\n%s", out)
	}
}

func TestCrossKindErrorsHaveALiteralFixAndGrammar(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	for code, want := range map[string][]string{
		"const specks = img.detect.specks()\nreturn specks.heal()": {"code: E_NOT_AWAITED", "regions are arguments of image methods", "fix: img.heal(regions, …)"},
		`return img.mask.box([0, 0, 10, 10]).exposure(1)`:          {"fix: img.within(mask, i => i.exposure(…))", "an Image method"},
		`return img.detect.specks().feather(2)`:                    {`"feather" is a Mask method`, "fix: regions.toMask().feather(…)"},
		`return img.stats().exposure(1)`:                           {"Value has no method \"exposure\"", "fix: img.exposure(…)"},
		`if (img > 1) return 1`:                                    {"code: E_NOT_AWAITED", "fix: (await img).w"},
		`img.foo = 1`:                                              {`cannot set "foo" on an Image node`},
		`return img.stats().look()`:                                {"stats is a Value node"},
	} {
		out := text(eval(t, app, sid, code))
		mustContain(t, out, want...)
		mustNotContain(t, out, " a Image", " an Mask", " an Value", " an Regions", "TypeError")
	}
	// a field that shares its name with a method of another kind is read after await
	mustContain(t, text(eval(t, app, sid, `return await img.check.dust().count`)), "code: E_NOT_AWAITED", "fix: (await img.check.dust()).count")
	mustContain(t, text(eval(t, app, sid, `return (await img.check.dust()).count`)), "0")
}

// TestConcurrentEvalsKeepTheirOwnSource: agents (pi) issue parallel tool
// calls; each eval's error must quote its own line, not another eval's.
func TestConcurrentEvalsKeepTheirOwnSource(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	const n = 12
	var wg sync.WaitGroup
	outs := make([]string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			code := fmt.Sprintf("const x%d = await img.stats()\nreturn x%d.nope%d.y", i, i, i)
			resp, err := app.Evaluate(context.Background(), sid, code)
			if err != nil {
				t.Error(err)
				return
			}
			outs[i] = text(resp)
		}(i)
	}
	wg.Wait()
	for i, out := range outs {
		want := fmt.Sprintf("where: 'line 2: return x%d.nope%d.y'", i, i)
		if !strings.Contains(out, want) {
			t.Fatalf("eval %d: want %q in\n%s", i, want, out)
		}
	}
}

// TestBatchedHelpNeverThrows is pi run 2 call 17: one unknown name used to
// kill the whole eval.
func TestBatchedHelpNeverThrows(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	resp := eval(t, app, sid, `return [await cv.help("overlay"), await cv.help("mask.luminance"), await cv.help("Mask.coverage"), cv.help("explain"), cv.help("nope")]`)
	out := text(resp)
	if resp.Cell.Execution.Status != "ok" {
		t.Fatalf("status %s:\n%s", resp.Cell.Execution.Status, out)
	}
	mustContain(t, out, "overlay:", "mask.luminance:", "coverage:", "explain:", "found: false", "name: nope")
}

// TestAwaitedDataKeepsItsNode is pi run 1 calls 10, 15, 23 and 24: agents
// await a node and keep using the result as the node.
func TestAwaitedDataKeepsItsNode(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: planted(t)})
	resp := eval(t, app, sid, `const r = await img.detect.specks({ polarity: 'both' })
const dust = r.where({ score: { gte: 0.5 } })
const viaFilter = r.filter(x => x.score >= 0.5)
const healed = img.heal(r.where({ score: { gte: 0.5 } }), { radius: 3 })
return {
  n: r.count, keys: Object.keys(r), json: Object.keys(JSON.parse(JSON.stringify(r))),
  dust: await dust.count(), filtered: (await viaFilter).count,
  explain: cv.explain(img.detect.specks({ polarity: 'both' })).graph.length > 0,
  after: (await healed.check.dust()).count,
  healedDirect: (await img.heal(await dust).check.dust()).count,
  overlay: img.overlay(r, { labels: "id" }).look("all"),
  summary: Object.keys(await img),
  brighter: (await (await img).exposure(1).stats()).meanL > (await img.stats()).meanL,
}`)
	out := text(resp)
	t.Logf("%s", out)
	if resp.Cell.Execution.Status != "ok" {
		t.Fatalf("status %s:\n%s", resp.Cell.Execution.Status, out)
	}
	mustContain(t, out, "n: 41", "keys: [count, frame, items, kind, scale]", "json: [count, frame, items, kind, scale]",
		"dust: 40", "filtered: 40", "explain: true", "after: 0", "healedDirect: 0", "look 1: all", "brighter: true")
	mustNotContain(t, out, "where:", "UNUSED_NODE")
	// returning awaited data renders exactly the data
	out = text(eval(t, app, sid, `const r = await img.detect.specks(); return { count: r.count, items: r.items.length, w: r.frame[2] }`))
	mustContain(t, out, "{count: 41, items: 41, w: 256}")
	out = text(eval(t, app, sid, `const s = await img.stats(); return s`))
	mustNotContain(t, out, "levels", "exposure", "pick:")
	// plain objects that never came from a node still get a structured error with a fix
	out = text(eval(t, app, sid, `return img.overlay({ items: [] }).look()`))
	mustContain(t, out, "code: E_WRONG_KIND", "fix:")
}

// TestImageMetadata: both pi runs inferred bit depth from histogram
// quantization; awaiting a source image now says it.
func TestImageMetadata(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	out := text(eval(t, app, sid, `return { img: await img, d: cv.explain(img), derived: await img.exposure(1) }`))
	mustContain(t, out, "bitDepth: 16", "format: tiff", "file: gradient-64x48-16bit.tif", "encoding: srgb", "sample: uint", "frame: [0, 0, 64, 48]",
		"images: img = gradient-64x48-16bit.tif (64×48, 16-bit TIFF, srgb)")
	out = text(eval(t, app, sid, `img = cv.open(`+quote(pngPath)+`); return await img`))
	mustContain(t, out, "bitDepth: 8", "format: png", "file: gradient-64x48-8bit.png")
}

func TestArgumentErrorsHaveFixesAndWarningsFold(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	for code, want := range map[string][]string{
		`return await img.mask.luminance({}).coverage()`:                                    {"code: E_ARG_TYPE", "luminance levels in [0, 1]", "fix: 'img.mask.luminance({ above: 0.7 })"},
		`return await img.mask.flood([500, 10]).coverage()`:                                 {"code: E_ARG_RANGE", "fix: img.mask.flood([63, 10])"},
		`return await img.mask.gradient({ from: "up" }).coverage()`:                         {"fix: 'mask.gradient({ from: \"top\" })'"},
		`return await img.overlay(img.mask.box([0, 0, 9, 9]), { color: "chartreuse-ish" })`: {"fix: 'color: \"red\""},
	} {
		mustContain(t, text(eval(t, app, sid, code)), want...)
	}
	out := text(eval(t, app, sid, "let m = 0\nfor (let i = 0; i < 5; i++) m += (await img.exposure(4).stats()).meanL\nreturn m"))
	mustContain(t, out, "RANGE line 2: ", "(×5)")
	if strings.Count(out, "RANGE") != 1 {
		t.Fatalf("warnings not folded:\n%s", out)
	}
}

// TestUnknownNamesOnCv: pi run 3 call 1 was `await cv.info()`.
func TestUnknownNamesOnCv(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	for code, want := range map[string][]string{
		`return await cv.info()`:        {"code: E_UNKNOWN_METHOD", `cv has no "info"`, "fix: await img   // or cv.explain(img)"},
		`return await cv.stats(img)`:    {`cv has no "stats"`, `"stats" is an Image method`, "fix: img.stats(…)"},
		`return cv.comapre([img, img])`: {`Did you mean "compare"?`, "fix: cv.compare(…)"},
		`return cv.evaluate(img)`:       {"code: E_UNKNOWN_METHOD", `cv has no "evaluate"`},
	} {
		mustContain(t, text(eval(t, app, sid, code)), want...)
	}
	// existing members, probes and feature checks still work
	out := text(eval(t, app, sid, `return { v: typeof cv.version, open: typeof cv.open, then: cv.then === undefined, json: JSON.stringify(cv).length > 2, keys: Object.keys(cv).includes("docs"), has: "info" in cv }`))
	mustContain(t, out, "v: string", "open: function", "then: true", "json: true", "keys: true", "has: false")
}

// TestAwaitedRegionsMisuseGetsStructuredErrors: pi run 3 calls 7 and 8, run 4
// call 5, verbatim.
func TestAwaitedRegionsMisuseGetsStructuredErrors(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: planted(t)})
	for code, want := range map[string][]string{
		// run 3 call 7: count is data after await
		`const dust = await img.check.dust({});
const specks = await img.detect.specks({});
const cnt = await specks.count();
return { cnt }`: {"code: TypeError", "where: 'line 3: const cnt = await specks.count();'",
			"After await, specks is data: specks.count is a number (41), not a method.", "fix: specks.count"},
		// run 4 call 5
		`const specks = img.detect.specks();
const all = await specks.sortBy("area");
return { total: all.count ? await all.count() : null }`: {"After await, all is data: all.count is a number (41)", "fix: all.count"},
		// run 3 call 8: awaited top(n) used as an array
		`const specks = await img.detect.specks({});
const rows = await specks.top(1000);
return { first: rows.slice(0, 3) }`: {"code: E_NOT_ARRAY", "where: 'line 3: return { first: rows.slice(0, 3) }'",
			"Awaited Regions are { count, items: [...] }", "fix: rows.items.slice(…)"},
		`const rows = await img.detect.specks()
const ids = rows.map(r => r.id)
return ids`: {"code: E_NOT_ARRAY", "fix: rows.items.map(…)"},
		`const rows = await img.detect.specks()
let n = 0
for (const r of rows) n++
return n`: {"code: E_NOT_ARRAY", "for (const r of regions.items)"},
	} {
		out := text(eval(t, app, sid, code))
		mustContain(t, out, want...)
	}
	// the data itself is unchanged
	out := text(eval(t, app, sid, `const r = await img.detect.specks(); return { n: r.count, items: r.items.length, keys: Object.keys(r).join(",") }`))
	mustContain(t, out, "n: 41", "items: 41", "keys: 'count,frame,items,kind,scale'")
}

func TestRawTypeErrorsOnDataNameTheOp(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	out := text(eval(t, app, sid, "const s = await img.stats()\nreturn s.exposure(1)"))
	mustContain(t, out, "code: TypeError", "s is plain data here, not a node", `"exposure" is an Image method`, "fix: img.exposure(…)")
	out = text(eval(t, app, sid, "const o = { a: 1 }\nreturn o.detect.specks()"))
	mustContain(t, out, "code: TypeError") // not a cv name: left alone
}

// TestLookQueue (GCV-004 §7.7, D9): every look created in an eval is
// delivered whether or not it is returned; returned looks first, then the
// others in creation order; equal looks once; at most MaxLooks, the others
// are not rendered and one LOOKS warning names each by line; looks of a
// prelude are discarded; a look created after the eval settled is named by
// the next eval's warnings.
func TestLookQueue(t *testing.T) {
	prelude := filepath.Join(t.TempDir(), "prelude.js")
	if err := os.WriteFile(prelude, []byte("img.look(\"from the prelude\")\nmem.p = 1\nreturn 1"), 0o644); err != nil { // #nosec G306 -- test data
		t.Fatal(err)
	}
	app, sid := newApp(t, cvsession.Options{Image: tiffPath, Preludes: []string{prelude}})
	captions := func(resp *replsession.EvaluateResponse) []string {
		var out []string
		for _, c := range resp.Content {
			out = append(out, c.Caption)
		}
		return out
	}
	// not returned: still delivered (the prelude's look was discarded: this is look 1)
	resp := eval(t, app, sid, "img.look(\"a\")\nreturn 1")
	if c := captions(resp); len(c) != 1 || !strings.HasPrefix(c[0], "look 1: a") {
		t.Fatalf("unreturned look: %q\n%s", c, text(resp))
	}
	// returned looks first, then the others in creation order; equal looks once
	resp = eval(t, app, sid, "const a = img.exposure(1).look(\"a\")\nimg.look(\"b\")\nimg.look(\"b\")\nreturn { c: img.exposure(-1).look(\"c\") }")
	if c := captions(resp); len(c) != 3 || !strings.HasPrefix(c[0], "look 2: c") || !strings.HasPrefix(c[1], "look 3: a") || !strings.HasPrefix(c[2], "look 4: b") {
		t.Fatalf("order: %q", c)
	}
	// six looks: four delivered, a LOOKS warning names the other two by line
	resp = eval(t, app, sid, "img.look(\"1\")\nimg.look(\"2\")\nimg.look(\"3\")\nimg.look(\"4\")\nimg.look(\"5\")\nreturn { six: img.look(\"6\") }")
	out := text(resp)
	if c := captions(resp); len(c) != 4 || !strings.HasPrefix(c[0], "look 5: 6") {
		t.Fatalf("budget: %q\n%s", c, out)
	}
	mustContain(t, out, `LOOKS line 4: 4 of 6 looks delivered; not delivered: line 4 look("4"), line 5 look("5"). Return one cv.sheet of the crops, or look in two evals.`)
	// a look created after the eval returned is not delivered; the next eval names it
	resp = eval(t, app, sid, "const f = () => img.look(\"late\")\nimg.gaussian(20).stats().then(f, f)   // runs after the eval returned\nreturn 1")
	if len(resp.Content) != 0 {
		t.Fatalf("late look delivered: %q", captions(resp))
	}
	time.Sleep(300 * time.Millisecond)
	out = text(eval(t, app, sid, "return 2"))
	mustContain(t, out, `LOOKS: look("late") was created after the previous eval had returned`)
}
