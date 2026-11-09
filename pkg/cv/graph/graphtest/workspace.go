package graphtest

import (
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// Workspace is a test workspace: a new temporary directory, with the whole
// filesystem as a read directory, so tests open fixtures anywhere by
// absolute path and save into the temporary directory.
func Workspace(t testing.TB) *graph.Workspace {
	t.Helper()
	ws, err := graph.OpenWorkspace(t.TempDir(), []string{"/"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ws.Close)
	return ws
}

// Options are engine options with a test workspace.
func Options(t testing.TB) graph.EngineOptions {
	t.Helper()
	return graph.EngineOptions{Workspace: Workspace(t)}
}
