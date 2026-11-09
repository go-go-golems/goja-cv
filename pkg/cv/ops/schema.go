package ops

import "github.com/go-go-golems/goja-cv/pkg/cv/graph"

// Field schemas of the ops' results (graph.FieldSpec, GCV-003 §7.5). One
// list per result: the items JavaScript receives, where/sortBy, help and CSV
// columns all read it.

func numField(name string, unit graph.Unit, doc string) graph.FieldSpec {
	return graph.FieldSpec{Name: name, Type: "number", Unit: unit, Doc: doc}
}

func strField(name, doc string) graph.FieldSpec {
	return graph.FieldSpec{Name: name, Type: "string", Doc: doc}
}

// fixedFields is an OpDef.Fields that does not depend on the call.
func fixedFields(fs []graph.FieldSpec) func(graph.Args, []*graph.Node) []graph.FieldSpec {
	return func(graph.Args, []*graph.Node) []graph.FieldSpec { return fs }
}

// geometryFields is the schema of components: geometry only.
var geometryFields = fixedFields(graph.GeometryFields)

// speckFields is the schema of detect.specks.
var speckFields = graph.WithFields(graph.GeometryFields,
	numField("contrast", graph.Level, "peak contrast against the local background, levels of the working space"),
	numField("localNoise", graph.Level, "noise (σ) of the background around the speck"),
	numField("sharpness", graph.Unitless, "how compact the peak is (soft round blobs such as stars are low)"),
	numField("score", graph.Unitless, "dust likelihood in [0, 1]; ≥ 0.5 counts as dust"),
	numField("meanL", graph.Level, "mean luminance inside the speck"),
	strField("polarity", `"dark" or "bright"`),
)

// speckProduces are the fields detect.specks adds to geometry.
var speckProduces = map[string]string{
	"contrast": "img.detect.specks()", "localNoise": "img.detect.specks()", "sharpness": "img.detect.specks()",
	"score": "img.detect.specks()", "polarity": "img.detect.specks()",
}

var statsBaseFields = []graph.FieldSpec{
	numField("meanL", graph.Level, "mean luminance (Rec. 709 weights)"),
	numField("medianL", graph.Level, "median luminance"),
	numField("p01", graph.Level, "1st percentile of luminance"),
	numField("p99", graph.Level, "99th percentile of luminance"),
	numField("clipLow", graph.Fraction, "fraction of pixels with a channel at 0"),
	numField("clipHigh", graph.Fraction, "fraction of pixels with a channel at 1"),
	{Name: "percentiles", Type: "object", Doc: "requested luminance percentiles by fraction"},
	{Name: "perChannel", Type: "object", Doc: "the same per channel (r, g, b, or gray)"},
	numField("w", graph.Unitless, "buffer width measured (buffer px)"),
	numField("h", graph.Unitless, "buffer height measured (buffer px)"),
	numField("scale", graph.Unitless, "evaluation scale"),
}

var statsSelectionFields = []graph.FieldSpec{
	numField("area", graph.DocPx2, "selected area"),
	numField("coverage", graph.Fraction, "selected fraction of the image"),
}

// statsFields: area and coverage only with a mask or a box.
func statsFields(a graph.Args, _ []*graph.Node) []graph.FieldSpec {
	if a.Has("mask") || a.Has("box") {
		return graph.WithFields(statsBaseFields, statsSelectionFields...)
	}
	return statsBaseFields
}

var histogramFields = []graph.FieldSpec{
	numField("bins", graph.Unitless, "number of bins"),
	strField("channel", "L, r, g or b"),
	strField("space", "working space of the levels"),
	{Name: "counts", Type: "number[]", Doc: "pixels per bin over [0, 1]"},
	numField("under", graph.Unitless, "pixels below 0"),
	numField("over", graph.Unitless, "pixels above 1"),
	numField("total", graph.Unitless, "pixels counted"),
}

var dustFields = []graph.FieldSpec{
	strField("verdict", `"clean", "light" (1–10) or "dusty" (> 10)`),
	numField("count", graph.Unitless, "specks with score ≥ 0.5"),
	numField("candidates", graph.Unitless, "specks found"),
	numField("maxArea", graph.DocPx2, "largest dust speck"),
	numField("coverage", graph.Fraction, "fraction of the image covered by dust"),
	numField("noise", graph.Level, "image noise"),
	{Name: "hint", Type: "string", Optional: true, Doc: "the heal call, when there is dust"},
}
