package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	cvmod "github.com/go-go-golems/goja-cv/pkg/gojamod/cv"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func msDuration(ms int) time.Duration { return time.Duration(ms) * time.Millisecond }

type opInfo struct {
	Name    string            `json:"name" yaml:"name"`
	On      string            `json:"on" yaml:"on"`
	Returns string            `json:"returns" yaml:"returns"`
	Doc     string            `json:"doc,omitempty" yaml:"doc,omitempty"`
	Args    map[string]string `json:"args,omitempty" yaml:"args,omitempty"`
}

func kindName(k graph.Kind) string {
	if k == graph.KindNone {
		return "cv"
	}
	return string(k)
}

func newOpsCommand(opts *rootOptions) *cobra.Command {
	var kind string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "ops",
		Short: "List the cv ops (the registry), optionally for one receiver kind",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			reg := cvmod.DefaultRegistry()
			valid := []string{"cv"}
			for _, k := range graph.Kinds {
				valid = append(valid, string(k))
			}
			if kind != "" && !contains(valid, kind) {
				return usageErr("--kind %q: valid kinds are %s", kind, strings.Join(valid, ", "))
			}
			var list []opInfo
			for _, op := range reg.Ops() {
				if kind != "" && kindName(op.On) != kind {
					continue
				}
				info := opInfo{Name: op.Name, On: kindName(op.On), Returns: string(op.Returns), Doc: op.Doc, Args: map[string]string{}}
				for i := range op.Args {
					if s := &op.Args[i]; !s.Hidden {
						info.Args[s.Name] = graph.DescribeArgSpec(s)
					}
				}
				list = append(list, info)
			}
			sort.SliceStable(list, func(i, j int) bool {
				if list[i].On != list[j].On {
					return kindOrder(list[i].On) < kindOrder(list[j].On)
				}
				return list[i].Name < list[j].Name
			})
			if asJSON {
				return writeJSON(opts.stdout, list)
			}
			for _, op := range list {
				names := make([]string, 0, len(op.Args))
				for n := range op.Args {
					names = append(names, n)
				}
				sort.Strings(names)
				doc := op.Doc
				if i := strings.Index(doc, ". "); i > 0 {
					doc = doc[:i+1]
				}
				fmt.Fprintf(opts.stdout, "%-8s %-28s → %-6s %s\n", op.On, op.Name+"("+strings.Join(names, ", ")+")", op.Returns, doc)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "Receiver kind: cv (sources), Image, Mask, Regions, Value, Param")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print JSON")
	return cmd
}

func kindOrder(k string) int {
	order := []string{"cv", "Image", "Mask", "Regions", "Value", "Param"}
	for i, x := range order {
		if x == k {
			return i
		}
	}
	return len(order)
}

// newHelpOpCommand replaces cobra's help command: "help OP" describes an op,
// "help COMMAND" (or no argument) shows command help.
func newHelpOpCommand(opts *rootOptions) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "help [OP | COMMAND]",
		Short: "Describe one cv op (cv.help(OP)), or show help for a command",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			if len(args) == 0 {
				return root.Help()
			}
			if sub, _, err := root.Find(args); err == nil && sub != root {
				return sub.Help()
			}
			h, err := cvmod.DefaultRegistry().Help(args[0])
			if err != nil {
				printHelpError(opts.stdout, err, asJSON)
				return silentFailure()
			}
			if asJSON {
				return writeJSON(opts.stdout, h)
			}
			return writeYAML(opts.stdout, h)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print JSON")
	return cmd
}

func printHelpError(w io.Writer, err error, asJSON bool) {
	ce, ok := graph.AsError(err)
	if !ok {
		ce = &graph.Error{Code: graph.EInternal, Message: err.Error()}
	}
	m := map[string]any{"code": ce.Code, "message": ce.Message}
	for k, v := range map[string]string{"hint": ce.Hint, "fix": ce.Fix} {
		if v != "" {
			m[k] = v
		}
	}
	if len(ce.Valid) > 0 {
		m["valid"] = ce.Valid
	}
	if asJSON {
		_ = writeJSON(w, map[string]any{"error": m})
		return
	}
	_ = writeYAML(w, map[string]any{"error": m})
}

func writeYAML(w io.Writer, v any) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return failure(err)
	}
	return enc.Close()
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func contains(xs []string, x string) bool {
	for _, e := range xs {
		if e == x {
			return true
		}
	}
	return false
}
