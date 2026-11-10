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
	out, rows, err := encodeCSV("x.csv", []any{map[string]any{"a": math.NaN(), "b": []any{1.0, math.Inf(1)}}})
	if err != nil {
		t.Fatal(err)
	}
	if rows != 1 || string(out) != "a,b\n,\"[1,null]\"\n" {
		t.Fatalf("csv %d rows: %q", rows, out)
	}
}

// CSV is a list of rows (GCV-004 §7.9): columns in first-seen order, records
// in their key order; anything else is E_ARG_TYPE with the fix, the rows of
// a value that has them.
func TestCSVRowsOnly(t *testing.T) {
	rows := []any{graph.Record{Keys: []string{"id", "x"}, Vals: map[string]any{"id": 1.0, "x": 2.0}}, map[string]any{"z": "a", "id": 2.0}}
	out, n, err := encodeCSV("r.csv", rows)
	if err != nil || n != 2 || string(out) != "id,x,z\n1,2,\n2,,a\n" {
		t.Fatalf("csv %d %v: %q", n, err, out)
	}
	for _, tc := range []struct {
		v   any
		fix string
	}{
		{map[string]any{"meanL": 0.5}, `cv.save("t.json", x)`},
		{map[string]any{"from": []any{0.0, 0.0}, "rows": []any{map[string]any{"t": 0.0}}}, `cv.save("t.csv", (await x).rows)`},
		{[]any{[]any{1.0, 2.0}}, `cv.save("t.json", x)`},
		{3.0, `cv.save("t.json", x)`},
	} {
		_, _, err := encodeCSV("t.csv", tc.v)
		ce, ok := graph.AsError(err)
		if !ok || ce.Code != graph.EArgType || ce.Fix != tc.fix {
			t.Fatalf("%v: %v", tc.v, err)
		}
	}
}
