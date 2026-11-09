package cvsession_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cvsession"
)

func contentByCaption(t *testing.T, app *cvsession.App, sid, code string) (string, map[string]string) {
	t.Helper()
	resp := eval(t, app, sid, code)
	out := mustOK(t, resp)
	items := map[string]string{}
	for _, c := range resp.Content {
		items[c.Caption] = string(c.Data)
	}
	return out, items
}

func TestDataContentLazyEagerAndColumns(t *testing.T) {
	app, sid := acceptanceApp(t)
	out, items := contentByCaption(t, app, sid, `
const specks = img.detect.specks().sortBy("area").top(5)
const r = await specks
r.items[0].note = "edited"
const rows = await img.measure([[200.5, 46.5], [43.5, 67.5]])
return {
  lazy: cv.csv(specks, { name: "specks" }),
  eager: cv.json(r.items, { name: "edited" }),
  cols: cv.csv(specks, { name: "cols", columns: ["id", "cx", "cy", "score"] }),
  rows: cv.json(rows, { name: "rows" }),
}`)
	_, prof := contentByCaption(t, app, sid, `return cv.csv(img.profile([190.5, 46.5], [195.5, 46.5]), { name: "prof" })`)
	mustContain(t, out, "lazy: '[content 1: data specks.csv (15 columns of top)]'", "eager: '[content 2: data edited.json (5 items)]'",
		"cols: '[content 3: data cols.csv (4 columns of top)]'")
	csv := items["data specks.csv"]
	if lines := strings.Split(strings.TrimSpace(csv), "\n"); len(lines) != 6 || lines[0] != "id,x,y,w,h,cx,cy,area,aspect,contrast,localNoise,sharpness,score,meanL,polarity" {
		t.Fatalf("specks.csv:\n%s", csv)
	}
	var edited []map[string]any
	if err := json.Unmarshal([]byte(items["data edited.json"]), &edited); err != nil || len(edited) != 5 || edited[0]["note"] != "edited" {
		t.Fatalf("edited.json %v: %s", err, items["data edited.json"])
	}
	if !strings.HasPrefix(items["data cols.csv"], "id,cx,cy,score\n") {
		t.Fatalf("cols.csv:\n%s", items["data cols.csv"])
	}
	if !strings.HasPrefix(items["data rows.json"], `[{"cx":200.5,"cy":46.5,"coreMean":`) {
		t.Fatalf("rows.json in schema order:\n%s", items["data rows.json"])
	}
	if p := prof["data prof.csv"]; !strings.HasPrefix(p, "t,cx,cy,meanL,meanR,meanG,meanB\n0,190.5,46.5,") || strings.Count(p, "\n") != 7 {
		t.Fatalf("prof.csv:\n%s", p)
	}
	// review of PR #4: a 2-sample profile is 2 rows (its from/to pairs are not
	// columns), and control characters are JSON escapes, not Go ones.
	_, short := contentByCaption(t, app, sid, `return {
  p: cv.csv(img.profile([0.5, 0.5], [1.5, 0.5]), { name: "short" }),
  j: cv.json({ "k\u0007": "a\u0000b\u000bc<&>" }, { name: "ctl" }),
}`)
	if p := short["data short.csv"]; !strings.HasPrefix(p, "t,cx,cy,meanL,") || strings.Count(p, "\n") != 3 {
		t.Fatalf("2-sample profile csv:\n%s", p)
	}
	var ctl map[string]string
	if err := json.Unmarshal([]byte(short["data ctl.json"]), &ctl); err != nil || ctl["k\a"] != "a\x00b\vc<&>" {
		t.Fatalf("ctl.json %v: %q", err, short["data ctl.json"])
	}
	for _, tc := range []struct{ code, want string }{
		{`return cv.json(img)`, `fix: node.save("x.tif")`},
		{`return cv.csv(img.detect.specks(), { name: "../x.csv" })`, `fix: 'cv.csv(x, { name: "x" })'`},
		{`return cv.csv(img.detect.specks(), { columns: ["scroe"] })`, `Did you mean "score"?`},
		{`return cv.json(1, { nmae: "x" })`, `Did you mean "name"?`},
	} {
		mustContain(t, text(eval(t, app, sid, tc.code)), tc.want)
	}
}

func TestDataSaveAndBudgets(t *testing.T) {
	img := planted(t)
	root := t.TempDir()
	app, sid := newApp(t, cvsession.Options{Image: img, BaseDir: filepath.Dir(img), OutRoot: root, MaxDataBytes: 4 << 10})
	out := mustOK(t, eval(t, app, sid, `
const s = await cv.csv(img.detect.specks(), { name: "x" }).save("t/specks")
const j = await cv.json({ a: 1 }).save("t/a.json")
return { s: [s.format, s.rows, s.path.endsWith("t/specks.csv")].join(" "), j: j.rows }`))
	mustContain(t, out, "s: csv 41 true", "j: 1")
	mustContain(t, text(eval(t, app, sid, `return await cv.json({ a: 1 }).save("a.csv")`)), "code: E_ARG_TYPE", `cv.json(x).save("a.json")`)

	// five looks and five data items: the fifth of each is not delivered
	resp := eval(t, app, sid, `return [1, 2, 3, 4, 5].map(i => img.exposure(i / 10).look("l" + i)).concat([1, 2, 3, 4, 5].map(i => cv.json({ i }, { name: "d" + i })))`)
	out = mustOK(t, resp)
	mustContain(t, out, "[not delivered: look: l5", "more than 4 looks in one eval", "[not delivered: data d5.json", "more than 4 data items",
		`LOOKS: 1 look was not delivered (at most 4 per eval): "l5". Split the work over several calls, or return fewer looks`,
		`DATA: 1 data item was not delivered (at most 4 per eval): "d5.json (1 keys)". Split the work over several calls, or return fewer data items`)
	if len(resp.Content) != 8 {
		t.Fatalf("%d content items, want 8", len(resp.Content))
	}
	// shown looks beyond the budget: marked in the shown list and named in LOOKS, also when the eval throws
	out = mustOK(t, eval(t, app, sid, `for (const i of [1, 2, 3, 4, 5, 6]) show(img.exposure(i / 10), { label: "s" + i }); return 1`))
	mustContain(t, out, "[not delivered: look: s5", "[not delivered: look: s6",
		`LOOKS: 2 looks were not delivered (at most 4 per eval): "s5", "s6". Split the work over several calls, or return fewer looks`)
	mustNotContain(t, out, "[look: s5", "DATA:")
	out = text(eval(t, app, sid, `for (const i of [1, 2, 3, 4, 5]) show(img.exposure(i / 10), { label: "t" + i }); throw new Error("boom")`))
	mustContain(t, out, "boom", "[not delivered: look: t5", `LOOKS: 1 look was not delivered (at most 4 per eval): "t5"`)
	// within the budget: no warning
	mustNotContain(t, mustOK(t, eval(t, app, sid, `return [1, 2, 3, 4].map(i => img.exposure(i / 10).look("ok" + i))`)), "LOOKS", "not delivered")
	// over the byte limit: a note to save it
	resp = eval(t, app, sid, `return cv.csv(img.measure(Array.from({ length: 200 }, (_, i) => [i + 0.5, 50.5])), { name: "big" })`)
	mustOK(t, resp)
	if len(resp.Content) != 1 || resp.Content[0].MIMEType != "text/plain" || !strings.Contains(string(resp.Content[0].Data), `too large for the conversation`) ||
		!strings.Contains(string(resp.Content[0].Data), `.save("big.csv")`) {
		t.Fatalf("big item: %+v", resp.Content)
	}
}
