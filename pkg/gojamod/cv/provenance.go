package cvmod

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/pkg/errors"
)

// Provenance sidecars (GCV-006 §12.6, Phase 6). Every cv.save writes
// <file>.provenance.json beside the file: what made it (goja-cv's version and
// revision, the session and eval, the script line of the save), what it is
// (the saved bytes' SHA-256), and what it came from (the node's path and
// signature, and every source file under the node with its size, mtime and
// SHA-256). Plain data has no node; its sidecar lists the sources the session
// had opened and says so. One function, stampSave, builds every sidecar, so a
// new kind of save cannot forget it.

// ProvenanceSuffix is appended to a saved file's name for its sidecar.
const ProvenanceSuffix = ".provenance.json"

// ProvenanceSchema names the sidecar's format.
const ProvenanceSchema = "goja-cv/provenance/1"

// Provenance is the sidecar of one saved file.
type Provenance struct {
	Schema  string             `json:"schema"`
	File    string             `json:"file"`
	Bytes   int                `json:"bytes"`
	SHA256  string             `json:"sha256"`
	At      string             `json:"at"`
	GojaCV  ProvenanceBuild    `json:"goja_cv"`
	Session string             `json:"session"`
	Eval    int                `json:"eval"`
	Site    ProvenanceSite     `json:"site"`
	Node    *ProvenanceNode    `json:"node,omitempty"`
	Data    string             `json:"data,omitempty"` // "plain": saved from JavaScript values, not a node
	Sources []ProvenanceSource `json:"sources"`
}

// ProvenanceBuild identifies the goja-cv that wrote the file.
type ProvenanceBuild struct {
	Module      string `json:"module"`
	Version     string `json:"version"`
	VCSRevision string `json:"vcs_revision,omitempty"`
	VCSModified bool   `json:"vcs_modified,omitempty"`
}

// ProvenanceSite is the script position of the cv.save call (the agent's
// line for eval code; the script's own line for a loaded file).
type ProvenanceSite struct {
	Script string `json:"script,omitempty"`
	Line   int    `json:"line,omitempty"`
}

// ProvenanceNode is the saved node: its path of labels and its cache
// signature at scale 1 (equal signatures mean equal values).
type ProvenanceNode struct {
	Path      []string `json:"path"`
	Signature string   `json:"signature"`
}

// ProvenanceSource is one source file the saved value depends on.
type ProvenanceSource struct {
	Name   string `json:"name"` // as the agent opened it
	Path   string `json:"path"` // absolute
	Size   int64  `json:"size"`
	MTime  string `json:"mtime"`
	SHA256 string `json:"sha256"`
}

// provenanceState is the binding's provenance bookkeeping.
type provenanceState struct {
	session string
	evals   int
	mu      sync.Mutex
	hashes  map[string]string // source stamp key → SHA-256 (hashed once per session)
	mapSite func(Site) Site   // set by the session layer: runtime lines → agent lines
}

func newProvenanceState() *provenanceState {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return &provenanceState{session: hex.EncodeToString(b[:]), hashes: map[string]string{}}
}

// SetSiteMapper installs fn to map a save's runtime position to the position
// the agent knows (cvsession maps eval-cell lines past its wrapper).
func SetSiteMapper(vm *goja.Runtime, fn func(Site) Site) {
	if b, ok := lookupBinding(vm); ok {
		b.prov.mapSite = fn
	}
}

// agentSite is site as the agent knows it (its own line in eval code).
func (b *binding) agentSite(site Site) Site {
	if b.prov.mapSite != nil {
		return b.prov.mapSite(site)
	}
	return site
}

// pendingProvenance is what the owner goroutine knows about a save; the save
// goroutine completes it with the bytes and the source hashes.
type pendingProvenance struct {
	rec     Provenance
	sources []graph.SourceMeta
}

