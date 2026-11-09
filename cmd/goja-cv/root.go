package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

// Exit codes.
const (
	exitOK      = 0
	exitCVError = 1
	exitUsage   = 2
)

// exitError carries an exit code; a nil err means the output is already printed.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	if e.err == nil {
		return fmt.Sprintf("exit %d", e.code)
	}
	return e.err.Error()
}

func usageErr(format string, args ...any) error {
	return &exitError{code: exitUsage, err: errors.Errorf(format, args...)}
}

func failure(err error) error { return &exitError{code: exitCVError, err: err} }

// silentFailure is a cv error whose structured output was already printed.
func silentFailure() error { return &exitError{code: exitCVError} }

type rootOptions struct {
	logLevel string
	stdout   io.Writer
	stderr   io.Writer
}

func (o *rootOptions) logger() zerolog.Logger {
	level, err := zerolog.ParseLevel(strings.ToLower(o.logLevel))
	if err != nil || o.logLevel == "" {
		level = zerolog.WarnLevel
	}
	return zerolog.New(zerolog.ConsoleWriter{Out: o.stderr, NoColor: true}).Level(level).With().Timestamp().Logger()
}

func newRootCommand(stdout, stderr io.Writer) *cobra.Command {
	opts := &rootOptions{stdout: stdout, stderr: stderr}
	root := &cobra.Command{
		Use:   "goja-cv",
		Short: "Lazy image analysis for agents: run cv scripts or serve the cv eval session over MCP",
		Long: `goja-cv runs JavaScript against the cv image API (require("cv")).

Scripts are the body of an async function: use await, and return what you want to see.
Results are printed as YAML; looks (node.look()) are written as image files.
Exit codes: 0 ok, 1 cv error (the structured error is still printed), 2 usage error.`,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().StringVar(&opts.logLevel, "log-level", "warn", "Log level (trace, debug, info, warn, error)")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &exitError{code: exitUsage, err: err}
	})
	root.AddCommand(newRunCommand(opts), newEvalCommand(opts), newOpsCommand(opts), newMCPCommand(opts))
	if err := setupHelp(root, opts); err != nil {
		fmt.Fprintf(stderr, "goja-cv: help pages unavailable: %v\n", err)
	}
	return root
}

// execute runs the CLI and returns the exit code.
func execute(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	root := newRootCommand(stdout, stderr)
	root.SetArgs(args)
	err := root.ExecuteContext(ctx)
	if err == nil {
		return exitOK
	}
	var ee *exitError
	if errors.As(err, &ee) {
		if ee.err != nil {
			fmt.Fprintf(stderr, "goja-cv: %v\n", ee.err)
		}
		return ee.code
	}
	// cobra's own errors: unknown command, wrong number of arguments
	fmt.Fprintf(stderr, "goja-cv: %v\nRun 'goja-cv --help' for usage.\n", err)
	return exitUsage
}
