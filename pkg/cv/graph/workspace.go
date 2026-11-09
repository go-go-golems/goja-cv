package graph

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// The workspace (GCV-004 §4.6, §7.8, D11): one read-write directory (--root)
// that cv.open and cv.save resolve relative paths in, plus read-only
// directories (--read) cv.open also reads from. Each is a Go os.Root, so
// the standard library refuses any path, .. or symlink that leaves it. A
// save never follows a symlink and never replaces a file the session opened
// (and did not itself write), compared by file identity.

// EAccess is the error code of a path outside the workspace and the read
// directories, a save outside the workspace, through a symlink, or over a
// file the session opened.
const EAccess = "E_ACCESS"

// EDuplicate is the error of saving one path twice in one eval.
const EDuplicate = "E_DUPLICATE"

// Workspace is the directory a session writes and reads relative paths in,
// plus read-only directories.
type Workspace struct {
	root  dirRoot   // --root: read and write
	reads []dirRoot // --read: read only

	mu      sync.Mutex
	opened  []os.FileInfo // files cv.open read (protected from saves)
	written []os.FileInfo // files cv.save wrote (may be saved again)
	tmp     atomic.Int64
}

type dirRoot struct {
	r        *os.Root
	dir      string // as given, cleaned and absolute
	resolved string // dir with symlinks resolved (macOS: /tmp → /private/tmp)
}

// Location names a file inside one of the roots: 0 the workspace, 1… the
// read directories in flag order, and the name relative to that root.
type Location struct {
	Root int    `json:"root"`
	Rel  string `json:"rel"`
}

func openDir(dir, what string) (dirRoot, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dirRoot{}, fmt.Errorf("%s %q: %w", what, dir, err)
	}
	abs = filepath.Clean(abs)
	st, err := os.Stat(abs)
	if err != nil {
		return dirRoot{}, fmt.Errorf("%s %q: %w", what, dir, err)
	}
	if !st.IsDir() {
		return dirRoot{}, fmt.Errorf("%s %q is not a directory", what, dir)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return dirRoot{}, fmt.Errorf("%s %q: %w", what, dir, err)
	}
	r, err := os.OpenRoot(abs)
	if err != nil {
		return dirRoot{}, fmt.Errorf("%s %q: %w", what, dir, err)
	}
	return dirRoot{r: r, dir: abs, resolved: resolved}, nil
}

// OpenWorkspace opens root (read and write) and reads (read only, in flag
// order). Each must be an existing directory.
func OpenWorkspace(root string, reads []string) (*Workspace, error) {
	w := &Workspace{}
	var err error
	if w.root, err = openDir(root, "workspace"); err != nil {
		return nil, err
	}
	for _, d := range reads {
		r, err := openDir(d, "read directory")
		if err != nil {
			w.Close()
			return nil, err
		}
		w.reads = append(w.reads, r)
	}
	return w, nil
}

// Close releases the directory handles.
func (w *Workspace) Close() {
	if w == nil {
		return
	}
	if w.root.r != nil {
		_ = w.root.r.Close()
	}
	for _, r := range w.reads {
		_ = r.r.Close()
	}
}

// Dir is the workspace directory (absolute).
func (w *Workspace) Dir() string { return w.root.dir }

// Reads are the read directories (absolute), in flag order.
func (w *Workspace) Reads() []string {
	out := make([]string, len(w.reads))
	for i, r := range w.reads {
		out[i] = r.dir
	}
	return out
}

func (w *Workspace) at(i int) dirRoot {
	if i == 0 {
		return w.root
	}
	return w.reads[i-1]
}

// Name is how a location is shown: its workspace-relative name, or the
// absolute name in a read directory.
func (w *Workspace) Name(l Location) string {
	if l.Root == 0 {
		return l.Rel
	}
	return filepath.Join(w.at(l.Root).dir, l.Rel)
}

func (w *Workspace) rootsText(forWrite bool) string {
	s := "the workspace " + w.root.dir
	if !forWrite && len(w.reads) > 0 {
		s += " and the read directories " + strings.Join(w.Reads(), ", ")
	}
	return s
}

// relIn is p relative to base when p lies in it.
func relIn(base, p string) (string, bool) {
	r, err := filepath.Rel(base, p)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) || filepath.IsAbs(r) {
		return "", false
	}
	return r, true
}

