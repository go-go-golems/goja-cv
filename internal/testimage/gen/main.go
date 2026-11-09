// Command gen writes the synthetic test images (go run ./internal/testimage/gen -out testdata [-planted]).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-go-golems/goja-cv/internal/testimage"
)

func main() {
	out := flag.String("out", "testdata", "output directory")
	planted := flag.Bool("planted", false, "also write the planted-fault scan (not committed; tests generate it)")
	flag.Parse()
	if err := testimage.Write(context.Background(), *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *planted {
		if _, err := testimage.WritePlanted(context.Background(), filepath.Join(*out, testimage.PlantedTIFF)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
