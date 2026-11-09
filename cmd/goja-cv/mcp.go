package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-go-golems/go-go-goja/pkg/replmcp"
	"github.com/go-go-golems/goja-cv/pkg/cvsession"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

type mcpFlags struct {
	http        string
	allowRemote bool
	image       string
	roots       []string
	outRoot     string
	prelude     []string
	timeout     int
}

func newMCPCommand(opts *rootOptions) *cobra.Command {
	f := &mcpFlags{}
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Serve the cv eval session as an MCP server (stdio by default)",
		Long: `Serve an MCP server with the cv eval tool, open, create_session, list_sessions, history and
bindings, the cv://reference resource and the cv-agent prompt. Looks are returned as image content.

The server speaks MCP over stdin/stdout unless --http is given. Only the cv and timer modules are
available to scripts. --root limits the files cv.open may read (default: any path).

  claude mcp add cv -- goja-cv mcp --image scan.tif --root .`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if f.http != "" {
				if err := checkListenAddress(f.http, f.allowRemote); err != nil {
					return err
				}
			}
			if f.image != "" {
				if _, err := os.Stat(f.image); err != nil {
					return usageErr("--image: %v", err)
				}
			}
			if err := checkOutRoot(f.outRoot); err != nil {
				return err
			}
			for _, r := range f.roots {
				if st, err := os.Stat(r); err != nil || !st.IsDir() {
					return usageErr("--root %q is not a directory", r)
				}
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			srv, closeFn, err := newMCPServer(ctx, opts, f)
			if err != nil {
				return err
			}
			defer closeFn()
			if f.http == "" {
				if err := replmcp.ServeStdio(ctx, srv.Server); err != nil && !errors.Is(err, context.Canceled) {
					return failure(errors.Wrap(err, "serve mcp over stdio"))
				}
				return nil
			}
			return serveHTTP(ctx, opts, srv, f.http)
		},
	}
	cmd.Flags().StringVar(&f.http, "http", "", "Serve MCP streamable HTTP on this address (for example 127.0.0.1:3092) instead of stdio")
	cmd.Flags().BoolVar(&f.allowRemote, "allow-remote", false, "Allow a non-loopback --http address (there is no authentication)")
	cmd.Flags().StringVar(&f.image, "image", "", "Image file to open in the default session and bind to img")
	cmd.Flags().StringSliceVar(&f.roots, "root", nil, "Directory cv.open may read from (repeatable)")
	cmd.Flags().StringVar(&f.outRoot, "out-root", "", "The only directory save() may write under (default: saving is disabled)")
	cmd.Flags().StringArrayVar(&f.prelude, "prelude", nil, "Script run in every new session before agent code (repeatable)")
	cmd.Flags().IntVar(&f.timeout, "timeout-ms", 0, "Eval timeout in milliseconds (default 60000)")
	return cmd
}

func newMCPServer(ctx context.Context, opts *rootOptions, f *mcpFlags) (*cvsession.MCPServer, func(), error) {
	logger := opts.logger()
	sopts := cvsession.Options{Image: f.image, Roots: f.roots, OutRoot: f.outRoot, Preludes: f.prelude, MaxDataBytes: cvsession.MCPMaxDataBytes, Logger: &logger}
	if f.timeout > 0 {
		sopts.EvalTimeout = msDuration(f.timeout)
	}
	app, err := cvsession.New(ctx, sopts)
	if err != nil {
		return nil, nil, failure(err)
	}
	closeFn := func() { _ = app.Close(context.Background()) }
	srv, err := cvsession.NewMCPServer(ctx, app)
	if err != nil {
		closeFn()
		return nil, nil, usageErr("start session: %v", errors.Cause(err))
	}
	return srv, closeFn, nil
}

func checkListenAddress(addr string, allowRemote bool) error {
	host, _, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil {
		return usageErr("--http %q: %v", addr, err)
	}
	if allowRemote {
		return nil
	}
	if host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	return usageErr("--http %q is not a loopback address; the MCP server has no authentication (pass --allow-remote to accept that)", addr)
}

func serveHTTP(ctx context.Context, opts *rootOptions, srv *cvsession.MCPServer, addr string) error {
	server := &http.Server{
		Addr:              addr,
		Handler:           replmcp.NewStreamableHTTPHandler(srv.Server, nil),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		logger := opts.logger()
		logger.Info().Str("addr", addr).Msg("goja-cv MCP server listening")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	group.Go(func() error {
		<-groupCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	})
	if err := group.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return failure(errors.Wrap(err, "serve mcp over http"))
	}
	return nil
}
