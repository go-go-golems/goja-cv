package cvsession_test

import (
	"context"
	"path/filepath"
	"strings"
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
	mustContain(t, out, "stats:", "meanL: 0.29507", "after: '[look 1: after (64×48, 1 image px = 1.0 doc px)]'",
		"context:", "images: img = gradient-64x48-16bit.tif (64×48)")
	mustNotContain(t, out, "UNUSED_NODE")
	if len(resp.Content) != 1 || resp.Content[0].MIMEType != "image/jpeg" || len(resp.Content[0].Data) == 0 {
		t.Fatalf("content %+v", resp.Content)
	}
	if got := resp.Content[0].Caption; got != "look 1: after (64×48, 1 image px = 1.0 doc px)" {
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

func TestShowBeforeThrowStillDeliversTheImage(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	resp := eval(t, app, sid, "show(img.exposure(-1), \"darker\")\nthrow new Error(\"boom\")")
	out := text(resp)
	t.Logf("result:\n%s", out)
	if resp.Cell.Execution.Status == "ok" {
		t.Fatalf("status ok, want an error")
	}
	mustContain(t, out, "error:", "code: Error", "message: boom", "where: 'line 2: throw new Error(\"boom\")'",
		"shown:", "[look 1: darker (64×48, 1 image px = 1.0 doc px)]")
	if len(resp.Content) != 1 || resp.Content[0].MIMEType != "image/jpeg" {
		t.Fatalf("content %+v", resp.Content)
	}
	// show returns its look and also queues it once
	resp = eval(t, app, sid, `const l = show(img, "x"); return { l, same: l === show(l) }`)
	mustContain(t, text(resp), "l: '[look 2: x", "same: true")
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
		`const a = img.exposure(0.3); return { m: await a.stats().meanL }`,
		`mem.a = img.exposure(0.3); return 1`,
		`const a = img.exposure(0.3); show(a); return 1`,
		`const s = img.stats(); const f = img.levels({ black: s.p01, white: s.p99 }); return f.look()`,
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
	mustContain(t, out, "code: E_UNKNOWN_OPTION", "where: 'line 2: ", "fix: 'levels({ black: 0.02 })'", "valid: [black, white, gamma]")

	out = text(eval(t, app, sid, "if (img.stats().meanL > 0.5) return 1"))
	mustContain(t, out, "code: E_LAZY_COMPARE", "where: 'line 1: ")

	out = text(eval(t, app, sid, "const a = 1\nreturn a +* 2"))
	mustContain(t, out, "code: SyntaxError", "where: 'line 2: return a +* 2'")

	out = text(eval(t, app, sid, "return mem.nothing.x"))
	mustContain(t, out, "code: TypeError", "where: 'line 1: return mem.nothing.x'", "keep it in mem")

	out = text(eval(t, app, sid, "return doc.apply(img)"))
	mustContain(t, out, "code: E_NO_DOCUMENT")
}

func TestNoImageThenOpenPersists(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{})
	out := text(eval(t, app, sid, "return await img.stats()"))
	mustContain(t, out, "code: E_NO_IMAGE", "fix: img = cv.open(\"scan.tif\")")
	out = text(eval(t, app, sid, "img = cv.open("+quote(pngPath)+")\nreturn await img"))
	mustContain(t, out, "kind: Image", "images: img = gradient-64x48-8bit.png (64×48)")
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

func TestParamsInContextAndPerson(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	out := text(eval(t, app, sid, `const ev = P.slider("ev", { min: -2, max: 2, value: 0.3 }); return (await img.exposure(ev).stats()).meanL`))
	mustContain(t, out, "params: ev = 0.3")
	out = text(eval(t, app, sid, `P.set("ev", 1); return 1`))
	mustContain(t, out, "params: ev = 1 (moved by the person)", "person: 'since your last eval: moved ev 0.3 → 1'")
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
	resp := eval(t, app, sid, "show(img)\nwhile (true) {}")
	out := text(resp)
	t.Logf("result:\n%s\nerror: %s", out, resp.Cell.Execution.Error)
	mustContain(t, out, "code: E_TIMEOUT", "300ms", "shown:")
	if len(resp.Content) != 1 {
		t.Fatalf("content %d, want the look shown before the timeout", len(resp.Content))
	}
	// the session is still usable
	resp = eval(t, app, sid, "return 1 + 1")
	if resp.Cell.Execution.Status != "ok" {
		t.Fatalf("after timeout: %s %s\n%s", resp.Cell.Execution.Status, resp.Cell.Execution.Error, text(resp))
	}
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
	if len(resp.Content) != 2 || resp.Content[0].MIMEType != "image/png" || resp.Content[1].MIMEType != "image/jpeg" {
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
		"const specks = img.detect.specks()\nreturn specks.heal()": {"code: E_UNKNOWN_METHOD", `hint: '"heal" is an Image method that takes Regions`, "fix: img.heal(regions, …)"},
		`return img.mask.box([0, 0, 10, 10]).exposure(1)`:          {"fix: img.within(mask, i => i.exposure(…))", "an Image method"},
		`return img.detect.specks().feather(2)`:                    {`"feather" is a Mask method`, "fix: regions.toMask().feather(…)"},
		`return img.pick("meanL")`:                                 {`"pick" is a Value method`, "fix: img.stats().pick(…)"},
		`return img.stats().exposure(1)`:                           {"Value has no method \"exposure\"", "fix: img.exposure(…)"},
		`if (img > 1) return 1`:                                    {"An Image node was used as a number"},
		`img.foo = 1`:                                              {`cannot set "foo" on an Image node`},
		`return img.stats().look()`:                                {"stats is a Value node"},
	} {
		out := text(eval(t, app, sid, code))
		mustContain(t, out, want...)
		mustNotContain(t, out, " a Image", " an Mask", " an Value", " an Regions", "TypeError")
	}
	// derived fields that share a name with a method of another kind still work
	mustContain(t, text(eval(t, app, sid, `return await img.check.dust().count`)), "0")
}
