package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var tiffPath, _ = filepath.Abs(filepath.Join("..", "..", "testdata", "gradient-64x48-16bit.tif"))

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := execute(context.Background(), args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestEvalWritesLookFiles(t *testing.T) {
	dir := t.TempDir()
	code, out, errOut := runCLI(t, "eval", "--read", filepath.Dir(tiffPath), "--image", tiffPath, "--out", dir, "-e",
		`return { meanL: (await img.stats()).meanL, after: img.exposure(0.5).look("after"), gray: img.toGray().look("gray") }`)
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	for _, f := range []string{"look-1-after.png", "look-2-gray.png"} {
		p := filepath.Join(dir, f)
		st, err := os.Stat(p)
		if err != nil || st.Size() == 0 {
			t.Fatalf("look file %s: %v", p, err)
		}
		if !strings.Contains(out, p) {
			t.Fatalf("output does not name %s:\n%s", p, out)
		}
	}
	for _, s := range []string{"meanL: 0.52726", "after: '[look 1: after — open(gradient-64x48-16bit.tif) → exposure(0.5) (64×48, 1 image px = 1.0 doc px)]'", "files:"} {
		if !strings.Contains(out, s) {
			t.Fatalf("output lacks %q:\n%s", s, out)
		}
	}
}

// TestEvalWarnsAboutLooksBeyondTheBudget: looks beyond the four of an eval
// are not written and the LOOKS warning names them.
func TestEvalWarnsAboutLooksBeyondTheBudget(t *testing.T) {
	dir := t.TempDir()
	code, out, errOut := runCLI(t, "eval", "--read", filepath.Dir(tiffPath), "--image", tiffPath, "--out", dir, "-e",
		`for (const i of [1, 2, 3, 4, 5, 6]) img.exposure(i / 10).look("s" + i); return 1`)
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	for _, s := range []string{"warnings:", `LOOKS line 1: 2 looks were not delivered (at most 4 per eval): line 1 look("s5"), line 1 look("s6"). Split the work over several calls, or return fewer looks`} {
		if !strings.Contains(out, s) {
			t.Fatalf("output lacks %q:\n%s", s, out)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "look-4-s4.png")); err != nil {
		t.Fatalf("fourth look not written: %v", err)
	}
	if m, _ := filepath.Glob(filepath.Join(dir, "*s5*")); len(m) > 0 {
		t.Fatalf("a dropped look was written: %v", m)
	}
}

func TestRunScriptJSONAndExitCodes(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "a.js")
	if err := os.WriteFile(script, []byte("img.exposure(-1).look(\"dark\")\nconsole.log(\"hi\")\nreturn 42\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := runCLI(t, "run", script, "--read", filepath.Dir(tiffPath), "--image", tiffPath, "--out", filepath.Join(dir, "looks"), "--json")
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, out, errOut)
	}
	var res jsonResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	if res.Status != "ok" || !strings.HasPrefix(res.Result, "42") || len(res.Files) != 1 || len(res.Console) != 1 {
		t.Fatalf("result %+v", res)
	}
	if _, err := os.Stat(res.Files[0].Path); err != nil || filepath.Base(res.Files[0].Path) != "look-1-dark.png" {
		t.Fatalf("look %+v: %v", res.Files[0], err)
	}

	// cv error: printed, exit 1; looks shown before the throw are still written
	code, out, _ = runCLI(t, "eval", "--read", filepath.Dir(tiffPath), "--image", tiffPath, "--out", dir, "-e", "img.look(\"before\")\nreturn img.brightness(1)")
	if code != exitCVError || !strings.Contains(out, "code: E_UNKNOWN_METHOD") || !strings.Contains(out, "look-1-before.png") {
		t.Fatalf("cv error: exit %d\n%s", code, out)
	}

	for name, args := range map[string][]string{
		"no code":       {"eval"},
		"missing image": {"eval", "--image", filepath.Join(dir, "nope.tif"), "-e", "return 1"},
		"unknown flag":  {"eval", "--frob", "-e", "return 1"},
		"no script":     {"run"},
		"bad script":    {"run", filepath.Join(dir, "nope.js")},
		"unknown cmd":   {"frob"},
		"bad kind":      {"ops", "--kind", "Foo"},
		"remote http":   {"mcp", "--http", "0.0.0.0:3999", "--root", dir},
		"bad root":      {"mcp", "--root", filepath.Join(dir, "nope")},
		"mcp no root":   {"mcp"},
		"bad read":      {"eval", "--read", filepath.Join(dir, "nope"), "-e", "return 1"},
		"image outside": {"eval", "--root", dir, "--image", tiffPath, "-e", "return 1"},
	} {
		if code, out, errOut := runCLI(t, args...); code != exitUsage {
			t.Fatalf("%s: exit %d, want 2\n%s\n%s", name, code, out, errOut)
		}
	}
}

