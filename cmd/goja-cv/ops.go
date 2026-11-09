package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
	"github.com/go-go-golems/goja-cv/pkg/cv/graph"
	"github.com/go-go-golems/goja-cv/pkg/doc"
	cvmod "github.com/go-go-golems/goja-cv/pkg/gojamod/cv"
	"github.com/pkg/errors"
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

// setupHelp installs the Glazed help system with the guidebook (pkg/doc).
// `goja-cv help NAME` resolves NAME as a guide topic slug, then as a
// command, then as a cv op / kind / function (the op registry), so
// `goja-cv help levels` keeps working next to `goja-cv help concepts`.
func setupHelp(root *cobra.Command, opts *rootOptions) error {
	hs, err := doc.NewHelpSystem()
	if err != nil {
		return err
	}
	if err := help_cmd.SetupCobraRootCommandE(hs, root); err != nil {
		return err
	}
	var helpCmd *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "help" {
			helpCmd = c
		}
	}
	if helpCmd == nil {
		return errors.New("help command not installed")
	}
	helpCmd.Use = "help [topic | command | op]"
	helpCmd.Short = "Guide topics (goja-cv help what-is-goja-cv), command help, or one cv op (goja-cv help levels)"
	var asJSON bool
	helpCmd.Flags().BoolVar(&asJSON, "json", false, "Print op help as JSON")
	glazedRun := helpCmd.Run
	helpCmd.Run = nil
	helpCmd.RunE = func(c *cobra.Command, args []string) error {
		if len(args) == 1 {
			if _, err := hs.GetSectionWithSlug(args[0]); err != nil {
				if sub, _, ferr := root.Find(args); ferr != nil || sub == root {
					return opHelp(opts, args[0], asJSON)
				}
			}
		}
		glazedRun(c, args)
		return nil
	}
	return nil
}

// opHelp prints cv.help(name) as YAML (or JSON); exit 1 when nothing matches.
func opHelp(opts *rootOptions, name string, asJSON bool) error {
	h := cvmod.Help(cvmod.DefaultRegistry(), name)
	var err error
	if asJSON {
		err = writeJSON(opts.stdout, h)
	} else {
		err = writeYAML(opts.stdout, h)
	}
	if err != nil {
		return err
	}
	if found, ok := h["found"].(bool); ok && !found {
		return silentFailure()
	}
	return nil
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