// locate maps a path an agent wrote to a root and a name in it (§10.5): a
// relative path is in the workspace (for a write), or in the first root
// that has it (for a read: the workspace, then the read directories); an
// absolute path must lie in a root, written with its own or its resolved
// name.
func (w *Workspace) locate(p string, forWrite bool) (Location, error) {
	if strings.TrimSpace(p) == "" {
		return Location{}, Errorf(EArgType, "a file path is empty.").WithFix(`cv.open("scan.tif")`)
	}
	n := len(w.reads) + 1
	if forWrite {
		n = 1
	}
	if !filepath.IsAbs(p) {
		rel := filepath.Clean(p)
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return Location{}, Errorf(EAccess, "%q leaves %s.", p, w.rootsText(forWrite)).
				WithHint("Relative paths are names inside the workspace; .. may not leave it.").
				WithFix(fmt.Sprintf("%q", filepath.Base(p)))
		}
		if forWrite {
			return Location{Root: 0, Rel: rel}, nil
		}
		for i := 0; i < n; i++ {
			_, err := w.at(i).r.Stat(rel)
			if err == nil {
				return Location{Root: i, Rel: rel}, nil
			}
			if !errors.Is(err, fs.ErrNotExist) {
				return Location{}, accessError(p, err)
			}
		}
		return Location{}, Errorf(EArgType, "no such file %q in %s.", p, w.rootsText(false)).
			WithHint("Relative paths are looked up in the workspace, then in each read directory; absolute paths must lie in one of them.")
	}
	ap := filepath.Clean(p)
	for i := 0; i < n; i++ {
		d := w.at(i)
		for _, base := range []string{d.dir, d.resolved} {
			if rel, ok := relIn(base, ap); ok {
				return Location{Root: i, Rel: rel}, nil
			}
		}
	}
	e := Errorf(EAccess, "%q is outside %s.", p, w.rootsText(forWrite))
	if forWrite {
		e.Hint = "cv.save writes only in the workspace; read directories are read-only."
		e.Fix = fmt.Sprintf("cv.save(%q, x)", filepath.Base(p))
	} else {
		e.Hint = "Start goja-cv with --read DIR for a directory to read from; a path through a symlink must be written with the directory's own name."
	}
	return Location{}, e
}

func accessError(p string, err error) *Error {
	return Errorf(EAccess, "%q: %v.", p, unwrapPath(err)).
		WithHint("Paths and symlinks may not leave the workspace or a read directory.")
}

func unwrapPath(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}

// Open opens a file for reading (cv.open, decoders).
func (w *Workspace) Open(p string) (*os.File, Location, error) {
	l, err := w.locate(p, false)
	if err != nil {
		return nil, Location{}, err
	}
	f, err := w.OpenAt(l)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, Location{}, Errorf(EArgType, "no such file %q in %s.", p, w.rootsText(false))
		}
		return nil, Location{}, accessError(p, err)
	}
	return f, l, nil
}

// OpenAt opens the file at a location.
func (w *Workspace) OpenAt(l Location) (*os.File, error) {
	if l.Root < 0 || l.Root > len(w.reads) {
		return nil, Errorf(EInternal, "no root %d", l.Root)
	}
	return w.at(l.Root).r.Open(l.Rel)
}

// NoteOpened records a file the session opened: saves may not replace it.
func (w *Workspace) NoteOpened(fi os.FileInfo) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.opened = append(w.opened, fi)
}

// SaveName is the workspace-relative name cv.save writes p to (E_ACCESS
// outside the workspace).
func (w *Workspace) SaveName(p string) (string, error) {
	l, err := w.locate(p, true)
	if err != nil {
		return "", err
	}
	if l.Rel == "." {
		return "", Errorf(EArgType, "cv.save needs a file name, not the workspace itself.").WithFix(`cv.save("x.json", data)`)
	}
	return l.Rel, nil
}

// protected reports whether fi is a file the session opened and did not
// write itself.
func (w *Workspace) protected(fi os.FileInfo) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, x := range w.written {
		if os.SameFile(fi, x) {
			return false
		}
	}
	for _, x := range w.opened {
		if os.SameFile(fi, x) {
			return true
		}
	}
	return false
}

// Write writes data to rel (from SaveName) in the workspace: never through
// a symlink, never over a file the session opened and did not write,
// through a temporary name renamed into place (a failed save leaves the old
// file). replaced reports whether a file was there.
func (w *Workspace) Write(rel string, data []byte) (bool, error) {
	r := w.root.r
	fi, err := r.Lstat(rel)
	replaced := err == nil
	switch {
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return false, accessError(rel, err)
	case replaced && fi.Mode()&os.ModeSymlink != 0:
		return false, Errorf(EAccess, "cv.save(%q): saves do not follow links; %s is a symlink.", rel, rel).
			WithHint("Remove the link or save under another name.")
	case replaced && fi.IsDir():
		return false, Errorf(EArgType, "cv.save(%q): %s is a directory.", rel, rel).WithFix(fmt.Sprintf("cv.save(%q, x)", filepath.Join(rel, "x.json")))
	case replaced && w.protected(fi):
		return false, Errorf(EAccess, "cv.save(%q) would replace %s, which this session opened.", rel, rel).
			WithHint("Saves never overwrite an input of the session; save under another name.").
			WithFix(fmt.Sprintf("cv.save(%q, x)", strings.TrimSuffix(rel, filepath.Ext(rel))+"-out"+filepath.Ext(rel)))
	}
	if dir := filepath.Dir(rel); dir != "." {
		if err := r.MkdirAll(dir, 0o755); err != nil {
			return false, accessError(rel, err)
		}
	}
	tmp := fmt.Sprintf("%s.tmp-%d-%d", rel, os.Getpid(), w.tmp.Add(1))
	if err := r.WriteFile(tmp, data, 0o644); err != nil { // #nosec G306 -- files the agent saves for people and programs
		_ = r.Remove(tmp)
		return false, Errorf(EEval, "cv.save(%q): %v", rel, unwrapPath(err))
	}
	if err := r.Rename(tmp, rel); err != nil {
		_ = r.Remove(tmp)
		return false, Errorf(EEval, "cv.save(%q): %v", rel, unwrapPath(err))
	}
	if st, err := r.Lstat(rel); err == nil {
		w.mu.Lock()
		w.written = append(w.written, st)
		w.mu.Unlock()
	}
	return replaced, nil
}
