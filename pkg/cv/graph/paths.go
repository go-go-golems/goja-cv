package graph

import (
	"os"
	"path/filepath"
	"strings"
)

// EAccess is the error code of a path outside the directories a session
// may read (cv.open with --root) or write (save with --out-root).
const EAccess = "E_ACCESS"

// Within reports whether path p lies inside directory root once both are
// resolved through symlinks. For a path that does not exist yet, the longest
// existing prefix is resolved and the rest appended, so a symlinked parent
// directory pointing outside root is caught before anything is created.
func Within(root, p string) bool {
	r := resolveExisting(filepath.Clean(root))
	t := resolveExisting(filepath.Clean(p))
	rel, err := filepath.Rel(r, t)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// resolveExisting resolves the symlinks of the longest existing prefix of
// the absolute path p and appends the rest. A dangling symlink is followed to
// its target (EvalSymlinks fails on it), so a link inside root pointing to a
// missing file outside root resolves outside root.
func resolveExisting(p string) string { return resolveDepth(p, 0) }

func resolveDepth(p string, depth int) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	if fi, err := os.Lstat(p); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		if depth >= 40 {
			return p // a symlink loop; WriteOut refuses symlinks anyway
		}
		target, err := os.Readlink(p)
		if err == nil {
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(p), target)
			}
			return resolveDepth(filepath.Clean(target), depth+1)
		}
	}
	dir, base := filepath.Split(p)
	dir = filepath.Clean(dir)
	if dir == p || dir == "" || dir == "." {
		return p
	}
	return filepath.Join(resolveDepth(dir, depth), base)
}

// OutPath resolves a path an agent passed to save against the out-root:
// relative paths are under root (never the working directory), absolute ones
// must lie in it. The result is absolute and inside root after symlink
// resolution; anything else is E_ACCESS. An empty root means saving is
// disabled.
func OutPath(root, p string) (string, error) {
	if root == "" {
		return "", Errorf(EAccess, "save(%q): saving is disabled in this session.", p).
			WithHint("Saving writes files only under an out-root: start goja-cv with --out-root DIR (eval, run and mcp).")
	}
	if strings.TrimSpace(p) == "" {
		return "", Errorf(EArgType, "save needs a file path (relative to the out-root).").WithFix(`node.save("views/speck.tif")`)
	}
	abs := p
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, p)
	}
	abs = filepath.Clean(abs)
	if !Within(root, abs) {
		return "", Errorf(EAccess, "save(%q): the path is outside the out-root %s.", p, root).
			WithHint("Paths are relative to the out-root; .. and symlinks may not leave it.").
			WithFix(`node.save("` + filepath.Base(p) + `")`)
	}
	return abs, nil
}

// WriteOut writes data to abs (from OutPath) under root: it creates the
// parent directories, checks containment again (a directory created
// concurrently could be a symlink) and replaces an existing file. replaced
// reports whether a file was there.
func WriteOut(root, abs string, data []byte) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return false, Errorf(EAccess, "save: cannot create %s: %v", filepath.Dir(abs), err)
	}
	if !Within(root, abs) {
		return false, Errorf(EAccess, "save: %s is outside the out-root %s.", abs, root)
	}
	fi, err := os.Lstat(abs)
	replaced := err == nil
	if replaced && fi.Mode()&os.ModeSymlink != 0 {
		// os.WriteFile follows symlinks: never write through one.
		return false, Errorf(EAccess, "save: %s is a symlink; save writes only regular files under the out-root.", abs).
			WithHint("Remove the link or save under another name.")
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil { // #nosec G306 -- files the agent saves for people and programs
		return false, Errorf(EEval, "save: %v", err)
	}
	return replaced, nil
}
