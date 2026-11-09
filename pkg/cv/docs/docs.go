// Package docs holds cv's agent documentation: short Markdown topics embedded
// in the binary, listed, searched and read from inside an eval through
// cv.docs (and served as MCP resources cv://docs/<id>).
//
// A topic file is topics/NN-id.md. Its first line is "# Title", the first
// paragraph after it is the summary, and "## " headings split it into
// sections that cv.docs.read(id, { section }) returns one at a time.
//
// Code blocks fenced as ```js run as eval bodies, in order, in one session per
// topic with the planted test scan as img (TestDocsCodeBlocksRun in
// pkg/cvsession). Info-string markers: "js fragment" is not run, "js
// error=E_CODE" must fail with that code, "js warns=CODE" must succeed with
// that warning; plain "js" must succeed without warnings.
package docs

import (
	"embed"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// EUnknownTopic is the error code of a topic or section that does not exist.
const EUnknownTopic = "E_UNKNOWN_TOPIC"

//go:embed topics/*.md
var topicFS embed.FS

// Section is one "## " part of a topic. The part before the first "## "
// heading is the intro (Title "").
type Section struct {
	Title string
	Body  string // Markdown including the heading line
}

// Topic is one documentation page.
type Topic struct {
	ID       string
	Title    string
	Summary  string
	Body     string // the whole Markdown file
	Sections []Section
}

// Entry is one row of List.
type Entry struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

// Hit is one search result.
type Hit struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Section string  `json:"section"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

var (
	loadOnce sync.Once
	topics   []*Topic
	byID     map[string]*Topic
)

var fileName = regexp.MustCompile(`^(\d+)-([a-z0-9-]+)\.md$`)

func load() {
	loadOnce.Do(func() {
		entries, err := topicFS.ReadDir("topics")
		if err != nil {
			panic(fmt.Sprintf("cv/docs: %v", err))
		}
		byID = map[string]*Topic{}
		for _, e := range entries {
			m := fileName.FindStringSubmatch(e.Name())
			if m == nil {
				panic(fmt.Sprintf("cv/docs: topic file %q is not named NN-id.md", e.Name()))
			}
			b, err := topicFS.ReadFile(path.Join("topics", e.Name()))
			if err != nil {
				panic(fmt.Sprintf("cv/docs: %v", err))
			}
			t := parse(m[2], string(b))
			topics = append(topics, t)
			byID[t.ID] = t
		}
		// ReadDir returns names sorted, so the NN prefix orders the topics.
	})
}

func parse(id, src string) *Topic {
	t := &Topic{ID: id, Body: strings.TrimSpace(src) + "\n"}
	lines := strings.Split(t.Body, "\n")
	cur := &Section{}
	var body []string
	flush := func() {
		cur.Body = strings.TrimSpace(strings.Join(body, "\n")) + "\n"
		t.Sections = append(t.Sections, *cur)
	}
	inCode := false
	for i, l := range lines {
		if strings.HasPrefix(l, "```") {
			inCode = !inCode
		}
		switch {
		case i == 0 && strings.HasPrefix(l, "# "):
			t.Title = strings.TrimSpace(l[2:])
			body = append(body, l)
		case !inCode && strings.HasPrefix(l, "## "):
			flush()
			cur, body = &Section{Title: strings.TrimSpace(l[3:])}, []string{l}
		default:
			body = append(body, l)
		}
	}
	flush()
	// summary: the first non-empty paragraph after the title
	var para []string
	for _, l := range lines[1:] {
		if strings.TrimSpace(l) == "" {
			if len(para) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(l, "#") || strings.HasPrefix(l, "```") {
			break
		}
		para = append(para, strings.TrimSpace(l))
	}
	t.Summary = strings.Join(para, " ")
	return t
}

// Topics returns every topic in order.
func Topics() []*Topic {
	load()
	return topics
}

// IDs lists the topic ids in order.
func IDs() []string {
	load()
	out := make([]string, len(topics))
	for i, t := range topics {
		out[i] = t.ID
	}
	return out
}

// List returns id, title and summary of every topic.
func List() []Entry {
	load()
	out := make([]Entry, len(topics))
	for i, t := range topics {
		out[i] = Entry{ID: t.ID, Title: t.Title, Summary: t.Summary}
	}
	return out
}

