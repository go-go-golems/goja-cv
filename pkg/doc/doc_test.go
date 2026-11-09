package doc

import (
	"regexp"
	"strings"
	"testing"

	cvmod "github.com/go-go-golems/goja-cv/pkg/gojamod/cv"
)

var wantSlugs = []string{"what-is-goja-cv", "concepts", "cli", "agents", "setup-claude-code", "setup-codex", "setup-pi", "recipes", "embedding", "troubleshooting"}

func TestPagesParseAndLoad(t *testing.T) {
	pages, err := Pages()
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != len(wantSlugs) {
		t.Fatalf("%d pages, want %d", len(pages), len(wantSlugs))
	}
	seen := map[string]bool{}
	for i, p := range pages {
		if p.Slug != wantSlugs[i] || p.Short == "" || p.SectionType == "" || len(p.Body) < 500 {
			t.Fatalf("page %d (%s): %+v", i, p.File, p)
		}
		if seen[p.Slug] {
			t.Fatalf("duplicate slug %s", p.Slug)
		}
		seen[p.Slug] = true
	}
	hs, err := NewHelpSystem()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range wantSlugs {
		sec, err := hs.GetSectionWithSlug(s)
		if err != nil || sec.Title == "" {
			t.Fatalf("help system: %s: %v", s, err)
		}
	}
}

// TestHelpReferencesResolve: every "goja-cv help X" in the pages names a
// page, a command or a cv op/kind/function.
func TestHelpReferencesResolve(t *testing.T) {
	pages, err := Pages()
	if err != nil {
		t.Fatal(err)
	}
	slugs := map[string]bool{}
	for _, p := range pages {
		slugs[p.Slug] = true
	}
	commands := map[string]bool{"run": true, "eval": true, "ops": true, "help": true, "mcp": true}
	ref := regexp.MustCompile("goja-cv help ([A-Za-z][\\w.-]*)")
	n := 0
	for _, p := range pages {
		for _, m := range ref.FindAllStringSubmatch(p.Body, -1) {
			n++
			name := m[1]
			if name == strings.ToUpper(name) { // a placeholder: goja-cv help NAME
				continue
			}
			if slugs[name] || commands[name] {
				continue
			}
			if h := cvmod.Help(cvmod.DefaultRegistry(), name); h["found"] == false {
				t.Fatalf("%s: goja-cv help %s resolves to nothing", p.Slug, name)
			}
		}
	}
	if n < 10 {
		t.Fatalf("only %d help references", n)
	}
}
