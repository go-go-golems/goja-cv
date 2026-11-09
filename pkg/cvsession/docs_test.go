package cvsession_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/go-go-golems/goja-cv/internal/testimage"
	"github.com/go-go-golems/goja-cv/pkg/cv/docs"
	"github.com/go-go-golems/goja-cv/pkg/cvsession"
)

var (
	plantedOnce sync.Once
	plantedPath string
	plantedErr  error
)

// planted writes the planted-fault scan once per test binary.
func planted(t *testing.T) string {
	t.Helper()
	plantedOnce.Do(func() {
		dir, err := os.MkdirTemp("", "cvdocs-")
		if err != nil {
			plantedErr = err
			return
		}
		plantedPath = filepath.Join(dir, testimage.PlantedTIFF)
		_, plantedErr = testimage.WritePlanted(context.Background(), plantedPath)
	})
	if plantedErr != nil {
		t.Fatal(plantedErr)
	}
	return plantedPath
}

type codeBlock struct {
	info, code string
	line       int
}

func TestMain(m *testing.M) {
	code := m.Run()
	if plantedPath != "" {
		_ = os.RemoveAll(filepath.Dir(plantedPath))
	}
	os.Exit(code)
}

func jsBlocks(md string) []codeBlock {
	var out []codeBlock
	lines := strings.Split(md, "\n")
	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "```") {
			continue
		}
		info := strings.TrimSpace(strings.TrimPrefix(lines[i], "```"))
		start := i
		var body []string
		for i++; i < len(lines) && !strings.HasPrefix(lines[i], "```"); i++ {
			body = append(body, lines[i])
		}
		if info == "js" || strings.HasPrefix(info, "js ") {
			out = append(out, codeBlock{info: info, code: strings.Join(body, "\n"), line: start + 1})
		}
	}
	return out
}

// TestDocsCodeBlocksRun evaluates every ```js block of every topic, in
// order, in one session per topic with the planted scan as img.
func TestDocsCodeBlocksRun(t *testing.T) {
	img := planted(t)
	total := 0
	for _, topic := range docs.Topics() {
		t.Run(topic.ID, func(t *testing.T) {
			app, sid := newApp(t, cvsession.Options{Image: img})
			for _, b := range jsBlocks(topic.Body) {
				if strings.Contains(b.info, "fragment") {
					continue
				}
				total++
				resp := eval(t, app, sid, b.code)
				out := text(resp)
				exec := resp.Cell.Execution
				where := topic.ID + ".md line " + strconv.Itoa(b.line)
				switch {
				case strings.Contains(b.info, "error="):
					code := b.info[strings.Index(b.info, "error=")+len("error="):]
					if exec.Status == "ok" || !strings.Contains(out, "code: "+code) {
						t.Fatalf("%s: want error %s, got status %s:\n%s", where, code, exec.Status, out)
					}
				default:
					if exec.Status != "ok" {
						t.Fatalf("%s: status %s %s:\n%s", where, exec.Status, exec.Error, out)
					}
					if i := strings.Index(b.info, "warns="); i >= 0 {
						if code := b.info[i+len("warns="):]; !strings.Contains(out, code+" line") {
							t.Fatalf("%s: want warning %s:\n%s", where, code, out)
						}
					} else if strings.Contains(out, "warnings:") {
						t.Fatalf("%s: unexpected warnings:\n%s", where, out)
					}
				}
				for _, ref := range exec.Content {
					if ref.Dropped != "" {
						t.Fatalf("%s: look dropped: %s", where, ref.Dropped)
					}
				}
			}
		})
	}
	t.Logf("%d code blocks ran", total)
	if total < 25 {
		t.Fatalf("only %d runnable blocks", total)
	}
}

func TestDocsFromEval(t *testing.T) {
	app, sid := newApp(t, cvsession.Options{Image: tiffPath})
	out := text(eval(t, app, sid, `return cv.docs.list().map(t => t.id)`))
	mustContain(t, out, "getting-started", "recipes")
	out = text(eval(t, app, sid, `return cv.docs.search("heal dust", { limit: 2 })`))
	mustContain(t, out, "id: recipes", "section: Remove dust", "snippet:", "score:")
	out = text(eval(t, app, sid, `return cv.docs.read("recipes", { section: "Remove dust" })`))
	mustContain(t, out, "## Remove dust", "img.detect.specks()")
	mustNotContain(t, out, "## Edit inside a mask")
	out = text(eval(t, app, sid, `return cv.docs.read("recipe")`))
	mustContain(t, out, "code: E_UNKNOWN_TOPIC", `hint: Did you mean "recipes"?`, `fix: cv.docs.read("recipes")`)
	out = text(eval(t, app, sid, `return cv.docs.read("recipes", { sectoin: "x" })`))
	mustContain(t, out, "code: E_UNKNOWN_OPTION", `Did you mean "section"?`)
	out = text(eval(t, app, sid, `return cv.docs.search("")`))
	mustContain(t, out, "code: E_ARG_TYPE")
	out = text(eval(t, app, sid, `const h = cv.help(); return { docs: h.docs, topics: h.topics }`))
	mustContain(t, out, "cv.docs.search(text)", "getting-started")
	out = text(eval(t, app, sid, `return cv.help("heal").heal.usage`))
	mustContain(t, out, "heal(regions|mask, { radius, method })")
}
