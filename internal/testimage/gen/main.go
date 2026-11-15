// Command gen writes the synthetic test images (go run ./internal/testimage/gen -out testdata [-planted]),
// or the benchmark scan (-bench-scan 6000x4000 writes bench-scan-6000x4000.npy, float32, 288 MB at that size).
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
	benchScan := flag.String("bench-scan", "", "write only the benchmark scan of this size, e.g. 6000x4000 (bench-scan-WxH.npy)")
	flag.Parse()
	if *benchScan != "" {
		var w, h int
		if _, err := fmt.Sscanf(*benchScan, "%dx%d", &w, &h); err != nil || w < 64 || h < 64 {
			fmt.Fprintln(os.Stderr, "-bench-scan wants WxH with both at least 64, e.g. 6000x4000")
			os.Exit(2)
		}
		if err := os.MkdirAll(*out, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		path := filepath.Join(*out, fmt.Sprintf("bench-scan-%dx%d.npy", w, h))
		if err := testimage.WriteNPY(path, testimage.BenchScan(w, h, w*h/60000), "<f4"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(path)
		return
	}
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
