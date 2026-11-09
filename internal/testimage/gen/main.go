// Command gen writes the synthetic test images (go run ./internal/testimage/gen -out testdata).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/go-go-golems/goja-cv/internal/testimage"
)

func main() {
	out := flag.String("out", "testdata", "output directory")
	flag.Parse()
	if err := testimage.Write(context.Background(), *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
