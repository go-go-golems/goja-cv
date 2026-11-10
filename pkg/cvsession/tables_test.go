package cvsession_test

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cvsession"
)

var updateResults = flag.Bool("update-results", false, "rewrite testdata/*.result files")

// TestTablesRenderFlatRecords: an array of two or more flat records is a
// literal block with the union of the keys, "-" for missing and null,
// quoted strings with whitespace, columns aligned by runes and no trailing
// spaces.
func TestTablesRenderFlatRecords(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	out := mustOK(t, eval(t, app, sid, `return [
  { id: 1, name: "a b", v: 0.123456, ok: true },
  { id: 22, v: null, extra: "é", ok: false },
  { id: 3, name: "x".repeat(70), v: undefined, extra: "" },
]`))
	want := "|-\n" +
		"  id  name                                                          v        ok     extra\n" +
		"  1   \"a b\"                                                         0.12346  true   -\n" +
		"  22  -                                                             -        false  é\n" +
		"  3   xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx…  -        -      \"\"\n"
	if !strings.HasPrefix(out, want) {
		t.Fatalf("table:\n%s\nwant:\n%s", out, want)
	}
	for _, l := range strings.Split(out, "\n") {
		if strings.TrimRight(l, " ") != l {
			t.Fatalf("trailing spaces in %q", l)
		}
	}
	// not tables: one record, arrays of arrays or numbers, a record with an
	// object or an accessor
	for code, want := range map[string]string{
		`return [{ a: 1 }]`:                                                "- {a: 1}",
		`return [[1, 2], [3, 4]]`:                                          "- [1, 2]",
		`return [{ a: 1 }, { a: { b: 2 } }]`:                               "- {a: 1}",
		`return [{ a: 1 }, { get a() { return 2 } }]`:                      "a: '[Getter]'",
		`return [{ a: 1 }, Object.assign(Object.create({}), { a: 2 }), 3]`: "- 3",
	} {
		mustContain(t, mustOK(t, eval(t, app, sid, code)), want)
	}
}

// TestTablesAreExemptFromTheCaps: a 200-row table is not cut at 48 rows or
// at 2,000 characters; over the budget it shows at least five rows and a
// paging note with its key path ("this array" at the top); tables of one
// result share the budget.
func TestTablesAreExemptFromTheCaps(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	out := mustOK(t, eval(t, app, sid, `return { rows: Array.from({ length: 200 }, (_, i) => ({ id: i, s: "ab" })) }`))
	if !strings.Contains(out, "\n  199  ab") || strings.Contains(out, "more (save all") || strings.Contains(out, "shown. Next") {
		t.Fatalf("200 short rows were cut:\n%s", out)
	}
	out = mustOK(t, eval(t, app, sid, `return { hot: { rows: Array.from({ length: 500 }, (_, i) => ({ id: i, cx: i * 1.25, cy: 2 * i, snr: i / 7 })) } }`))
	m := regexp.MustCompile(`… rows 1–(\d+) of 500 shown\. Next: hot\.rows\.slice\((\d+), (\d+)\); all: await cv\.save\("rows\.csv", hot\.rows\)`).FindStringSubmatch(out)
	if m == nil || m[1] != m[2] || len(out) > 9000 {
		t.Fatalf("paging note (%d characters):\n%s", len(out), out)
	}
	out = mustOK(t, eval(t, app, sid, `return Array.from({ length: 900 }, (_, i) => ({ id: i, v: i }))`))
	mustContain(t, out, "Next: this array.slice(", `await cv.save("rows.csv", this array)`)
	// two tables share the budget; the second still shows five rows
	out = mustOK(t, eval(t, app, sid, `const t = n => Array.from({ length: n }, (_, i) => ({ id: i, text: "x".repeat(50) }))
return { a: t(400), b: t(400) }`))
	mustContain(t, out, "Next: a.slice(", "Next: b.slice(5, 10)")
	if len(out) > 9000 || strings.Contains(out, "# CUT") {
		t.Fatalf("two tables overran the result (%d):\n%s", len(out), out)
	}
}

