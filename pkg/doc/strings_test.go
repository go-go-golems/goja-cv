package doc

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// removedNames are calls and options GCV-004 removed. No non-test Go or
// JavaScript source may mention them: an agent only types a name some
// document or error showed it (guide §12).
var removedNames = []string{
	"{ space", "toMask", "cv.csv", "cv.json", "out-root", "P.slider", "show(", "cv.evaluate",
	"regions.measure", "sortBy(", "node.save", "cv.describe", "cv.isNode",
}

func TestNoRemovedNamesInSources(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, dir := range []string{"pkg", "cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			if strings.HasSuffix(p, "_test.go") || (!strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, ".js")) {
				return nil
			}
			b, err := os.ReadFile(p) // #nosec G304 -- the repository's own sources
			if err != nil {
				return err
			}
			for i, line := range strings.Split(string(b), "\n") {
				for _, name := range removedNames {
					if strings.Contains(line, name) {
						t.Errorf("%s:%d mentions the removed %q: %s", p, i+1, name, strings.TrimSpace(line))
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
