package graph

import (
	"os"
	"path/filepath"
	"testing"
)

// TestOutPathSymlinks: links inside the out-root must not let save write
// outside it, whether the link's target exists or not (review of PR #4).
func TestOutPathSymlinks(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if p, err := OutPath(root, "a/b.csv"); err != nil || p != filepath.Join(root, "a", "b.csv") {
		t.Fatalf("plain path: %q %v", p, err)
	}
	if _, err := WriteOut(root, filepath.Join(root, "a", "b.csv"), []byte("x")); err != nil {
		t.Fatal(err)
	}
	if _, err := OutPath(root, "../escape.csv"); err == nil {
		t.Fatal("../ accepted")
	}

	// a dangling link to a missing file outside root
	dangling := filepath.Join(outside, "created-by-save.csv")
	if err := os.Symlink(dangling, filepath.Join(root, "dangling.csv")); err != nil {
		t.Skip("symlinks unsupported:", err)
	}
	if _, err := OutPath(root, "dangling.csv"); err == nil {
		t.Fatal("dangling symlink out of the root accepted by OutPath")
	}
	if _, err := WriteOut(root, filepath.Join(root, "dangling.csv"), []byte("x")); err == nil {
		t.Fatal("WriteOut wrote through a dangling symlink")
	}
	if _, err := os.Lstat(dangling); !os.IsNotExist(err) {
		t.Fatalf("file created outside the out-root: %v", err)
	}

	// a relative dangling link, and a link to an existing file outside
	if err := os.Symlink(filepath.Join("..", filepath.Base(outside), "rel.csv"), filepath.Join(root, "rel.csv")); err == nil {
		if _, err := OutPath(root, "rel.csv"); err == nil && filepath.Dir(root) == filepath.Dir(outside) {
			t.Fatal("relative dangling symlink out of the root accepted")
		}
	}
	existing := filepath.Join(outside, "existing.csv")
	if err := os.WriteFile(existing, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(existing, filepath.Join(root, "existing.csv")); err != nil {
		t.Fatal(err)
	}
	if _, err := OutPath(root, "existing.csv"); err == nil {
		t.Fatal("symlink to an existing file outside accepted")
	}

	// a link that stays inside the root is still not written through
	if err := os.Symlink(filepath.Join(root, "a", "b.csv"), filepath.Join(root, "inside.csv")); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteOut(root, filepath.Join(root, "inside.csv"), []byte("y")); err == nil {
		t.Fatal("WriteOut wrote through a symlink")
	}
}
