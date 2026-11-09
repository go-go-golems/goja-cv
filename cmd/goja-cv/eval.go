package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-go-golems/go-go-goja/pkg/replsession"
	"github.com/go-go-golems/go-go-goja/pkg/resultcontent"
	"github.com/go-go-golems/goja-cv/pkg/cvsession"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type evalFlags struct {
	image   string
	out     string
	outRoot string
	prelude []string
	json    bool
	timeout int
}

func (f *evalFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.image, "image", "", "Image file to open and bind to img")
	cmd.Flags().StringVar(&f.out, "out", "", "Directory for the delivered looks and data files (default: a new temporary directory)")
	cmd.Flags().StringVar(&f.outRoot, "out-root", "", "The only directory save() may write under (default: saving is disabled)")
	cmd.Flags().StringArrayVar(&f.prelude, "prelude", nil, "Script run in the session before the code (repeatable; e.g. helpers that define mem.*)")
	cmd.Flags().BoolVar(&f.json, "json", false, "Print a JSON object instead of YAML")
	cmd.Flags().IntVar(&f.timeout, "timeout-ms", 0, "Eval timeout in milliseconds (default 60000)")
}

func newRunCommand(opts *rootOptions) *cobra.Command {
	f := &evalFlags{}
	cmd := &cobra.Command{
		Use:   "run SCRIPT.js",
		Short: "Run a cv script once; YAML to stdout, looks written as files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			code, err := os.ReadFile(args[0])
			if err != nil {
				return usageErr("read script: %v", err)
			}
			return runCode(cmd.Context(), opts, f, string(code))
		},
	}
	f.register(cmd)
	return cmd
}

func newEvalCommand(opts *rootOptions) *cobra.Command {
	f := &evalFlags{}
	var code string
	cmd := &cobra.Command{
		Use:   "eval -e CODE",
		Short: "Run inline cv code once; YAML to stdout, looks written as files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(code) == "" {
				return usageErr("eval needs code: goja-cv eval -e 'return await img.stats()' --image scan.tif")
			}
			return runCode(cmd.Context(), opts, f, code)
		},
	}
	cmd.Flags().StringVarP(&code, "eval", "e", "", "Code to run (the body of an async function)")
	f.register(cmd)
	return cmd
}

// contentFile is one delivered content item written to disk (a look or a
// data item), or one that was not delivered.
type contentFile struct {
	Caption  string `json:"caption"`
	MIMEType string `json:"mimeType"`
	Bytes    int    `json:"bytes"`
	Rows     *int   `json:"rows,omitempty"` // data: CSV records or JSON array items
	Path     string `json:"path,omitempty"`
	Dropped  string `json:"dropped,omitempty"`
}

type jsonResult struct {
	Status  string        `json:"status"`
	Result  string        `json:"result"`
	Error   string        `json:"error,omitempty"`
	Console []string      `json:"console,omitempty"`
	Files   []contentFile `json:"files,omitempty"`
}

func runCode(ctx context.Context, opts *rootOptions, f *evalFlags, code string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if f.image != "" {
		if _, err := os.Stat(f.image); err != nil {
			return usageErr("--image: %v", err)
		}
	}
	logger := opts.logger()
	if err := checkOutRoot(f.outRoot); err != nil {
		return err
	}
	for _, p := range f.prelude {
		if st, err := os.Stat(p); err != nil || st.IsDir() {
			return usageErr("--prelude %q is not a readable file", p)
		}
	}
	sopts := cvsession.Options{Image: f.image, OutRoot: f.outRoot, Preludes: f.prelude, Logger: &logger}
	if f.timeout > 0 {
		sopts.EvalTimeout = msDuration(f.timeout)
	}
	app, err := cvsession.New(ctx, sopts)
	if err != nil {
		return failure(err)
	}
	defer func() { _ = app.Close(context.Background()) }()
	session, err := app.CreateSession(ctx)
	if err != nil {
		if msg := errors.Cause(err).Error(); strings.HasPrefix(msg, "prelude ") {
			return failure(errors.New(msg)) // a script error in a prelude: exit 1 like agent code
		}
		return usageErr("start session: %v", errors.Cause(err))
	}
	resp, err := app.Evaluate(ctx, session.ID, code)
	if err != nil && (resp == nil || resp.Cell == nil) {
		return failure(err)
	}
	files, err := writeContent(resp, f.out)
	if err != nil {
		return failure(err)
	}
	exec := resp.Cell.Execution
	if f.json {
		out := jsonResult{Status: exec.Status, Result: exec.Result, Error: exec.Error, Files: files}
		for _, c := range exec.Console {
			out.Console = append(out.Console, fmt.Sprintf("[%s] %s", c.Kind, c.Message))
		}
		enc := json.NewEncoder(opts.stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return failure(err)
		}
	} else {
		printText(opts.stdout, resp, files)
	}
	if exec.Status != "ok" {
		return silentFailure()
	}
	return nil
}

