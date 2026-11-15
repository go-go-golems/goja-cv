package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-go-golems/goja-cv/pkg/cvsession"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// The bench (GCV-006 §12.3, Phase 0) runs scenarios: agent-like sequences of
// evals, each repeated in a fresh session so every repeat starts cold. It
// reports per eval the p50 and p95 wall time over the repeats, the op runs and
// cache hits and misses the eval caused, and the bytes cached after it; per
// scenario, the process's peak resident memory. Proposals that claim to make
// goja-cv faster are accepted on these numbers.

type benchFlags struct {
	image   string
	root    string
	read    []string
	prelude []string
	repeat  int
	timeout int
	json    bool
}

func newBenchCommand(opts *rootOptions) *cobra.Command {
	f := &benchFlags{}
	cmd := &cobra.Command{
		Use:   "bench SCENARIO.js...",
		Short: "Time scenarios of evals, each repeat in a fresh session; YAML report to stdout",
		Long: `bench runs each scenario file --repeat times, every repeat in a fresh session (cold caches).

A scenario is a script split into evals by separator lines "// --- name": each part is the body of
one eval, run in order in the same session, so later evals see the variables and cache the earlier
ones left (keep values on mem: const and let do not survive an eval). Text before the first separator is a
comment and is not run.

Per eval it reports p50 and p95 milliseconds over the repeats, the op runs and cache hits and misses
the eval caused, and the megabytes cached after it; per scenario, the process's peak resident memory
so far. Looks are rendered and discarded.

Generate the 24-megapixel benchmark scan with:
  go run ./internal/testimage/gen -out DIR -bench-scan 6000x4000
and run the shipped scenarios with:
  goja-cv bench --root DIR --repeat 3 examples/bench/*.js`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBench(cmd.Context(), opts, f, args)
		},
	}
	cmd.Flags().StringVar(&f.image, "image", "", "Image file to open and bind to img in every session (in --root or a --read directory)")
	cmd.Flags().Var(&onceString{p: &f.root, name: "root"}, "root", "The workspace (default: the current directory)")
	cmd.Flags().StringArrayVar(&f.read, "read", nil, "A read-only directory cv.open may also read (repeatable)")
	cmd.Flags().StringArrayVar(&f.prelude, "prelude", nil, "Script file loaded in every session before the scenario (repeatable)")
	cmd.Flags().IntVar(&f.repeat, "repeat", 3, "Fresh-session repeats per scenario")
	cmd.Flags().IntVar(&f.timeout, "timeout-ms", 0, "Eval timeout in milliseconds (default 60000)")
	cmd.Flags().BoolVar(&f.json, "json", false, "Print JSON instead of YAML")
	return cmd
}

// benchEval is one eval of a scenario.
type benchEval struct {
	name string
	code string
}

var benchSeparator = regexp.MustCompile(`^//\s*---\s*(.*?)\s*$`)

// parseScenario splits a scenario script into evals at "// --- name" lines.
// Text before the first separator is a comment.
func parseScenario(src string) ([]benchEval, error) {
	var evals []benchEval
	var cur *benchEval
	for _, line := range strings.Split(src, "\n") {
		if m := benchSeparator.FindStringSubmatch(line); m != nil {
			evals = append(evals, benchEval{name: m[1]})
			cur = &evals[len(evals)-1]
			if cur.name == "" {
				cur.name = fmt.Sprintf("eval %d", len(evals))
			}
			continue
		}
		if cur != nil {
			cur.code += line + "\n"
		}
	}
	for i := range evals {
		if strings.TrimSpace(evals[i].code) == "" {
			return nil, errors.Errorf("eval %q has no code", evals[i].name)
		}
	}
	if len(evals) == 0 {
		return nil, errors.New(`no "// --- name" separator: a scenario is split into evals by such lines`)
	}
	return evals, nil
}

// EvalReport is one eval's numbers over the repeats.
type EvalReport struct {
	Name        string  `json:"name" yaml:"name"`
	P50Ms       float64 `json:"p50_ms" yaml:"p50_ms"`
	P95Ms       float64 `json:"p95_ms" yaml:"p95_ms"`
	Runs        int     `json:"runs" yaml:"runs"`
	CacheHits   int64   `json:"cache_hits" yaml:"cache_hits"`
	CacheMisses int64   `json:"cache_misses" yaml:"cache_misses"`
	CachedMB    float64 `json:"cached_mb" yaml:"cached_mb"`
	Status      string  `json:"status" yaml:"status"`
	Error       string  `json:"error,omitempty" yaml:"error,omitempty"`
}

// ScenarioReport is one scenario's evals and memory.
type ScenarioReport struct {
	Name      string       `json:"name" yaml:"name"`
	File      string       `json:"file" yaml:"file"`
	Repeats   int          `json:"repeats" yaml:"repeats"`
	TotalP50  float64      `json:"total_p50_ms" yaml:"total_p50_ms"`
	PeakRSSMB float64      `json:"peak_rss_mb" yaml:"peak_rss_mb"`
	Evals     []EvalReport `json:"evals" yaml:"evals"`
}

