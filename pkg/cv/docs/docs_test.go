package docs

import (
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

func TestListHasEveryTopicWithTitleAndSummary(t *testing.T) {
	want := []string{"getting-started", "units-and-scales", "nodes-and-evaluation", "lambdas", "masks-and-regions", "looks", "errors", "recipes", "encodings-and-spaces", "views", "measuring", "data-out", "task-libraries"}
	list := List()
	if len(list) != len(want) {
		t.Fatalf("topics %v", IDs())
	}
	for i, e := range list {
		if e.ID != want[i] || e.Title == "" || len(e.Summary) < 40 || strings.HasPrefix(e.Summary, "#") {
			t.Fatalf("entry %d = %+v", i, e)
		}
	}
	for _, tp := range Topics() {
		if len(tp.Sections) < 2 || tp.Sections[0].Title != "" {
			t.Fatalf("%s sections %d", tp.ID, len(tp.Sections))
		}
		if len(tp.Body) > 6000 {
			t.Fatalf("%s is %d characters; keep topics short", tp.ID, len(tp.Body))
		}
	}
}

func TestReadTopicAndSection(t *testing.T) {
	md, err := Read("recipes", "")
	if err != nil || !strings.HasPrefix(md, "# Recipes") || !strings.Contains(md, "## Remove dust") {
		t.Fatalf("read recipes: %v\n%s", err, md)
	}
	sec, err := Read("recipes", "remove dust")
	if err != nil || !strings.HasPrefix(sec, "## Remove dust") || strings.Contains(sec, "## Edit inside a mask") {
		t.Fatalf("section: %v\n%s", err, sec)
	}
	if sec2, err := Read("RECIPES", "Remove"); err != nil || sec2 != sec {
		t.Fatalf("prefix/case section: %v", err)
	}
}

func TestReadUnknownTopicAndSection(t *testing.T) {
	_, err := Read("recipe", "")
	e, ok := graph.AsError(err)
	if !ok || e.Code != EUnknownTopic || e.Hint != `Did you mean "recipes"?` || e.Fix != `cv.docs.read("recipes")` || len(e.Valid) != 13 {
		t.Fatalf("unknown topic: %+v", err)
	}
	_, err = Read("dust", "")
	if e, ok := graph.AsError(err); !ok || !strings.Contains(e.Fix, `cv.docs.read("recipes", { section: "Remove dust" })`) {
		t.Fatalf("search fallback: %+v", err)
	}
	_, err = Read("recipes", "remove dsut")
	if e, ok := graph.AsError(err); !ok || e.Code != EUnknownTopic || e.Fix != `cv.docs.read("recipes", { section: "Remove dust" })` {
		t.Fatalf("unknown section: %+v", err)
	}
}

func TestSearchRanksTheRightSection(t *testing.T) {
	for q, want := range map[string]string{
		"heal dust specks":      "recipes/Remove dust",
		"feather mask":          "masks-and-regions/Combining and refining",
		"E_NOT_AWAITED":         "nodes-and-evaluation/E_NOT_AWAITED",
		"analysisScale":         "units-and-scales/analysisScale",
		"literal options await": "nodes-and-evaluation/Options are literal values",
		"looks delivered order": "looks/Delivery",
	} {
		hits := Search(q, 3)
		found := false
		for _, h := range hits[:min(2, len(hits))] {
			found = found || h.ID+"/"+h.Section == want
		}
		if !found {
			t.Fatalf("search %q: %+v, want %s in the top two", q, hits, want)
		}
		if hits[0].Snippet == "" || hits[0].Score <= 0 {
			t.Fatalf("search %q: empty snippet/score %+v", q, hits[0])
		}
	}
	if len(Search("the", 5)) != 0 || len(Search("zzzqqq", 5)) != 0 {
		t.Fatal("stopwords / nonsense should find nothing")
	}
	if len(Search("mask", 2)) != 2 {
		t.Fatal("limit")
	}
}