// beginProvenance collects, on the owner goroutine, everything about a save
// that needs the graph or the runtime: site, eval, node path, signature and
// the sources under the node (or, for plain data, the sources the session
// opened).
func (b *binding) beginProvenance(rel string, site Site, node *graph.Node) *pendingProvenance {
	site = b.agentSite(site)
	p := &pendingProvenance{rec: Provenance{
		Schema: ProvenanceSchema, File: rel, GojaCV: buildInfo(), Session: b.prov.session, Eval: b.prov.evals,
		Site: ProvenanceSite{Script: site.Source, Line: site.Line},
	}}
	var roots []graph.NodeID
	if node != nil {
		sig := b.eng.Evaluator().Signature(node.ID, 1)
		p.rec.Node = &ProvenanceNode{Path: b.eng.Path(node.ID), Signature: hex.EncodeToString(sig[:])}
		roots = []graph.NodeID{node.ID}
	} else {
		p.rec.Data = "plain: saved from JavaScript values; sources are those the session had opened"
		b.mu.Lock()
		roots = append(roots, b.opened...)
		b.mu.Unlock()
	}
	seen := map[graph.NodeID]bool{}
	have := map[string]bool{}
	var walk func(id graph.NodeID)
	walk = func(id graph.NodeID) {
		if seen[id] {
			return
		}
		seen[id] = true
		n, ok := b.eng.Node(id)
		if !ok {
			return
		}
		if _, isSource := graph.SourceInfo(n); isSource {
			m := graph.Meta[graph.SourceMeta](n.Args)
			if k := sourceHashKey(m); !have[k] {
				have[k] = true
				p.sources = append(p.sources, m)
			}
		}
		for _, in := range n.Inputs {
			walk(in)
		}
	}
	for _, id := range roots {
		walk(id)
	}
	return p
}

// stampSave completes the sidecar of a save whose bytes are data and writes
// it beside rel. It runs on the save goroutine; a sidecar that cannot be
// written fails the save, since a saved file without provenance cannot be
// traced.
func (b *binding) stampSave(p *pendingProvenance, data []byte) (string, error) {
	sum := sha256.Sum256(data)
	p.rec.Bytes, p.rec.SHA256 = len(data), hex.EncodeToString(sum[:])
	p.rec.At = time.Now().UTC().Format(time.RFC3339)
	ws := b.eng.Workspace()
	p.rec.Sources = []ProvenanceSource{}
	for _, m := range p.sources {
		src := ProvenanceSource{Name: m.Name, Path: ws.PathOf(m.Loc)}
		if fi, err := os.Stat(src.Path); err == nil {
			src.Size, src.MTime = fi.Size(), fi.ModTime().UTC().Format(time.RFC3339Nano)
		}
		h, err := b.sourceHash(m)
		if err != nil {
			return "", err
		}
		src.SHA256 = h
		p.rec.Sources = append(p.rec.Sources, src)
	}
	out, err := json.MarshalIndent(p.rec, "", "  ")
	if err != nil {
		return "", errors.Wrap(err, "provenance")
	}
	name := p.rec.File + ProvenanceSuffix
	if _, err := ws.Write(name, append(out, '\n')); err != nil {
		return "", err
	}
	return name, nil
}

// sourceHash is the SHA-256 of a source file, read once per session per
// file version (its location and stamp).
func (b *binding) sourceHash(m graph.SourceMeta) (string, error) {
	k := sourceHashKey(m)
	b.prov.mu.Lock()
	h, ok := b.prov.hashes[k]
	b.prov.mu.Unlock()
	if ok {
		return h, nil
	}
	f, err := b.eng.Workspace().OpenAt(m.Loc)
	if err != nil {
		return "", graph.Errorf(graph.EEval, "provenance: cannot read source %s: %v", m.Name, err)
	}
	defer func() { _ = f.Close() }()
	d := sha256.New()
	if _, err := io.Copy(d, f); err != nil {
		return "", graph.Errorf(graph.EEval, "provenance: cannot hash source %s: %v", m.Name, err)
	}
	h = hex.EncodeToString(d.Sum(nil))
	b.prov.mu.Lock()
	b.prov.hashes[k] = h
	b.prov.mu.Unlock()
	return h, nil
}

func sourceHashKey(m graph.SourceMeta) string {
	return strings.Join([]string{strconv.Itoa(m.Loc.Root), m.Loc.Rel, m.Stamp}, "\x00")
}

var (
	buildOnce sync.Once
	build     ProvenanceBuild
)

// buildInfo is the module, version and VCS revision of the running binary
// (from the Go build information; empty revision in tests).
func buildInfo() ProvenanceBuild {
	buildOnce.Do(func() {
		build = ProvenanceBuild{Module: "github.com/go-go-golems/goja-cv", Version: Version}
		bi, ok := debug.ReadBuildInfo()
		if !ok {
			return
		}
		if bi.Main.Path == build.Module && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			build.Version = bi.Main.Version
		}
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				build.VCSRevision = s.Value
			case "vcs.modified":
				build.VCSModified = s.Value == "true"
			}
		}
	})
	return build
}
