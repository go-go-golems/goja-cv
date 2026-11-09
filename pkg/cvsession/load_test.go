package cvsession_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cvsession"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// writeFiles writes files (relative name → text) under dir.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil { // #nosec G306 -- test scripts
			t.Fatal(err)
		}
	}
}

// loadApp is a session with the planted scan, a workspace and two read
// directories (the scan's, then lib) holding scripts.
func loadApp(t *testing.T, opts cvsession.Options) (*cvsession.App, string, string, string) {
	t.Helper()
	img := planted(t)
	ws, lib := t.TempDir(), t.TempDir()
	writeFiles(t, lib, map[string]string{
		"tl/helpers.js": "// tl/helpers.js: helpers for tests\nmem.f = x => { // add one\n  return x + 1\n}\nmem.bad = () => {\n  return nope.y\n}\nreturn \"ready\"\n",
		"tl/args.js":    "args.seen = true\nconst local = 1\nawait null\nreturn { args, n: (await img.stats()).meanL > 0 }\n",
		"tl/nest.js":    "const inner = await cv.load(\"tl/helpers.js\")\nreturn \"nested \" + inner\n",
		"tl/self.js":    "return await cv.load(\"tl/self.js\")\n",
		"tl/syntax.js":  "mem.a = 1\nconst x = )\n",
		"tl/fails.js":   "// a pipeline that fails after an await\nawait null\nthrow new Error(\"late boom\")\n",
		"tl/warn.js":    "const c = img.crop([-10, -10, 50, 50])\nreturn (await c).w\n",
		"tl/looks.js":   "await null\nmem.loaded = (await img.stats()).meanL\nimg.exposure(0.5).look(\"from file\")\nreturn 1\n",
		"shadow.js":     "return \"read\"\n",
		"twin.js":       "throw new Error(\"from the read directory\")\n",
		"a space.js":    "return 1\n",
	})
	writeFiles(t, ws, map[string]string{"shadow.js": "return \"workspace\"\n", "twin.js": "return 1\n"})
	opts.Image, opts.Root, opts.Read = img, ws, []string{filepath.Dir(img), lib}
	app, sid := newApp(t, opts)
	return app, sid, ws, lib
}

// TestLoadRunsScriptFiles: cv.load resolves like cv.open, runs the file as
// an async function body with args, returns its value and runs it again on
// every call.
func TestLoadRunsScriptFiles(t *testing.T) {
	app, sid, ws, lib := loadApp(t, cvsession.Options{})
	out := mustOK(t, eval(t, app, sid, `const r = await cv.load("tl/helpers.js"); return { r, f: mem.f(41) }`))
	mustContain(t, out, "r: ready", "f: 42")
	// args by reference, {} by default; top-level await; const stays local
	out = mustOK(t, eval(t, app, sid, `const a = { k: 1 }; const r = await cv.load("tl/args.js", a); const d = await cv.load("tl/args.js"); return { seen: a.seen, n: r.n, d: d.args, local: typeof local }`))
	mustContain(t, out, "seen: true", "n: true", "d: {seen: true}", "local: undefined")
	// the workspace shadows a read directory; an absolute path reaches the shadowed file
	mustContain(t, mustOK(t, eval(t, app, sid, `return await cv.load("shadow.js")`)), "workspace")
	abs := filepath.Join(lib, "shadow.js")
	mustContain(t, mustOK(t, eval(t, app, sid, `return await cv.load(`+jsQuote(abs)+`)`)), "read")
	// a second load re-reads an edited file
	writeFiles(t, ws, map[string]string{"shadow.js": "return \"edited\"\n"})
	mustContain(t, mustOK(t, eval(t, app, sid, `return await cv.load("shadow.js")`)), "edited")
	// nested loads; 40 parallel loads
	mustContain(t, mustOK(t, eval(t, app, sid, `return await cv.load("tl/nest.js")`)), "nested ready")
	mustContain(t, mustOK(t, eval(t, app, sid, `const all = await Promise.all(Array.from({ length: 40 }, () => cv.load("tl/helpers.js"))); return all.filter(x => x === "ready").length`)), "40")
}