func printText(w io.Writer, resp *replsession.EvaluateResponse, files []contentFile) {
	exec := resp.Cell.Execution
	fmt.Fprintln(w, strings.TrimRight(exec.Result, "\n"))
	if len(exec.Console) > 0 {
		list := &yaml.Node{Kind: yaml.SequenceNode}
		for _, c := range exec.Console {
			list.Content = append(list.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: fmt.Sprintf("[%s] %s", c.Kind, c.Message)})
		}
		printYAML(w, "console", list)
	}
	if len(files) > 0 {
		list := &yaml.Node{Kind: yaml.MappingNode}
		for _, l := range files {
			v := l.Path
			if v == "" {
				v = "dropped: " + l.Dropped
			} else if l.Rows != nil {
				v += fmt.Sprintf(" (%d rows)", *l.Rows)
			}
			list.Content = append(list.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: l.Caption},
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v})
		}
		printYAML(w, "files", list)
	}
}

func printYAML(w io.Writer, key string, value *yaml.Node) {
	doc := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{{Kind: yaml.ScalarNode, Value: key}, value}}
	b, err := yaml.Marshal(doc)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "\n%s", b)
}

var (
	lookCaption = regexp.MustCompile(`^look (\d+): (.*?)(?: — | \()`)
	dataCaption = regexp.MustCompile(`^data ([A-Za-z0-9_-]{1,64})(\.[a-z]+)$`)
)

// writeContent writes the delivered content of resp into dir (a new
// temporary directory when dir is empty): looks as look-<n>-<slug>.<ext>,
// data items as <name>.<ext> from their caption (a second item of the same
// name gets -2). Items that were not delivered are listed with the reason.
func writeContent(resp *replsession.EvaluateResponse, dir string) ([]contentFile, error) {
	refs := resp.Cell.Execution.Content
	if len(refs) == 0 {
		return nil, nil
	}
	if dir == "" {
		d, err := os.MkdirTemp("", "goja-cv-out-")
		if err != nil {
			return nil, errors.Wrap(err, "create output directory")
		}
		dir = d
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, errors.Wrap(err, "create --out directory")
	}
	var out []contentFile
	used := map[string]int{}
	for _, ref := range refs {
		cf := contentFile{Caption: ref.Caption, MIMEType: ref.MIMEType, Bytes: ref.Bytes, Dropped: ref.Dropped}
		if ref.Index < 0 || ref.Index >= len(resp.Content) {
			if cf.Dropped == "" {
				cf.Dropped = "not delivered"
			}
			out = append(out, cf)
			continue
		}
		item := resp.Content[ref.Index]
		name, ext := fmt.Sprintf("content-%d", ref.Provider), resultcontent.ExtensionForMIME(item.MIMEType)
		switch m := lookCaption.FindStringSubmatch(item.Caption); {
		case m != nil:
			name = "look-" + m[1] + "-" + slug(m[2])
		default:
			if d := dataCaption.FindStringSubmatch(item.Caption); d != nil {
				name = d[1]
				rows := countRows(item.MIMEType, item.Data)
				cf.Rows = &rows
			}
		}
		if used[name+ext]++; used[name+ext] > 1 {
			name = fmt.Sprintf("%s-%d", name, used[name+ext])
		}
		abs, err := filepath.Abs(filepath.Join(dir, name+ext))
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(abs, item.Data, 0o644); err != nil { // #nosec G306 -- result files for the person running the CLI
			return nil, errors.Wrap(err, "write content")
		}
		cf.Path = abs
		out = append(out, cf)
	}
	return out, nil
}

// countRows counts CSV records (without the header) or JSON array items
// (1 for any other JSON value).
func countRows(mime string, data []byte) int {
	switch mime {
	case "text/csv":
		recs, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
		if err != nil || len(recs) == 0 {
			return 0
		}
		return len(recs) - 1
	case "application/json":
		var v any
		if json.Unmarshal(data, &v) == nil {
			if l, ok := v.([]any); ok {
				return len(l)
			}
		}
		return 1
	}
	return 0
}

var nonSlug = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func slug(s string) string {
	s = strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if len(s) > 40 {
		s = strings.TrimRight(s[:40], "-")
	}
	if s == "" {
		return "look"
	}
	return s
}

// checkOutRoot requires --out-root, when given, to be an existing directory.
func checkOutRoot(dir string) error {
	if dir == "" {
		return nil
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return usageErr("--out-root %q is not a directory", dir)
	}
	return nil
}
