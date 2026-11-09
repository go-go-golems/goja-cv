package dataout

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
)

// JSON is written by encoding/json (GCV-004 §7.9): strings round-trip,
// records keep their key order, NaN and ±Inf become null.
func TestEncodeJSONRoundTripsOrderAndNaN(t *testing.T) {
	odd := "a\x00b c\"d\\e\n<f>"
	rec := graph.Record{Keys: []string{"z", "a", "m"}, Vals: map[string]any{"z": 1.0, "a": odd, "m": math.NaN()}}
	v := []any{rec, map[string]any{"b": math.Inf(1), "a": []any{math.Inf(-1), 2.5}}, graph.Record{Keys: []string{"n"}, Vals: map[string]any{"n": rec}}}
	out, rows, err := encodeJSON(v)
	if err != nil {
		t.Fatal(err)
	}
	if rows != 3 {
		t.Fatalf("rows %d", rows)
	}
	s := string(out)
	if !strings.HasPrefix(s, `[{"z":1,"a":`) || !strings.Contains(s, `"m":null}`) || !strings.Contains(s, `{"a":[null,2.5],"b":null}`) ||
		!strings.Contains(s, `{"n":{"z":1,`) {
		t.Fatalf("order or nulls: %s", s)
	}
	var back []map[string]any
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, s)
	}
	if back[0]["a"] != odd {
		t.Fatalf("string round trip: %q, want %q", back[0]["a"], odd)
	}
	if strings.Contains(s, `\x00`) {
		t.Fatalf("Go escape in JSON: %s", s)
	}
}

func TestCSVCellsUsePlain(t *testing.T) {
	out, rows, err := encodeCSV([]any{map[string]any{"a": math.NaN(), "b": []any{1.0, math.Inf(1)}}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rows != 1 || string(out) != "a,b\n,\"[1,null]\"\n" {
		t.Fatalf("csv %d rows: %q", rows, out)
	}
}