// TestLoadErrors: bad names, missing files, files outside the roots, syntax
// errors with the file's line, runtime errors with where and called from,
// and the nesting limit.
func TestLoadErrors(t *testing.T) {
	app, sid, _, lib := loadApp(t, cvsession.Options{})
	for code, want := range map[string][]string{
		`return await cv.load("tl/helpers")`:                 {"code: E_ARG_TYPE", `fix: await cv.load("tl/helpers.js")`},
		`return await cv.load("a space.js")`:                 {"code: E_ARG_TYPE", "no spaces or parentheses"},
		`return await cv.load("tl/missing.js")`:              {"code: E_ARG_TYPE", `no such file "tl/missing.js" in the workspace`, "read directories", "cv.scripts()"},
		`return await cv.load("/etc/hosts.js")`:              {"code: E_ACCESS", "--read DIR"},
		`return await cv.load(42)`:                           {"code: E_ARG_TYPE", "needs a script path string"},
		`return await cv.load("tl/syntax.js")`:               {"code: SyntaxError", "message: 'tl/syntax.js:2:11: Unexpected token )'", `where: 'line 1: return await cv.load("tl/syntax.js")'`},
		`return await cv.load("tl/self.js")`:                 {"code: E_TOO_MANY", "cv.load is nested 32 deep: tl/self.js → tl/self.js", "does a file load itself?", "where: 'tl/self.js:1:"},
		"await cv.load(\"tl/helpers.js\")\nreturn mem.bad()": {"code: ReferenceError", "where: 'tl/helpers.js:6: return nope.y'", "called from: 'line 2: return mem.bad()'"},
		"const p = cv.load(\"tl/fails.js\")\nreturn await p": {"message: late boom", "where: 'tl/fails.js:3: throw new Error(\"late boom\")'"},
	} {
		out := text(eval(t, app, sid, code))
		mustContain(t, out, want...)
	}
	// a shadowed file reached by its absolute path has its own name
	mustContain(t, text(eval(t, app, sid, `return await cv.load(`+jsQuote(filepath.Join(lib, "twin.js"))+`)`)), "where: 'read2:twin.js:1: throw new Error(")
	// a function the file defined, called in a later eval
	mustContain(t, text(eval(t, app, sid, "const x = 1\nconst y = 2\nreturn mem.bad()")), "where: 'tl/helpers.js:6: return nope.y'", "called from: 'line 3: return mem.bad()'")
}

// TestLoadSettlesInTheEval: warnings in a file carry file:line, looks a file
// creates are delivered, an unawaited load finishes before the eval's
// result, and an unawaited load that fails is UNHANDLED with its location.
func TestLoadSettlesInTheEval(t *testing.T) {
	app, sid, _, _ := loadApp(t, cvsession.Options{})
	mustContain(t, mustOK(t, eval(t, app, sid, `return await cv.load("tl/warn.js")`)), "RANGE tl/warn.js:1:")
	resp := eval(t, app, sid, `cv.load("tl/looks.js"); return 1`)
	out := mustOK(t, resp)
	if len(resp.Cell.Execution.Content) != 1 {
		t.Fatalf("the unawaited load's look is not in this eval: %d looks\n%s", len(resp.Cell.Execution.Content), out)
	}
	mustContain(t, out, "loaded (number)")
	out = mustOK(t, eval(t, app, sid, `cv.load("tl/fails.js"); return 1`))
	mustContain(t, out, "UNHANDLED tl/fails.js:3: a promise was rejected and never handled: late boom. fix: await the call that returned it")
	mustNotContain(t, mustOK(t, eval(t, app, sid, `cv.load("tl/fails.js").catch(e => 0); return 1`)), "UNHANDLED")
	out = mustOK(t, eval(t, app, sid, `Promise.reject(new Error("boom")); return 1`))
	mustContain(t, out, "UNHANDLED line 1: a promise was rejected and never handled: boom.")
	mustNotContain(t, text(eval(t, app, sid, `return await Promise.reject(new Error("own"))`)), "UNHANDLED")
}

