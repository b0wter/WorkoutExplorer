// Command brotliassets writes a .br sidecar next to every compressible file
// of a web build's assets directory (F-22 H-7), for Caddy's `precompressed br`.
// A file whose brotli form is not smaller gets none.
//
//	brotliassets <build/web/assets>
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"workout-explorer/internal/precompress"
)

var textual = map[string]bool{".js": true, ".css": true, ".json": true, ".svg": true, ".html": true, ".txt": true, ".map": true, ".wasm": true}

// run compresses dir's files and returns how many sidecars it wrote. Stale
// sidecars are rewritten, so it is idempotent.
func run(dir string) (int, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range ents {
		name := e.Name()
		if !e.Type().IsRegular() || !textual[strings.ToLower(filepath.Ext(name))] {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return n, err
		}
		br, err := precompress.Brotli(src)
		if err != nil {
			return n, err
		}
		if len(br) >= len(src) {
			os.Remove(filepath.Join(dir, name+".br"))
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, name+".br"), br, 0o644); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: brotliassets <assets dir>")
		os.Exit(2)
	}
	n, err := run(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "brotliassets:", err)
		os.Exit(1)
	}
	fmt.Printf("brotliassets: %d sidecars\n", n)
}
