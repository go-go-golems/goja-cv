package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil { // #nosec G306 -- test files
			t.Fatal(err)
		}
	}
}

// TestScripts: doc lines, skipped directories, depth, shadowing, names
// cv.load cannot run, and the file and entry bounds.
func TestScripts(t *testing.T) {
	root, read := t.TempDir(), t.TempDir()
	writeTree(t, root, map[string]string{
		"a.js":              "\n// a: first comment\nconst x = 1\n",
		"b.js":              "/*\n * b: block comment\n */\n",
		"c.js":              "const c = 1 // not a doc\n",
		"x/y/z/d.js":        "// four levels\n",
		"x/y/z/w/e.js":      "// five levels: not visited\n",
		".git/h.js":         "//\n",
		"node_modules/m.js": "//\n",
		"with space.js":     "//\n",
		"notes.md":          "# no",
		"shared/s.js":       "// from the workspace\n",
	})
	writeTree(t, read, map[string]string{"shared/s.js": "// from the read directory\n", "r.js": "// read only\n"})
	w, err := OpenWorkspace(root, []string{read})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	got := w.Scripts()
	want := []Script{{"a.js", "a: first comment"}, {"b.js", "b: block comment"}, {"c.js", ""}, {"r.js", "read only"}, {"shared/s.js", "from the workspace"}, {"x/y/z/d.js", "four levels"}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("scripts\n got %v\nwant %v", got, want)
	}
	// names: a shadowed file reached in a later root is read<N>:
	if n := w.ScriptName(Location{Root: 1, Rel: "shared/s.js"}); n != "read1:shared/s.js" {
		t.Fatalf("shadowed name %q", n)
	}
	if n := w.ScriptName(Location{Root: 1, Rel: "r.js"}); n != "r.js" {
		t.Fatalf("name %q", n)
	}

	// bounds: at most scriptFiles files
	many := t.TempDir()
	files := map[string]string{}
	for i := 0; i < scriptFiles+20; i++ {
		files[fmt.Sprintf("f%04d.js", i)] = ""
	}
	writeTree(t, many, files)
	wm, err := OpenWorkspace(many, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer wm.Close()
	if n := len(wm.Scripts()); n != scriptFiles {
		t.Fatalf("%d files listed, want %d", n, scriptFiles)
	}
	// bounds: at most scriptEntries entries visited per root
	wide := t.TempDir()
	files = map[string]string{"zz/last.js": ""}
	for i := 0; i < scriptEntries; i++ {
		files[fmt.Sprintf("a%05d.txt", i)] = ""
	}
	writeTree(t, wide, files)
	ww, err := OpenWorkspace(wide, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ww.Close()
	if n := len(ww.Scripts()); n != 0 {
		t.Fatalf("the walk went past %d entries: %d scripts", scriptEntries, n)
	}
}