// TestScriptsListsLoadableFiles: cv.scripts lists the .js files with their
// first comment line, skips dot directories and node_modules, and omits
// shadowed files.
func TestScriptsListsLoadableFiles(t *testing.T) {
	app, sid, ws, _ := loadApp(t, cvsession.Options{})
	writeFiles(t, ws, map[string]string{".git/hook.js": "//x\n", "node_modules/m/index.js": "//x\n", "notes.txt": "x"})
	out := mustOK(t, eval(t, app, sid, `return cv.scripts()`))
	mustContain(t, out, "tl/helpers.js", "tl/helpers.js: helpers for tests", "a pipeline that fails after an await", "shadow.js")
	mustNotContain(t, out, "hook.js", "index.js", "notes.txt", "a space.js")
	if n := strings.Count(out, "shadow.js"); n != 1 {
		t.Fatalf("shadow.js listed %d times:\n%s", n, out)
	}
}

// TestPreludesAreLoads: a prelude is a cv.load at session start: it must lie
// in a root, its errors name the file and line, and later MCP sessions get
// it too.
func TestPreludesAreLoads(t *testing.T) {
	img := planted(t)
	lib := t.TempDir()
	writeFiles(t, lib, map[string]string{
		"helpers.js": "mem.double = x => 2 * x\nmem.meanL = async () => (await img.stats()).meanL\nconst s = await img.stats()\nmem.p99 = s.p99\nimg.look(\"discarded\")\nreturn \"helpers ready\"\n",
		"more.js":    "mem.quad = x => mem.double(mem.double(x))\n",
		"bad.js":     "mem.ok = 1\nconst x = nope.y\n",
		"syntax.js":  "const x = )\n",
	})
	a, b := filepath.Join(lib, "helpers.js"), filepath.Join(lib, "more.js")
	opts := cvsession.Options{Image: img, Read: []string{filepath.Dir(img), lib}, Preludes: []string{a, b}}
	app, sid := newApp(t, opts)
	out := mustOK(t, eval(t, app, sid, `return { q: mem.quad(3), m: (await mem.meanL()) > 0, p: mem.p99 > 0 }`))
	mustContain(t, out, "q: 12", "m: true", "p: true")
	mustNotContain(t, out, "helpers ready", "look 1")

	cs := connect(t, opts)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "create_session", Arguments: map[string]any{}})
	if err != nil || res.IsError {
		t.Fatalf("create_session: %v %+v", err, res)
	}
	txt, _, _ := splitResult(t, res)
	id := regexp.MustCompile(`session-[0-9a-f-]{36}`).FindString(txt)
	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "eval", Arguments: map[string]any{"code": "return mem.quad(5)", "sessionId": id}})
	if err != nil {
		t.Fatal(err)
	}
	if txt, _, _ := splitResult(t, res); !strings.HasPrefix(txt, "20") {
		t.Fatalf("a later MCP session has no prelude helpers:\n%s", txt)
	}

	for p, want := range map[string]string{
		filepath.Join(lib, "bad.js"):    "prelude bad.js:2: nope is not defined",
		filepath.Join(lib, "syntax.js"): "prelude syntax.js:1:11: Unexpected token )",
	} {
		failing, err := cvsession.New(context.Background(), testWorkspace(t, cvsession.Options{Image: img, Read: []string{filepath.Dir(img), lib}, Preludes: []string{a, p}}))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = failing.Close(context.Background()) })
		if _, err := failing.CreateSession(context.Background()); err == nil || err.Error() != want {
			t.Fatalf("failing prelude: %v, want %q", err, want)
		}
		if len(failing.Sessions()) != 0 {
			t.Fatalf("the failed session is listed: %v", failing.Sessions())
		}
	}
	// outside every root, or missing
	if _, err := cvsession.New(context.Background(), cvsession.Options{Root: t.TempDir(), Preludes: []string{a}}); err == nil || !strings.Contains(err.Error(), "add --read "+lib) {
		t.Fatalf("a prelude outside the roots: %v", err)
	}
	if _, err := cvsession.New(context.Background(), cvsession.Options{Root: lib, Preludes: []string{filepath.Join(lib, "nope.js")}}); err == nil {
		t.Fatal("a missing prelude file is accepted")
	}
}

func jsQuote(s string) string { return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"` }
