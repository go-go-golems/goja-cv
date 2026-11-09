// Package doc is the goja-cv guidebook: Glazed help pages (Markdown with
// help frontmatter) embedded in the binary. `goja-cv help <slug>` renders
// them through the Glazed help system, and the MCP server serves them as
// cv://guide/<slug> resources.
//
// The agent-facing topics read from inside eval (cv.docs) live in
// pkg/cv/docs; these pages are for the people who set goja-cv up.
package doc

import (
	"embed"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/go-go-golems/glazed/pkg/help"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

//go:embed *.md
var docFS embed.FS

// AddDocToHelpSystem loads the guidebook into a Glazed help system.
func AddDocToHelpSystem(hs *help.HelpSystem) error {
	return hs.LoadSectionsFromFS(docFS, ".")
}

// NewHelpSystem returns a help system with the guidebook loaded.
func NewHelpSystem() (*help.HelpSystem, error) {
	hs := help.NewHelpSystem()
	if err := AddDocToHelpSystem(hs); err != nil {
		return nil, err
	}
	return hs, nil
}

// Page is one guidebook page.
type Page struct {
	Slug        string   `yaml:"Slug"`
	Title       string   `yaml:"Title"`
	Short       string   `yaml:"Short"`
	SectionType string   `yaml:"SectionType"`
	IsTopLevel  bool     `yaml:"IsTopLevel"`
	Topics      []string `yaml:"Topics"`
	Commands    []string `yaml:"Commands"`
	File        string   `yaml:"-"`
	Body        string   `yaml:"-"` // Markdown without the frontmatter
}

// Pages parses every page's frontmatter and body, ordered by file name.
func Pages() ([]Page, error) {
	names, err := fs.Glob(docFS, "*.md")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	var out []Page
	for _, n := range names {
		b, err := docFS.ReadFile(n)
		if err != nil {
			return nil, err
		}
		p, err := parsePage(b)
		if err != nil {
			return nil, errors.Wrapf(err, "guide page %s", n)
		}
		p.File = path.Base(n)
		out = append(out, p)
	}
	return out, nil
}

func parsePage(b []byte) (Page, error) {
	var p Page
	s := strings.TrimPrefix(string(b), "\uFEFF")
	if !strings.HasPrefix(s, "---\n") {
		return p, errors.New("missing frontmatter")
	}
	end := strings.Index(s[4:], "\n---\n")
	if end < 0 {
		return p, errors.New("unterminated frontmatter")
	}
	if err := yaml.Unmarshal([]byte(s[4:4+end]), &p); err != nil {
		return p, errors.Wrap(err, "frontmatter")
	}
	if p.Slug == "" || p.Title == "" {
		return p, errors.New("frontmatter needs Slug and Title")
	}
	p.Body = strings.TrimSpace(s[4+end+5:]) + "\n"
	return p, nil
}