func TestOpsAndHelp(t *testing.T) {
	code, out, _ := runCLI(t, "ops", "--json")
	var ops []opInfo
	if code != exitOK || json.Unmarshal([]byte(out), &ops) != nil || len(ops) < 10 {
		t.Fatalf("ops --json: exit %d\n%s", code, out)
	}
	code, out, _ = runCLI(t, "ops", "--kind", "Image")
	if code != exitOK || !strings.Contains(out, "exposure(ev)") || strings.Contains(out, "\nValue ") {
		t.Fatalf("ops --kind Image: exit %d\n%s", code, out)
	}
	code, out, _ = runCLI(t, "help", "exposure")
	if code != exitOK || !strings.Contains(out, "exposure:\n  args:") {
		t.Fatalf("help exposure: exit %d\n%s", code, out)
	}
	code, out, _ = runCLI(t, "help", "blur")
	if code != exitCVError || !strings.Contains(out, `hint: Did you mean "gaussian"?`) {
		t.Fatalf("help blur: exit %d\n%s", code, out)
	}
	code, out, _ = runCLI(t, "help", "eval")
	if code != exitOK || !strings.Contains(out, "eval -e CODE") {
		t.Fatalf("help eval: exit %d\n%s", code, out)
	}
}

// TestMCPCommandServer drives the server object goja-cv mcp serves through an
// in-memory client: the Part 1 Session A flow.
func TestMCPCommandServer(t *testing.T) {
	ctx := context.Background()
	var errOut bytes.Buffer
	srv, closeFn, err := newMCPServer(ctx, &rootOptions{stderr: &errOut}, &mcpFlags{image: tiffPath, root: t.TempDir(), read: []string{filepath.Dir(tiffPath)}})
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := srv.Server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ss.Close() }()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "t", Version: "t"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cs.Close() }()
	call := func(code string) (string, int) {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "eval", Arguments: map[string]any{"code": code}})
		if err != nil {
			t.Fatal(err)
		}
		text, images := "", 0
		for _, c := range res.Content {
			switch x := c.(type) {
			case *mcp.TextContent:
				text += x.Text
			case *mcp.ImageContent:
				images++
			}
		}
		return text, images
	}
	text, n := call("const s = await img.stats()\nreturn { meanL: s.meanL, black: s.clipLow, white: s.clipHigh, p01: s.p01, p99: s.p99 }")
	if n != 0 || !strings.Contains(text, "meanL: 0.52726") {
		t.Fatalf("stats: %d images\n%s", n, text)
	}
	text, n = call("const fixed = img.levels({ black: 0.0, white: 0.74 }).exposure(0.35)\ncv.compare([img, fixed], { mode: \"side\" }).look(\"before / after\")\nfixed.histogram().look()\nconst { meanL, clipLow, clipHigh } = await fixed.stats()\nreturn { meanL, clipLow, clipHigh }")
	if n != 2 || !strings.Contains(text, "look 1: before / after") || !strings.Contains(text, "meanL:") {
		t.Fatalf("correction: %d images\n%s", n, text)
	}
	text, _ = call(`img = cv.open("/etc/hosts")`)
	if !strings.Contains(text, "code: E_ACCESS") {
		t.Fatalf("roots: %s", text)
	}
}

