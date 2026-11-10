package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mkfile(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func wantCode(t *testing.T, what string, err error, code string, contains ...string) {
	t.Helper()
	ce, ok := AsError(err)
	if !ok || ce.Code != code {
		t.Fatalf("%s: want %s, got %v", what, code, err)
	}
	for _, c := range contains {
		if !strings.Contains(ce.Message+" "+ce.Hint+" "+ce.Fix, c) {
			t.Fatalf("%s: %q lacks %q", what, ce.Message, c)
		}
	}
}

func readAll(t *testing.T, ws *Workspace, p string) (string, Location) {
	t.Helper()
	f, l, err := ws.Open(p)
	if err != nil {
		t.Fatalf("open %s: %v", p, err)
	}
	defer func() { _ = f.Close() }()
	b := make([]byte, 64)
	n, _ := f.Read(b)
	return string(b[:n]), l
}

// TestWorkspacePaths (§7.8, §10.5): relative names resolve in the
// workspace first, then in the read directories in order; absolute names
// must lie in a root (own or resolved name); .. may not leave; saves are
// workspace-only.
func TestWorkspacePaths(t *testing.T) {
	base := t.TempDir()
	root, r1, r2 := filepath.Join(base, "work"), filepath.Join(base, "r1"), filepath.Join(base, "r2")
	mkfile(t, filepath.Join(root, "both.txt"), "work")
	mkfile(t, filepath.Join(r1, "both.txt"), "r1")
	mkfile(t, filepath.Join(r2, "only2.txt"), "r2")
	mkfile(t, filepath.Join(r1, "sub", "a.txt"), "a")
	mkfile(t, filepath.Join(base, "outside.txt"), "x")
	ws, err := OpenWorkspace(root, []string{r1, r2})
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	if s, l := readAll(t, ws, "both.txt"); s != "work" || l != (Location{0, "both.txt"}) {
		t.Fatalf("both: %s %v (the workspace wins)", s, l)
	}
	if s, l := readAll(t, ws, "only2.txt"); s != "r2" || l != (Location{2, "only2.txt"}) {
		t.Fatalf("second read directory: %s %v", s, l)
	}
	if s, l := readAll(t, ws, filepath.Join(r1, "sub", "a.txt")); s != "a" || l != (Location{1, filepath.Join("sub", "a.txt")}) {
		t.Fatalf("absolute in r1: %s %v", s, l)
	}
	_, _, err = ws.Open("none.txt")
	wantCode(t, "missing", err, EArgType, "no such file", root, r1, r2)
	_, _, err = ws.Open("../outside.txt")
	wantCode(t, "..", err, EAccess)
	_, _, err = ws.Open(filepath.Join(base, "outside.txt"))
	wantCode(t, "absolute outside", err, EAccess, "outside", root, r1)
	// saves: relative is always the workspace; read directories are read-only
	if rel, err := ws.SaveName("only2.txt"); err != nil || rel != "only2.txt" {
		t.Fatalf("save name %q %v", rel, err)
	}
	_, err = ws.SaveName(filepath.Join(r1, "x.json"))
	wantCode(t, "save into a read directory", err, EAccess, "read directories are read-only")
	_, err = ws.SaveName("../x.json")
	wantCode(t, "save ..", err, EAccess)
	if _, err := ws.Write("new/dir/x.json", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "new", "dir", "x.json")); string(b) != "{}" {
		t.Fatalf("written %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(r2, "only2.txt")); string(b) != "r2" {
		t.Fatal("a read directory was written")
	}
}

// TestWorkspaceAliases: a directory reached through a symlink is the root
// by its own or its resolved name (/tmp and /private/tmp on macOS); an
// outside alias is refused, naming both.
func TestWorkspaceAliases(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(base, "data")
	mkfile(t, filepath.Join(data, "a.txt"), "a")
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(data, alias); err != nil {
		t.Fatal(err)
	}
	ws, err := OpenWorkspace(alias, nil) // given through the alias: both names work
	if err != nil {
		t.Fatal(err)
	}
	readAll(t, ws, filepath.Join(alias, "a.txt"))
	readAll(t, ws, filepath.Join(data, "a.txt"))
	ws.Close()
	other := filepath.Join(base, "other")
	if err := os.Symlink(data, other); err != nil {
		t.Fatal(err)
	}
	ws, err = OpenWorkspace(data, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	_, _, err = ws.Open(filepath.Join(other, "a.txt"))
	wantCode(t, "outside alias", err, EAccess, other, data)
}

// TestWorkspaceSymlinks: links inside the workspace that stay inside are
// read but never saved through; links (also dangling ones and linked
// directories) that leave it are refused by os.Root.
func TestWorkspaceSymlinks(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "work")
	mkfile(t, filepath.Join(root, "real.txt"), "real")
	mkfile(t, filepath.Join(base, "secret.txt"), "secret")
	if err := os.MkdirAll(filepath.Join(base, "outdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	for link, target := range map[string]string{
		"in.txt":       "real.txt",
		"out.txt":      "../secret.txt",
		"dangling.txt": "../missing.txt",
		"outdir":       "../outdir",
	} {
		if err := os.Symlink(target, filepath.Join(root, link)); err != nil {
			t.Fatal(err)
		}
	}
	ws, err := OpenWorkspace(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	if s, _ := readAll(t, ws, "in.txt"); s != "real" {
		t.Fatalf("inside link read %q", s)
	}
	_, err = ws.Write("in.txt", []byte("x"))
	wantCode(t, "save through an inside link", err, EAccess, "saves do not follow links")
	for _, p := range []string{"out.txt", "dangling.txt", "outdir/x.txt"} {
		_, _, err := ws.Open(p)
		wantCode(t, "read "+p, err, EAccess)
	}
	for _, p := range []string{"out.txt", "dangling.txt"} {
		_, err := ws.Write(p, []byte("x"))
		wantCode(t, "save "+p, err, EAccess)
	}
	_, err = ws.Write("outdir/x.txt", []byte("x"))
	wantCode(t, "save into a linked directory outside", err, EAccess)
	if _, err := os.Stat(filepath.Join(base, "outdir", "x.txt")); err == nil {
		t.Fatal("wrote through a linked directory")
	}
	if b, _ := os.ReadFile(filepath.Join(base, "secret.txt")); string(b) != "secret" {
		t.Fatal("wrote outside")
	}
	if _, err := os.Stat(filepath.Join(base, "missing.txt")); err == nil {
		t.Fatal("created the dangling target")
	}
}

// TestWorkspaceProtectsOpenedFiles: a save never replaces a file the
// session opened (also through a hard link); a file the session saved may be
// saved again after reopening; other files are replaced (replaced: true).
func TestWorkspaceProtectsOpenedFiles(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "scan.tif"), "pixels")
	mkfile(t, filepath.Join(root, "other.json"), "{}")
	if err := os.Link(filepath.Join(root, "scan.tif"), filepath.Join(root, "hard.tif")); err != nil {
		t.Fatal(err)
	}
	ws, err := OpenWorkspace(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	f, _, err := ws.Open("scan.tif")
	if err != nil {
		t.Fatal(err)
	}
	st, _ := f.Stat()
	_ = f.Close()
	ws.NoteOpened(st)
	for _, p := range []string{"scan.tif", "hard.tif"} {
		_, err := ws.Write(p, []byte("new"))
		wantCode(t, "replace "+p, err, EAccess, "which this session opened")
	}
	if b, _ := os.ReadFile(filepath.Join(root, "scan.tif")); string(b) != "pixels" {
		t.Fatalf("scan.tif is %q", b)
	}
	if replaced, err := ws.Write("other.json", []byte("[]")); err != nil || !replaced {
		t.Fatalf("other.json: %v %v", replaced, err)
	}
	// save → reopen → save
	if replaced, err := ws.Write("out.tif", []byte("1")); err != nil || replaced {
		t.Fatal(replaced, err)
	}
	f, _, _ = ws.Open("out.tif")
	st, _ = f.Stat()
	_ = f.Close()
	ws.NoteOpened(st)
	if replaced, err := ws.Write("out.tif", []byte("2")); err != nil || !replaced {
		t.Fatalf("save again after reopening: %v %v", replaced, err)
	}
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Fatalf("temporary file left: %s", e.Name())
		}
	}
}

// TestWorkspaceTempFiles: temporary files are created exclusively under
// random names, so a link planted at a predictable temporary name (here the
// old "<name>.tmp-<pid>-<n>" scheme) is never followed into a protected
// input (review of PR #6).
func TestWorkspaceTempFiles(t *testing.T) {
	root := t.TempDir()
	mkfile(t, filepath.Join(root, "scan.tif"), "pixels")
	ws, err := OpenWorkspace(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	f, _, err := ws.Open("scan.tif")
	if err != nil {
		t.Fatal(err)
	}
	st, _ := f.Stat()
	_ = f.Close()
	ws.NoteOpened(st)
	for n := 1; n <= 4; n++ {
		if err := os.Symlink("scan.tif", filepath.Join(root, fmt.Sprintf("out.json.tmp-%d-%d", os.Getpid(), n))); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 4; i++ {
		if _, err := ws.Write("out.json", []byte(`{"ok":true}`)); err != nil {
			t.Fatal(err)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(root, "scan.tif")); string(b) != "pixels" {
		t.Fatalf("scan.tif overwritten through a planted temp link: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "out.json")); string(b) != `{"ok":true}` {
		t.Fatalf("out.json is %q", b)
	}
}
