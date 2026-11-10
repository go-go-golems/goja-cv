package cvmod_test

import (
	"strings"
	"testing"

	cvmod "github.com/go-go-golems/goja-cv/pkg/gojamod/cv"
)

func TestHelpNeverFailsAndResolvesEveryForm(t *testing.T) {
	reg := cvmod.DefaultRegistry()
	for name, key := range map[string]string{
		"levels": "levels", "mask.box": "mask.box", "cv.compare": "compare", "Image.coverage": "coverage",
		"mask": "mask", "Mask": "mask", "regions": "regions", "Regions": "regions",
		"explain": "explain", "cv.explain": "explain",
		"docs": "docs", "open": "open", "look": "look",
	} {
		h := cvmod.Help(reg, name)
		if _, ok := h[key]; !ok {
			t.Fatalf("help(%q) = %v, want key %q", name, h, key)
		}
	}
	for _, k := range []string{"Image", "Value", "image"} {
		h := cvmod.Help(reg, k)
		if h["kind"] == nil || len(h["methods"].(map[string]any)) == 0 {
			t.Fatalf("help(%q) = %v", k, h)
		}
	}
	if m := cvmod.Help(reg, "Image")["methods"].(map[string]any); m["coverage"] == nil || m["feather"] == nil {
		t.Fatalf("the mask ops are Image methods: %v", m)
	}
	// Mask and Regions are concepts: Mask.coverage and Regions.filter point at them
	for name, want := range map[string][]any{"Regions.filter": {"regions"}, "Mask.coverage": {"coverage", "mask"}, "Regions.heal": {"heal", "regions"}} {
		h := cvmod.Help(reg, name)
		if h["found"] != false || strings.Join(stringsOf(h["didYouMean"]), ",") != strings.Join(stringsOf(want), ",") {
			t.Fatalf("help(%q) = %v", name, h)
		}
	}
	if fix := cvmod.Help(reg, "Regions.filter")["fix"]; fix != "(await regions).filter(r => r.score >= 0.5)" {
		t.Fatalf("Regions.filter fix %v", fix)
	}
	for name, want := range map[string]string{
		"mask.stats":   "stats",
		"gausian":      "gaussian",
		"Regions.heal": "heal",
		"exposre":      "exposure",
	} {
		h := cvmod.Help(reg, name)
		dym, _ := h["didYouMean"].([]any)
		if h["found"] != false || len(dym) == 0 || dym[0] != want || h["hint"] == "" {
			t.Fatalf("help(%q) = %v, want didYouMean[0] %q", name, h, want)
		}
	}
	if hint := cvmod.Help(reg, "mask.stats")["hint"].(string); !strings.Contains(hint, "img.stats({ mask: m })") {
		t.Fatalf("mask.stats hint %q", hint)
	}
	if fix := cvmod.Help(reg, "Regions.heal")["fix"]; fix != "img.heal(regions, …)" {
		t.Fatalf("Regions.heal fix %v", fix)
	}
	if h := cvmod.Help(reg, "frobnicate"); h["found"] != false || h["name"] != "frobnicate" {
		t.Fatalf("frobnicate %v", h)
	}
	if h := cvmod.Help(reg, ""); h["docs"] == nil || h["functions"] == nil {
		t.Fatalf("overview %v", h)
	}
}

// Opening, closing and top-hat are compositions of erode and dilate, found
// by cv.help as concept entries (GCV-005 D19); grow and shrink are gone.
func TestHelpMorphologyConcepts(t *testing.T) {
	reg := cvmod.DefaultRegistry()
	for name, want := range map[string]string{
		"opening": "x.erode(r).dilate(r)",
		"closing": "x.dilate(r).erode(r)",
		"tophat":  "x.sub(x.erode(r).dilate(r))",
	} {
		e, ok := cvmod.Help(reg, name)[name].(map[string]any)
		if !ok || !strings.Contains(e["usage"].(string), want) {
			t.Fatalf("help(%q) = %v, want usage with %q", name, e, want)
		}
	}
	for _, op := range []string{"erode", "dilate"} {
		e, ok := cvmod.Help(reg, op)[op].(map[string]any)
		if !ok || !strings.Contains(e["doc"].(string), "disk") {
			t.Fatalf("help(%q) = %v", op, e)
		}
	}
	for _, old := range []string{"grow", "shrink"} {
		if h := cvmod.Help(reg, old); h["found"] != false {
			t.Fatalf("help(%q) = %v, want found: false", old, h)
		}
	}
}

func stringsOf(v any) []string {
	l, _ := v.([]any)
	out := make([]string, len(l))
	for i, x := range l {
		out[i], _ = x.(string)
	}
	return out
}