// TestResultFormat pins the text of a representative result: detector
// items, measured items and a nested object with two tables, measured peaks
// (testdata/representative.result; -update-results rewrites it).
func TestResultFormat(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: planted(t)})
	out := mustOK(t, eval(t, app, sid, `const sp = await img.detect.specks()
const top = sp.sort((a, b) => b.contrast - a.contrast).slice(0, 6)
const m = await img.as("linear").measure(top.map(r => ({ cx: r.cx, cy: r.cy, r: 2, label: "s" + r.id })))
const lin = img.as("linear")
const pk = await lin.peaks({ radius: [2, 4], polarity: "both" })
const peaks = await lin.view("speck").measure(pk.slice(0, 6), { prefix: "sp" })
const specks = top.map(({ localNoise, ...r }) => r)   // localNoise differs in the 5th digit between amd64 and arm64
return { n: sp.length, specks, measured: { rows: m, cols: Object.keys(m[0]).length }, peaks }`))
	golden := filepath.Join("testdata", "representative.result")
	if *updateResults {
		if err := os.WriteFile(golden, []byte(out), 0o644); err != nil { // #nosec G306 -- test golden
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if out != string(want) {
		t.Fatalf("result changed (go test -run TestResultFormat -update-results):\n%s", out)
	}
}

// TestPeaksRenderAsAPagedTable: awaited peaks returned bare are a table;
// a long list is paged with a note that names the array.
func TestPeaksRenderAsAPagedTable(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: planted(t)})
	out := mustOK(t, eval(t, app, sid, `return await img.as("linear").peaks({ snr: 0, polarity: "both" })`))
	m := regexp.MustCompile(`(?m)^\|-\n  id +x +y +w +h +cx +cy +area +aspect +r +coreMean +ringMean +step +spread +snr +ringCoverage$`).FindString(out)
	note := regexp.MustCompile(`… rows 1–(\d+) of (\d+) shown\. Next: this array\.slice\((\d+), (\d+)\); all: await cv\.save\("rows\.csv", this array\)`).FindStringSubmatch(out)
	if m == "" || note == nil || note[1] != note[3] || len(out) > 9000 {
		t.Fatalf("peaks (%d characters):\n%s", len(out), out)
	}
}

// TestFunctionDocsInTheContextBlock: the context block lists mem functions
// with short signatures and gives a function's doc line when its name is
// new or its source changed (not after an unchanged reload); a getter on mem
// is not run; a function in a result renders as its signature and doc line.
func TestFunctionDocsInTheContextBlock(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	def := `mem.f = (a, b = 2) => { // add b
  return a + b
}
mem.loooooooooooooooong = ({ alpha = 1, beta = [2, 3, 4] } = {}) => alpha
mem.n = 3
return 1`
	out := mustOK(t, eval(t, app, sid, def))
	mustContain(t, out, "mem: n (number)", "functions: mem.f(a, b = 2), mem.loooooooooooooooong({ alpha = 1, be…",
		"new functions: |-\n    mem.f(a, b = 2): add b\n    mem.loooooooooooooooong({ alpha = 1, beta = [2, 3, 4] } = {})")
	out = mustOK(t, eval(t, app, sid, def)) // the same source again: nothing new
	mustContain(t, out, "functions: mem.f(a, b = 2)")
	mustNotContain(t, out, "new functions")
	out = mustOK(t, eval(t, app, sid, "mem.f = (a, b = 3) => { // add b, now 3\n  return a + b\n}\nreturn 1"))
	mustContain(t, out, "new functions: |-\n    mem.f(a, b = 3): add b, now 3")
	mustNotContain(t, out, "mem.loooooooooooooooong({ alpha = 1, beta = [2, 3, 4] } = {})")
	// a getter on mem is not run
	out = mustOK(t, eval(t, app, sid, `let runs = 0; Object.defineProperty(mem, "g", { get() { runs++; return 1 }, enumerable: true }); mem.runs = () => runs; return 1`))
	mustContain(t, out, "g (getter)")
	mustContain(t, mustOK(t, eval(t, app, sid, `return mem.runs()`)), "0\n")
	// functions in results
	out = mustOK(t, eval(t, app, sid, `return { f: mem.f, alone: [mem.f, Math.max] }`))
	mustContain(t, out, "f: 'f(a, b = 3): add b, now 3'", "mem.f(a, b = 3): add b, now 3", "max()")
}

// TestPreludeFunctionsAreNewInTheFirstEval: a prelude does not mark its
// functions as shown: the first agent eval lists their docs.
func TestPreludeFunctionsAreNewInTheFirstEval(t *testing.T) {
	lib := t.TempDir()
	writeFiles(t, lib, map[string]string{"lib.js": "mem.f = x => { // add one\n  return x + 1\n}\n"})
	app, sid := newApp(t, cvsession.Options{Image: tiffPath, Read: []string{filepath.Dir(tiffPath), lib}, Preludes: []string{filepath.Join(lib, "lib.js")}})
	mustContain(t, mustOK(t, eval(t, app, sid, `return 1`)), "new functions: |-\n    mem.f(x): add one")
	mustNotContain(t, mustOK(t, eval(t, app, sid, `return await cv.load("lib.js")`)), "new functions")
}

// TestUnhandledRejections: a rejection nobody handled is one UNHANDLED
// warning; a caught one and the eval's own failure are not.
func TestUnhandledRejections(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	mustContain(t, mustOK(t, eval(t, app, sid, "const x = 1\nPromise.reject(new Error(\"x\"))\nreturn 1")), "UNHANDLED line 2: a promise was rejected and never handled: x. fix: await the call that returned it")
	mustNotContain(t, mustOK(t, eval(t, app, sid, `Promise.reject(new Error("x")).catch(() => 0); return 1`)), "UNHANDLED")
	for _, code := range []string{`throw new Error("own")`, `return Promise.reject(new Error("own"))`, `await Promise.reject(new Error("own"))`} {
		out := text(eval(t, app, sid, code))
		mustContain(t, out, "message: own")
		mustNotContain(t, out, "UNHANDLED")
	}
}