// sample is one eval in one repeat.
type sample struct {
	ms               float64
	runs             int
	hits, misses     int64
	cachedBytes      int64
	status, errorMsg string
}

func runBench(ctx context.Context, opts *rootOptions, f *benchFlags, files []string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if f.repeat < 1 {
		return usageErr("--repeat must be at least 1")
	}
	if err := checkRoots(f.root, f.read, f.image, f.prelude); err != nil {
		return err
	}
	var reports []ScenarioReport
	for _, file := range files {
		src, err := os.ReadFile(file) // #nosec G304 -- a scenario named by the operator
		if err != nil {
			return usageErr("read scenario: %v", err)
		}
		evals, err := parseScenario(string(src))
		if err != nil {
			return usageErr("%s: %v", file, err)
		}
		samples := make([][]sample, len(evals)) // [eval][repeat]
		for r := 0; r < f.repeat; r++ {
			got, err := benchRepeat(ctx, opts, f, evals)
			if err != nil {
				return failure(errors.Wrapf(err, "%s, repeat %d", file, r+1))
			}
			for i, s := range got {
				samples[i] = append(samples[i], s)
			}
		}
		reports = append(reports, summarize(file, evals, samples, f.repeat))
	}
	doc := map[string]any{"scenarios": reports}
	if f.json {
		enc := json.NewEncoder(opts.stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(doc)
	}
	b, err := yaml.Marshal(doc)
	if err != nil {
		return failure(err)
	}
	_, err = opts.stdout.Write(b)
	return err
}

// benchRepeat runs every eval of a scenario once, in one fresh session.
func benchRepeat(ctx context.Context, opts *rootOptions, f *benchFlags, evals []benchEval) ([]sample, error) {
	logger := opts.logger()
	sopts := cvsession.Options{Image: f.image, Root: f.root, Read: f.read, Preludes: f.prelude, Logger: &logger}
	if f.timeout > 0 {
		sopts.EvalTimeout = msDuration(f.timeout)
	}
	app, err := cvsession.New(ctx, sopts)
	if err != nil {
		return nil, err
	}
	defer func() { _ = app.Close(context.Background()) }()
	session, err := app.CreateSession(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "start session")
	}
	before, err := app.EngineStats(ctx, session.ID)
	if err != nil {
		return nil, err
	}
	out := make([]sample, len(evals))
	for i, ev := range evals {
		t0 := time.Now()
		resp, err := app.Evaluate(ctx, session.ID, ev.code)
		ms := float64(time.Since(t0)) / float64(time.Millisecond)
		s := sample{ms: ms, status: "ok"}
		switch {
		case err != nil && (resp == nil || resp.Cell == nil):
			s.status, s.errorMsg = "error", err.Error()
		case resp.Cell.Execution.Status != "ok":
			s.status, s.errorMsg = resp.Cell.Execution.Status, firstLine(resp.Cell.Execution.Error)
		}
		after, err := app.EngineStats(ctx, session.ID)
		if err != nil {
			return nil, err
		}
		s.runs = after.Runs - before.Runs
		s.hits = after.Cache.Hits - before.Cache.Hits
		s.misses = after.Cache.Misses - before.Cache.Misses
		s.cachedBytes = after.Cache.Bytes
		out[i], before = s, after
	}
	return out, nil
}

func summarize(file string, evals []benchEval, samples [][]sample, repeats int) ScenarioReport {
	rep := ScenarioReport{
		Name: strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)), File: file, Repeats: repeats,
		PeakRSSMB: round1(float64(peakRSSBytes()) / (1 << 20)),
	}
	for i, ev := range evals {
		ss := samples[i]
		ms := make([]float64, len(ss))
		for j, s := range ss {
			ms[j] = s.ms
		}
		last := ss[len(ss)-1] // counts are the same in every cold repeat; times are not
		er := EvalReport{
			Name: ev.name, P50Ms: round1(percentile(ms, 50)), P95Ms: round1(percentile(ms, 95)),
			Runs: last.runs, CacheHits: last.hits, CacheMisses: last.misses,
			CachedMB: round1(float64(last.cachedBytes) / (1 << 20)), Status: last.status, Error: last.errorMsg,
		}
		for _, s := range ss { // report a failure even if only one repeat failed
			if s.status != "ok" {
				er.Status, er.Error = s.status, s.errorMsg
			}
		}
		rep.TotalP50 += er.P50Ms
		rep.Evals = append(rep.Evals, er)
	}
	rep.TotalP50 = round1(rep.TotalP50)
	return rep
}

// percentile is the nearest-rank percentile of xs (p in (0, 100]).
func percentile(xs []float64, p float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	rank := int(math.Ceil(p / 100 * float64(len(s))))
	if rank < 1 {
		rank = 1
	}
	return s[rank-1]
}

func round1(x float64) float64 { return math.Round(x*10) / 10 }

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
