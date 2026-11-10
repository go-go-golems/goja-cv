package graph

import (
	"bufio"
	"io"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// Script files (GCV-005 §6.1, §6.2): cv.load runs a .js file found like any
// file cv.open reads, and cv.scripts lists the ones it can run. A script's
// name appears in stack frames ("dust/helpers.js:57:3"), so it has no
// whitespace or parentheses.

const (
	scriptLevels  = 4    // directory levels cv.scripts visits (the root and three below it)
	scriptEntries = 5000 // entries visited per root
	scriptFiles   = 500  // files listed
	scriptDocRead = 4096 // bytes read for a file's first comment
)

// Script is a .js file cv.load can run: the path it accepts and the
// file's first comment line.
type Script struct {
	Path string
	Doc  string
}

// LoadableName reports whether p names a file cv.load runs: it ends in .js
// and has no whitespace or parentheses.
func LoadableName(p string) bool {
	if !strings.HasSuffix(p, ".js") {
		return false
	}
	return !strings.ContainsFunc(p, func(r rune) bool { return unicode.IsSpace(r) || r == '(' || r == ')' })
}

// ScriptName is the name a loaded file has in stack frames and locations:
// its path relative to the root it was found in ("dust/helpers.js"), or,
// when an earlier root has a file of the same relative path (an absolute
// path into a shadowed read directory), "read<N>:dust/helpers.js" with N the
// read directory's position in flag order.
func (w *Workspace) ScriptName(l Location) string {
	rel := filepath.ToSlash(l.Rel)
	for i := 0; i < l.Root; i++ {
		if _, err := w.at(i).r.Stat(l.Rel); err == nil {
			return "read" + strconv.Itoa(l.Root) + ":" + rel
		}
	}
	return rel
}

// Scripts lists the .js files cv.load can run, sorted by path: a bounded
// walk of the workspace and the read directories (scriptLevels levels,
// scriptEntries entries per root, scriptFiles files), skipping directories
// whose name starts with "." and node_modules. A file shadowed by one with
// the same relative path in an earlier root is not listed. It never fails.
func (w *Workspace) Scripts() []Script {
	seen := map[string]bool{}
	var out []Script
	for i := 0; i <= len(w.reads) && len(out) < scriptFiles; i++ {
		fsys := w.at(i).r.FS()
		visited := 0
		_ = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
			if visited++; visited > scriptEntries || len(out) >= scriptFiles {
				return fs.SkipAll
			}
			if err != nil || p == "." {
				return nil
			}
			if d.IsDir() {
				if n := d.Name(); strings.HasPrefix(n, ".") || n == "node_modules" || strings.Count(p, "/")+2 > scriptLevels {
					return fs.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() || !LoadableName(p) || seen[p] {
				return nil
			}
			seen[p] = true
			out = append(out, Script{Path: p, Doc: firstComment(fsys, p)})
			return nil
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// firstComment is the text of the first comment of a script, when the
// file starts with one (blank lines skipped): a // line, or the first
// non-empty line of a /* block.
func firstComment(fsys fs.FS, p string) string {
	f, err := fsys.Open(p)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(io.LimitReader(f, scriptDocRead))
	block := false
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		switch {
		case block:
			l = strings.TrimSpace(strings.TrimSuffix(strings.TrimLeft(l, "*"), "*/"))
			if l != "" {
				return l
			}
			if strings.Contains(sc.Text(), "*/") {
				return ""
			}
		case l == "":
		case strings.HasPrefix(l, "//"):
			return strings.TrimSpace(strings.TrimPrefix(l, "//"))
		case strings.HasPrefix(l, "/*"):
			l = strings.TrimSpace(strings.TrimSuffix(strings.TrimLeft(strings.TrimPrefix(l, "/*"), "*"), "*/"))
			if l != "" {
				return l
			}
			block = true
		default:
			return ""
		}
	}
	return ""
}
