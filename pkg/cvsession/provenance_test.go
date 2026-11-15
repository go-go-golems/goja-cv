package cvsession_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cvmod "github.com/go-go-golems/goja-cv/pkg/gojamod/cv"
)

func readSidecar(t *testing.T, ws, rel string) cvmod.Provenance {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(ws, rel+cvmod.ProvenanceSuffix))
	if err != nil {
		t.Fatalf("sidecar of %s: %v", rel, err)
	}
	var p cvmod.Provenance
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatalf("sidecar of %s: %v\n%s", rel, err, b)
	}
	return p
}

func fileSHA(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// TestSaveWritesProvenanceSidecar: every save writes <file>.provenance.json
// with the saved bytes' hash, the agent's line, the eval number, the node's
// path and signature, and each source under the node with its hash; the
// save's result names the sidecar.
func TestSaveWritesProvenanceSidecar(t *testing.T) {
	app, sid, ws := saveApp(t)
	mustOK(t, eval(t, app, sid, `return 1`)) // eval 1
	res := mustOK(t, eval(t, app, sid, `const v = img.view("speck")

const a = await cv.save("views/speck.tif", v)
return a.provenance`)) // eval 2; the save is on agent line 3
	mustContain(t, res, "views/speck.tif.provenance.json")
	p := readSidecar(t, ws, "views/speck.tif")
	if p.Schema != cvmod.ProvenanceSchema || p.File != "views/speck.tif" || p.Eval != 2 || p.Site.Line != 3 || p.Site.Script != "" {
		t.Fatalf("record: %+v", p)
	}
	if p.SHA256 != fileSHA(t, filepath.Join(ws, "views", "speck.tif")) || p.Bytes == 0 || p.At == "" || p.Session == "" {
		t.Fatalf("bytes: %+v", p)
	}
	if p.Node == nil || len(p.Node.Signature) != 64 || !strings.HasPrefix(p.Node.Path[0], "open(scan.tif") || p.Data != "" {
		t.Fatalf("node: %+v", p.Node)
	}
	if len(p.Sources) != 1 || p.Sources[0].Name != "scan.tif" || p.Sources[0].SHA256 != fileSHA(t, filepath.Join(ws, "scan.tif")) ||
		p.Sources[0].Size == 0 || p.Sources[0].MTime == "" || !filepath.IsAbs(p.Sources[0].Path) {
		t.Fatalf("sources: %+v", p.Sources)
	}
	if p.GojaCV.Module != "github.com/go-go-golems/goja-cv" || p.GojaCV.Version == "" {
		t.Fatalf("build: %+v", p.GojaCV)
	}
	// the same value saved again has the same signature; another value does not
	mustOK(t, eval(t, app, sid, `await cv.save("again.tif", img.view("speck")); await cv.save("other.tif", img.exposure(1)); return 1`))
	if again, other := readSidecar(t, ws, "again.tif"), readSidecar(t, ws, "other.tif"); again.Node.Signature != p.Node.Signature ||
		other.Node.Signature == p.Node.Signature || again.Eval != 3 {
		t.Fatalf("signatures: %s %s %s", p.Node.Signature, again.Node.Signature, other.Node.Signature)
	}
}

// TestPlainDataProvenanceListsOpenedSources: data saved from JavaScript
// values has no node; its sidecar says so and lists the session's sources.
func TestPlainDataProvenanceListsOpenedSources(t *testing.T) {
	app, sid, ws := saveApp(t)
	mustOK(t, eval(t, app, sid, `const sp = await img.detect.specks()
return (await cv.save("labels.json", sp.slice(0, 3).map(s => ({ x: s.cx, y: s.cy, label: "dust" })))).rows`))
	p := readSidecar(t, ws, "labels.json")
	if p.Node != nil || !strings.HasPrefix(p.Data, "plain") || len(p.Sources) != 1 || p.Sources[0].Name != "scan.tif" || p.Site.Line != 2 {
		t.Fatalf("plain: %+v", p)
	}
	if p.SHA256 != fileSHA(t, filepath.Join(ws, "labels.json")) {
		t.Fatalf("sha: %s", p.SHA256)
	}
}

// TestSaveRefusesSidecarNames: an agent cannot save a file named like a
// sidecar.
func TestSaveRefusesSidecarNames(t *testing.T) {
	app, sid, _ := saveApp(t)
	mustContain(t, text(eval(t, app, sid, `return await cv.save("x.json.provenance.json", 1)`)), "code: E_ARG_TYPE", "sidecars", "x.json-data.json")
}

// TestSidecarFailureKeepsTheOldFile: when the sidecar's name cannot be
// written (here a directory), the save fails before the file is replaced.
func TestSidecarFailureKeepsTheOldFile(t *testing.T) {
	app, sid, ws := saveApp(t)
	mustOK(t, eval(t, app, sid, `return (await cv.save("keep.json", [1])).rows`))
	if err := os.Remove(filepath.Join(ws, "keep.json"+cvmod.ProvenanceSuffix)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(ws, "keep.json"+cvmod.ProvenanceSuffix), 0o755); err != nil {
		t.Fatal(err)
	}
	mustContain(t, text(eval(t, app, sid, `return await cv.save("keep.json", [2, 3])`)), "is a directory")
	if b, _ := os.ReadFile(filepath.Join(ws, "keep.json")); string(b) != "[\n  1\n]\n" && string(b) != "[1]\n" {
		t.Fatalf("keep.json was replaced: %q", b)
	}
}

// TestChangedSourceIsNotHashed: a source replaced after the session read it
// is reported as changed, not hashed as if it were the version the pixels
// came from.
func TestChangedSourceIsNotHashed(t *testing.T) {
	app, sid, ws := saveApp(t)
	mustOK(t, eval(t, app, sid, `return (await img.stats()).meanL`)) // decoded: the pixels are the opened version
	scan := filepath.Join(ws, "scan.tif")
	b, err := os.ReadFile(scan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scan, append(b, 0), 0o600); err != nil { // another version under the same name
		t.Fatal(err)
	}
	mustOK(t, eval(t, app, sid, `return (await cv.save("v.tif", img.view("speck"))).path`))
	p := readSidecar(t, ws, "v.tif")
	if len(p.Sources) != 1 || p.Sources[0].SHA256 != "" || !strings.Contains(p.Sources[0].Changed, "changed after the session opened it") {
		t.Fatalf("sources: %+v", p.Sources)
	}
}
