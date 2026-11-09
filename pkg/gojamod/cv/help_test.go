package cvmod_test

import (
	"strings"
	"testing"

	cvmod "github.com/go-go-golems/goja-cv/pkg/gojamod/cv"
)

func TestHelpNeverFailsAndResolvesEveryForm(t *testing.T) {
	reg := cvmod.DefaultRegistry()
	for name, key := range map[string]string{
		"levels": "levels", "mask.box": "mask.box", "cv.compare": "compare", "Mask.coverage": "coverage",
		"Regions.filter": "filter", "explain": "explain", "cv.explain": "explain",
		"docs": "docs", "open": "open", "look": "look",
	} {
		h := cvmod.Help(reg, name)
		if _, ok := h[key]; !ok {
			t.Fatalf("help(%q) = %v, want key %q", name, h, key)
		}
	}
	for _, k := range []string{"Image", "Mask", "Regions", "Value", "mask"} {
		h := cvmod.Help(reg, k)
		if h["kind"] == nil || len(h["methods"].(map[string]any)) == 0 {
			t.Fatalf("help(%q) = %v", k, h)
		}
	}
	if m := cvmod.Help(reg, "Mask")["methods"].(map[string]any); m["coverage"] == nil {
		t.Fatalf("Mask methods %v", m)
	}
	if on := cvmod.Help(reg, "Regions.where")["where"].(map[string]any)["on"]; on != "Regions" {
		t.Fatalf("Regions.where on = %v", on)
	}
	for name, want := range map[string]string{
		"mask.stats":   "stats",
		"blurr":        "gaussian",
		"Regions.heal": "heal",
		"brightness":   "exposure",
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