// Get returns topic id or E_UNKNOWN_TOPIC with a did-you-mean.
func Get(id string) (*Topic, error) {
	load()
	if t, ok := byID[strings.TrimSpace(strings.ToLower(id))]; ok {
		return t, nil
	}
	e := graph.Errorf(EUnknownTopic, "There is no docs topic %q.", id).WithValid(IDs())
	if sug := graph.Closest(id, IDs()); sug != "" {
		e.Hint = fmt.Sprintf("Did you mean %q?", sug)
		e.Fix = fmt.Sprintf("cv.docs.read(%q)", sug)
	} else if hits := Search(id, 1); len(hits) > 0 {
		e.Hint = fmt.Sprintf("Search finds %q (section %q).", hits[0].ID, hits[0].Section)
		e.Fix = fmt.Sprintf("cv.docs.read(%q, { section: %q })", hits[0].ID, hits[0].Section)
	} else {
		e.Hint = "cv.docs.list() lists the topics; cv.docs.search(text) finds sections."
		e.Fix = "cv.docs.list()"
	}
	return nil, e
}

// Read returns the Markdown of topic id, or of one of its sections (matched
// by title, case-insensitively, or by a prefix of it).
func Read(id, section string) (string, error) {
	t, err := Get(id)
	if err != nil {
		return "", err
	}
	section = strings.TrimSpace(section)
	if section == "" {
		return t.Body, nil
	}
	var titles []string
	for _, s := range t.Sections {
		if s.Title != "" {
			titles = append(titles, s.Title)
		}
	}
	for _, s := range t.Sections {
		if s.Title != "" && strings.EqualFold(s.Title, section) {
			return s.Body, nil
		}
	}
	for _, s := range t.Sections {
		if s.Title != "" && strings.HasPrefix(strings.ToLower(s.Title), strings.ToLower(section)) {
			return s.Body, nil
		}
	}
	e := graph.Errorf(EUnknownTopic, "Topic %q has no section %q.", t.ID, section).WithValid(titles)
	if sug := graph.Closest(section, titles); sug != "" {
		e.Hint = fmt.Sprintf("Did you mean %q?", sug)
		e.Fix = fmt.Sprintf("cv.docs.read(%q, { section: %q })", t.ID, sug)
	} else {
		e.Hint = "Sections: " + strings.Join(titles, ", ") + "."
		e.Fix = fmt.Sprintf("cv.docs.read(%q)", t.ID)
	}
	return "", e
}

var word = regexp.MustCompile(`[A-Za-z0-9_.]+`)

var stop = map[string]bool{"a": true, "an": true, "the": true, "to": true, "of": true, "in": true, "on": true, "and": true,
	"or": true, "is": true, "it": true, "i": true, "how": true, "do": true, "what": true, "with": true, "for": true, "my": true}

func terms(q string) []string {
	var out []string
	seen := map[string]bool{}
	for _, w := range word.FindAllString(strings.ToLower(q), -1) {
		w = strings.Trim(w, ".")
		if w == "" || stop[w] || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	return out
}

// Search ranks sections by keyword matches (title matches weigh more, a
// section must match every term or score at least as many term hits as
// there are terms) and returns at most limit hits (default 8).
func Search(q string, limit int) []Hit {
	load()
	if limit <= 0 {
		limit = 8
	}
	ts := terms(q)
	if len(ts) == 0 {
		return []Hit{}
	}
	phrase := strings.ToLower(strings.TrimSpace(q))
	var hits []Hit
	for _, t := range topics {
		for _, s := range t.Sections {
			title := strings.ToLower(t.Title + " " + s.Title)
			body := strings.ToLower(s.Body)
			score, matched := 0.0, 0
			for _, w := range ts {
				c := strings.Count(body, w)
				tc := strings.Count(title, w)
				if c+tc > 0 {
					matched++
				}
				score += float64(min(c, 6)) + 4*float64(tc)
			}
			if matched == 0 {
				continue
			}
			score *= float64(matched) / float64(len(ts)) // favour sections that match every term
			if len(ts) > 1 && strings.Contains(body, phrase) {
				score += 5
			}
			sec := s.Title
			if sec == "" {
				sec = "(intro)"
			}
			hits = append(hits, Hit{ID: t.ID, Title: t.Title, Section: sec, Snippet: snippet(s.Body, ts), Score: float64(int(score*10+0.5)) / 10})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	if hits == nil {
		hits = []Hit{}
	}
	return hits
}

// snippet is about 160 characters of one line around the first term found.
func snippet(body string, ts []string) string {
	flat := strings.Join(strings.Fields(body), " ")
	low := strings.ToLower(flat)
	at := -1
	for _, w := range ts {
		if i := strings.Index(low, w); i >= 0 && (at < 0 || i < at) {
			at = i
		}
	}
	if at < 0 {
		at = 0
	}
	start := max(0, at-60)
	end := min(len(flat), start+160)
	for start > 0 && !utf8Start(flat[start]) {
		start--
	}
	for end < len(flat) && !utf8Start(flat[end]) {
		end++
	}
	out := flat[start:end]
	if start > 0 {
		out = "…" + out
	}
	if end < len(flat) {
		out += "…"
	}
	return out
}

func utf8Start(b byte) bool { return b < 0x80 || b >= 0xC0 }
