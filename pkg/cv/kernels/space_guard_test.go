package kernels

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forbiddenConversions are the buffer functions that convert between
// spaces. Kernels and ops are space-agnostic: the graph inserts convert nodes
// (GCV-001 §7.1), so none of these may be called from their non-test files.
var forbiddenConversions = map[string]bool{
	"ToPerceptual": true, "ToLinear": true,
	"Linear8": true, "Linear16": true,
	"EncodeSRGB8": true, "EncodeSRGB16": true,
}

func TestKernelsAreSpaceAgnostic(t *testing.T) {
	for _, dir := range []string{".", filepath.Join("..", "ops")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(dir, name)
			fs := token.NewFileSet()
			f, err := parser.ParseFile(fs, path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(f, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "buffer" && forbiddenConversions[sel.Sel.Name] {
					t.Errorf("%s: buffer.%s converts between spaces; kernels and ops must receive converted inputs from the graph", fs.Position(sel.Pos()), sel.Sel.Name)
				}
				return true
			})
		}
	}
}
