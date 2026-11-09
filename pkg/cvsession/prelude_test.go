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

func writeScript(t *testing.T, name, code string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(code), 0o644); err != nil { // #nosec G306 -- test script
		t.Fatal(err)
	}
	return p
}

// TestPreludesRunInEverySession: preludes define mem helpers (they may
// await), run in order in every session (also MCP sessions created later),
// and a failing one names its file and line.
func TestPreludesRunInEverySession(t *testing.T) {
	img := planted(t)
	a := writeScript(t, "helpers.js", "mem.double = x => 2 * x\nmem.meanL = async () => (await img.stats()).meanL\nconst s = await img.stats()\nmem.p99 = s.p99\nreturn \"helpers ready\"\n")
	b := writeScript(t, "more.js", "mem.quad = x => mem.double(mem.double(x))\n")
	app, sid := newApp(t, cvsession.Options{Image: img, Preludes: []string{a, b}})
	out := mustOK(t, eval(t, app, sid, `return { q: mem.quad(3), m: (await mem.meanL()) > 0, p: mem.p99 > 0 }`))
	mustContain(t, out, "q: 12", "m: true", "p: true", "mem: double (function), meanL (function), p99 (number), quad (function)")
	mustNotContain(t, out, "helpers ready")

	cs := connect(t, cvsession.Options{Image: img, Preludes: []string{a, b}})
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "create_session", Arguments: map[string]any{}})
	if err != nil || res.IsError {
		t.Fatalf("create_session: %v %+v", err, res)
	}
	text, _, _ := splitResult(t, res)
	id := regexp.MustCompile(`session-[0-9a-f-]{36}`).FindString(text)
	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "eval", Arguments: map[string]any{"code": "return mem.quad(5)", "sessionId": id}})
	if err != nil {
		t.Fatal(err)
	}
	if text, _, _ := splitResult(t, res); !strings.HasPrefix(text, "20") {
		t.Fatalf("a later MCP session has no prelude helpers:\n%s", text)
	}

	bad := writeScript(t, "bad.js", "mem.ok = 1\nconst x = nope.y\n")
	failing, err := cvsession.New(context.Background(), cvsession.Options{Image: img, Root: t.TempDir(), Read: []string{filepath.Dir(img)}, Preludes: []string{a, bad}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = failing.Close(context.Background()) })
	if _, err := failing.CreateSession(context.Background()); err == nil || err.Error() != "prelude bad.js:2: nope is not defined" {
		t.Fatalf("failing prelude: %v", err)
	}
	if len(failing.Sessions()) != 0 {
		t.Fatalf("the failed session is listed: %v", failing.Sessions())
	}
	if _, err := cvsession.New(context.Background(), cvsession.Options{Preludes: []string{filepath.Join(t.TempDir(), "nope.js")}}); err == nil {
		t.Fatal("a missing prelude file is accepted")
	}
}
