package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
	code, out, errOut := runCLI(t, "eval", "--image", tiffPath, "--out", dir, "-e",
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
	for _, s := range []string{"meanL: 0.29507", "after: '[look 1: after — open(gradient-64x48-16bit.tif) → exposure(0.5) (64×48, 1 image px = 1.0 doc px)]'", "files:"} {
		if !strings.Contains(out, s) {
			t.Fatalf("output lacks %q:\n%s", s, out)
		}
	}
}

func TestRunScriptJSONAndExitCodes(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "a.js")
	if err := os.WriteFile(script, []byte("show(img.exposure(-1), \"dark\")\nconsole.log(\"hi\")\nreturn 42\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := runCLI(t, "run", script, "--image", tiffPath, "--out", filepath.Join(dir, "looks"), "--json")
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
	code, out, _ = runCLI(t, "eval", "--image", tiffPath, "--out", dir, "-e", "show(img, \"before\")\nreturn img.brightness(1)")
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
		"remote http":   {"mcp", "--http", "0.0.0.0:3999"},
		"bad root":      {"mcp", "--root", filepath.Join(dir, "nope")},
		"bad out-root":  {"eval", "--out-root", filepath.Join(dir, "nope"), "-e", "return 1"},
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
	srv, closeFn, err := newMCPServer(ctx, &rootOptions{stderr: &errOut}, &mcpFlags{image: tiffPath, roots: []string{filepath.Dir(tiffPath)}})
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
	if n != 0 || !strings.Contains(text, "meanL: 0.29507") {
		t.Fatalf("stats: %d images\n%s", n, text)
	}
	text, n = call("const fixed = img.levels({ black: 0.0, white: 0.74 }).exposure(0.35)\nshow(cv.compare([img, fixed], { mode: \"side\" }), { label: \"before / after\" })\nshow(fixed.histogram())\nreturn await fixed.stats().pick(\"meanL\", \"clipLow\", \"clipHigh\")")
	if n != 2 || !strings.Contains(text, "look 1: before / after") || !strings.Contains(text, "meanL:") {
		t.Fatalf("correction: %d images\n%s", n, text)
	}
	text, _ = call(`img = cv.open("/etc/hosts")`)
	if !strings.Contains(text, "code: E_ACCESS") {
		t.Fatalf("roots: %s", text)
	}
}

// TestDataFilesFromEval: data items are written as <name>.<ext> next to the
// looks, a second item of the same name gets -2, --json lists them under
// files with their row counts, and save writes under --out-root.
func TestDataFilesFromEval(t *testing.T) {
	dir := t.TempDir()
	out, root := filepath.Join(dir, "out"), filepath.Join(dir, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, errOut := runCLI(t, "eval", "--image", tiffPath, "--out", out, "--out-root", root, "--json", "-e", `
const regs = img.mask.luminance({ above: 0.5 }).components()
const saved = await cv.csv(regs).save("tables/regs.csv")
return {
  look: img.look("scene"),
  spots: cv.csv(regs, { name: "spots" }),
  again: cv.csv(regs.top(1), { name: "spots" }),
  raw: cv.json([{ b: 1, a: 2 }, { b: 3, a: 4 }], { name: "probe" }),
  anon: cv.json({ n: 1 }),
  saved: saved.rows,
}`)
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, stdout, errOut)
	}
	var res jsonResult
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout)
	}
	byName := map[string]contentFile{}
	for _, f := range res.Files {
		byName[filepath.Base(f.Path)] = f
	}
	for _, name := range []string{"look-1-scene.png", "spots.csv", "spots-2.csv", "probe.json", "data-5.json"} {
		if _, ok := byName[name]; !ok {
			t.Fatalf("no %s in %+v\n%s", name, res.Files, res.Result)
		}
	}
	if r := byName["probe.json"].Rows; r == nil || *r != 2 {
		t.Fatalf("probe rows %v", r)
	}
	if r := byName["spots-2.csv"].Rows; r == nil || *r != 1 {
		t.Fatalf("spots-2 rows %v", r)
	}
	probe, _ := os.ReadFile(byName["probe.json"].Path)
	if string(probe) != `[{"b":1,"a":2},{"b":3,"a":4}]`+"\n" {
		t.Fatalf("probe.json keeps key order: %s", probe)
	}
	csvData, _ := os.ReadFile(byName["spots.csv"].Path)
	if !strings.HasPrefix(string(csvData), "id,x,y,w,h,cx,cy,area,aspect\n") {
		t.Fatalf("spots.csv header: %s", csvData)
	}
	saved, err := os.ReadFile(filepath.Join(root, "tables", "regs.csv"))
	if err != nil || string(saved) != string(csvData) {
		t.Fatalf("saved csv: %v\n%s", err, saved)
	}
	for _, s := range []string{"spots: '[content 2: data spots.csv (9 columns of components)]'", "raw: '[content 4: data probe.json (2 items)]'", "anon: '[content 5: data data-5.json (1 keys)]'"} {
		if !strings.Contains(res.Result, s) {
			t.Fatalf("result lacks %q:\n%s", s, res.Result)
		}
	}
}