// TestSaveFromEval: cv.save writes in --root (relative names), the looks go
// to --out, a --read directory is never written, and --json lists the look
// files.
func TestSaveFromEval(t *testing.T) {
	dir := t.TempDir()
	out, root := filepath.Join(dir, "out"), filepath.Join(dir, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, errOut := runCLI(t, "eval", "--read", filepath.Dir(tiffPath), "--image", tiffPath, "--out", out, "--root", root, "--json", "-e", `
const regs = img.mask.luminance({ above: 0.5 }).components()
const t = await cv.save("tables/regs.csv", regs)
const j = await cv.save("probe.json", [{ b: 1, a: 2 }, { b: 3, a: 4 }])
const v = await cv.save("scene.tif", img)
return { look: img.look("scene"), rows: t.rows, j: j.rows, again: (await cv.open(v.path)).w }`)
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, stdout, errOut)
	}
	var res jsonResult
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout)
	}
	if len(res.Files) != 1 || filepath.Base(res.Files[0].Path) != "look-1-scene.png" || filepath.Dir(res.Files[0].Path) != out {
		t.Fatalf("files %+v", res.Files)
	}
	probe, _ := os.ReadFile(filepath.Join(root, "probe.json"))
	if string(probe) != `[{"b":1,"a":2},{"b":3,"a":4}]`+"\n" {
		t.Fatalf("probe.json keeps key order: %s", probe)
	}
	csvData, err := os.ReadFile(filepath.Join(root, "tables", "regs.csv"))
	if err != nil || !strings.HasPrefix(string(csvData), "id,x,y,w,h,cx,cy,area,aspect\n") {
		t.Fatalf("regs.csv: %v\n%s", err, csvData)
	}
	if !strings.Contains(res.Result, "again: 64") {
		t.Fatalf("result:\n%s", res.Result)
	}
	// relative names resolve in the workspace; a read directory is not written
	code, _, _ = runCLI(t, "eval", "--read", filepath.Dir(tiffPath), "--root", root, "-e", `return await cv.save(`+strconv.Quote(filepath.Join(filepath.Dir(tiffPath), "x.json"))+`, 1)`)
	if code != exitCVError {
		t.Fatalf("save into a read directory: exit %d", code)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(tiffPath), "x.json")); err == nil {
		t.Fatal("wrote into a read directory")
	}
}

// TestPreludeFlag: --prelude scripts run before the code; a failing one is
// a cv error (exit 1) naming its file and line; a missing one a usage error.
func TestPreludeFlag(t *testing.T) {
	dir := t.TempDir()
	good, bad := filepath.Join(dir, "h.js"), filepath.Join(dir, "bad.js")
	if err := os.WriteFile(good, []byte("mem.f = x => x + 1\nreturn 'ready'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("\n\nnope()\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, out, _ := runCLI(t, "eval", "--prelude", good, "-e", "return mem.f(41)"); code != exitOK || !strings.HasPrefix(out, "42") {
		t.Fatalf("prelude: exit %d\n%s", code, out)
	}
	if code, out, errOut := runCLI(t, "eval", "--prelude", good, "--prelude", bad, "-e", "return 1"); code != exitCVError || !strings.Contains(out+errOut, "prelude bad.js:3:") {
		t.Fatalf("failing prelude: exit %d\n%s\n%s", code, out, errOut)
	}
	if code, _, _ := runCLI(t, "eval", "--prelude", filepath.Join(dir, "nope.js"), "-e", "return 1"); code != exitUsage {
		t.Fatalf("missing prelude: exit %d", code)
	}
}
