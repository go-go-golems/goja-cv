package cvsession_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/go-go-golems/go-go-goja/pkg/replsession"
	"github.com/go-go-golems/goja-cv/pkg/cv/docs"
	"github.com/go-go-golems/goja-cv/pkg/cvsession"
	guide "github.com/go-go-golems/goja-cv/pkg/doc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func connect(t *testing.T, opts cvsession.Options) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	app, err := cvsession.New(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Close(context.Background()) })
	srv, err := cvsession.NewMCPServer(ctx, app)
	if err != nil {
		t.Fatal(err)
	}
	st, ct := mcp.NewInMemoryTransports()
	ss, err := srv.Server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "cv-test", Version: "test"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func callEval(t *testing.T, cs *mcp.ClientSession, code string) (string, []*mcp.ImageContent, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "eval", Arguments: map[string]any{"code": code}})
	if err != nil {
		t.Fatalf("call eval: %v", err)
	}
	return splitResult(t, res)
}

func splitResult(t *testing.T, res *mcp.CallToolResult) (string, []*mcp.ImageContent, bool) {
	t.Helper()
	var text strings.Builder
	var images []*mcp.ImageContent
	for _, c := range res.Content {
		switch x := c.(type) {
		case *mcp.TextContent:
			text.WriteString(x.Text)
		case *mcp.ImageContent:
			images = append(images, x)
		default:
			t.Fatalf("unexpected content %T", c)
		}
	}
	return text.String(), images, res.IsError
}

func TestMCPServerToolsResourcesPrompt(t *testing.T) {
	cs := connect(t, cvsession.Options{Image: tiffPath})
	ctx := context.Background()
	if got := cs.InitializeResult().Instructions; !strings.Contains(got, "body of an async function") {
		t.Fatalf("instructions %q", got)
	}
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, tool := range tools.Tools {
		names[tool.Name] = tool.Description
	}
	for _, n := range []string{"eval", "open", "create_session", "list_sessions", "history", "bindings"} {
		if _, ok := names[n]; !ok {
			t.Fatalf("missing tool %s in %v", n, names)
		}
	}
	if !strings.Contains(names["eval"], "body of an async function") {
		t.Fatalf("eval description %q", names["eval"])
	}
	res, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: cvsession.ReferenceURI})
	if err != nil {
		t.Fatal(err)
	}
	ref := res.Contents[0].Text
	for _, n := range []string{"## Image methods", "`exposure(ev)` → Image", "`levels(black, white, gamma, auto)`", "`cv.open(path, { encoding, box, origin, as })`"} {
		if !strings.Contains(ref, n) {
			t.Fatalf("reference lacks %q:\n%s", n, ref)
		}
	}
	p, err := cs.GetPrompt(ctx, &mcp.GetPromptParams{Name: "cv-agent"})
	if err != nil || len(p.Messages) != 1 {
		t.Fatalf("prompt: %v %+v", err, p)
	}

	text, images, isErr := callEval(t, cs, `return { stats: await img.stats(), after: img.exposure(0.5).look("after") }`)
	t.Logf("eval result:\n%s", text)
	if isErr || len(images) != 1 || images[0].MIMEType != "image/png" || len(images[0].Data) < 100 {
		t.Fatalf("isErr=%v images=%d\n%s", isErr, len(images), text)
	}
	mustContain(t, text, "after: '[look 1: after — open(gradient-64x48-16bit.tif) → exposure(0.5) (64×48, 1 image px = 1.0 doc px)]'",
		"content 1 look 1: after — open(gradient-64x48-16bit.tif) → exposure(0.5) (64×48, 1 image px = 1.0 doc px): attached (image/png")

	text, images, isErr = callEval(t, cs, "show(img, \"before\")\nreturn img.blur(2)")
	if !isErr || len(images) != 1 {
		t.Fatalf("error eval: isErr=%v images=%d\n%s", isErr, len(images), text)
	}
	mustContain(t, text, "code: E_UNKNOWN_METHOD", "where: 'line 2: return img.blur(2)'", "fix: img.gaussian(…)")
}

func TestMCPOpenTool(t *testing.T) {
	cs := connect(t, cvsession.Options{})
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "open", Arguments: map[string]any{"path": pngPath}})
	if err != nil {
		t.Fatal(err)
	}
	text, images, isErr := splitResult(t, res)
	if isErr || len(images) != 1 {
		t.Fatalf("open: isErr=%v images=%d\n%s", isErr, len(images), text)
	}
	mustContain(t, text, "kind: Image", "look 1: opened", "images: img = gradient-64x48-8bit.png (64×48, 8-bit PNG, srgb)")
	text, _, _ = callEval(t, cs, `return (await img.stats()).w`)
	mustContain(t, text, "64")
}

