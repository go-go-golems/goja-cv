// Command goja-cv runs cv scripts and serves the cv eval session over MCP.
//
//	goja-cv run SCRIPT.js [--image FILE] [--out DIR] [--json]
//	goja-cv eval -e CODE [--image FILE] [--out DIR] [--json]
//	goja-cv ops [--kind K] [--json]
//	goja-cv help OP [--json]
//	goja-cv mcp [--http ADDR] [--allow-remote] [--image FILE] [--root DIR...]
//
// Exit codes: 0 ok, 1 cv error (still printed), 2 usage error.
package main

import (
	"context"
	"os"
)

func main() {
	os.Exit(execute(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