// TestSessionA is Part 1's Session A ("Is this scan exposed well?") through
// the MCP server: stats, then a candidate correction with looks before and
// after and a histogram, then the corrected stats.
func TestSessionA(t *testing.T) {
	cs := connect(t, cvsession.Options{Image: tiffPath})
	text, images, isErr := callEval(t, cs, `const s = await img.stats()
return {
  meanL: s.meanL,
  black: s.clipLow,
  white: s.clipHigh,
  p01: s.p01, p99: s.p99,
}`)
	if isErr || len(images) != 0 {
		t.Fatalf("stats: %v %d\n%s", isErr, len(images), text)
	}
	mustContain(t, text, "meanL: 0.29507", "black: 0.036133", "p99: 0.83255")
	mustNotContain(t, text, "warnings:")

	// Part 1 verbatim: cv.compare landed with M3.
	text, images, isErr = callEval(t, cs, `const fixed = img.levels({ black: 0.0, white: 0.74 }).exposure(0.35)
show(cv.compare([img, fixed], { mode: "side" }), { label: "before / after" })
show(fixed.histogram())
return await fixed.stats().pick("meanL", "clipLow", "clipHigh")`)
	t.Logf("session A step 2:\n%s", text)
	if isErr || len(images) != 2 {
		t.Fatalf("correction: isErr=%v images=%d\n%s", isErr, len(images), text)
	}
	mustContain(t, text, "meanL: 0.50822", "clipLow:", "clipHigh:",
		"shown:", "look 1: before / after — compare([2 images]) (132×48, 1 image px = 1.0 doc px)", "look 2: open(gradient-64x48-16bit.tif) → levels → exposure(0.35) → histogram (512×204 chart)")
	if images[0].MIMEType != "image/png" || images[1].MIMEType != "image/png" {
		t.Fatalf("mimes %s %s", images[0].MIMEType, images[1].MIMEType)
	}
	mustNotContain(t, text, "UNUSED_NODE", "error")
}

func TestMCPInstructionsAreCompactAndDocsAreResources(t *testing.T) {
	cs := connect(t, cvsession.Options{Image: tiffPath})
	ctx := context.Background()
	ins := cs.InitializeResult().Instructions
	if len(ins) > 4000 {
		t.Fatalf("instructions are %d characters, want < 4000", len(ins))
	}
	for _, s := range []string{"body of an async function", "mem", "node.look(", "show(", "doc px", "fix",
		"img.stats()", "img.stats({ box:", "img.levels(", "cv.compare(", "img.mask.luminance(", "img.within(", "img.detect.specks()", "img.heal(",
		"P.slider(", "cv.docs.search(", "cv.docs.read(", "cv.help(op)", "cv://docs/<id>", "cv://reference", "cv://guide/<slug>", "not available yet"} {
		if !strings.Contains(ins, s) {
			t.Fatalf("instructions lack %q", s)
		}
	}
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name == "eval" && !strings.Contains(tool.Description, "cv.docs.search") {
			t.Fatalf("eval description %q", tool.Description)
		}
	}
	res, err := cs.ListResources(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	uris := map[string]bool{}
	for _, r := range res.Resources {
		uris[r.URI] = true
	}
	for _, id := range docs.IDs() {
		if !uris[cvsession.DocsURIPrefix+id] {
			t.Fatalf("missing resource for topic %s in %v", id, uris)
		}
	}
	if !uris[cvsession.ReferenceURI] {
		t.Fatal("missing cv://reference")
	}
	pages, err := guide.Pages()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pages {
		if !uris[cvsession.GuideURIPrefix+p.Slug] {
			t.Fatalf("missing guide resource %s", p.Slug)
		}
	}
	gr, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "cv://guide/setup-pi"})
	if err != nil || !strings.HasPrefix(gr.Contents[0].Text, "# Setting up goja-cv in the pi coding agent") || !strings.Contains(gr.Contents[0].Text, "directTools") {
		t.Fatalf("read cv://guide/setup-pi: %v", err)
	}
	rr, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "cv://docs/recipes"})
	if err != nil || !strings.HasPrefix(rr.Contents[0].Text, "# Recipes") || rr.Contents[0].MIMEType != "text/markdown" {
		t.Fatalf("read cv://docs/recipes: %v %+v", err, rr)
	}
	text, _, isErr := callEval(t, cs, `return cv.docs.read("getting-started", { section: "The globals" })`)
	if isErr || !strings.Contains(text, "## The globals") {
		t.Fatalf("docs via eval:\n%s", text)
	}
}

// TestMCPUnknownSessionIsAHelpfulToolError: agents invent session ids (pi run 2
// call 2 passed "probe"); the cv eval tool must say what exists and to omit it.
func TestMCPUnknownSessionIsAHelpfulToolError(t *testing.T) {
	cs := connect(t, cvsession.Options{Image: tiffPath})
	ctx := context.Background()
	for _, tool := range []string{"eval", "history"} {
		args := map[string]any{"sessionId": "probe"}
		if tool == "eval" {
			args["code"] = "return 1"
		}
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("%s: protocol error %v", tool, err)
		}
		text, _, isErr := splitResult(t, res)
		if !isErr || !strings.Contains(text, `session "probe" does not exist`) || !strings.Contains(text, "omit sessionId") ||
			!strings.Contains(text, "There is only the default session") {
			t.Fatalf("%s: isErr=%v text=%q", tool, isErr, text)
		}
	}
	// the default session still works and "probe" was not remembered
	text, _, isErr := callEval(t, cs, "return 2")
	if isErr || !strings.HasPrefix(text, "2") {
		t.Fatalf("default session: %q", text)
	}
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_sessions", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if text, _, _ := splitResult(t, res); strings.Contains(text, "probe") {
		t.Fatalf("list_sessions remembers probe: %s", text)
	}
	if _, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "create_session", Arguments: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "eval", Arguments: map[string]any{"code": "return 1", "sessionId": "probe"}})
	if err != nil {
		t.Fatal(err)
	}
	if text, _, _ := splitResult(t, res); !strings.Contains(text, "Existing sessions: ") || !strings.Contains(text, "(default)") {
		t.Fatalf("after create_session: %s", text)
	}
}

func TestAppEvaluateKeepsSessionNotFound(t *testing.T) {
	app, _ := newApp(t, cvsession.Options{})
	_, err := app.Evaluate(context.Background(), "probe", "return 1")
	if !errors.Is(err, replsession.ErrSessionNotFound) {
		t.Fatalf("err %v does not wrap replsession.ErrSessionNotFound", err)
	}
}
