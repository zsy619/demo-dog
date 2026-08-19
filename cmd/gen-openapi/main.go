// gen-openapi renders docs/openapi.json. Run with:
//
//	go run ./cmd/gen-openapi
//
// or to a specific path:
//
//	go run ./cmd/gen-openapi -out docs/openapi.json
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/zsy619/demo-dog/backend/internal/openapi"
)

func main() {
	out := flag.String("out", "docs/openapi.json", "output path for the OpenAPI spec")
	flag.Parse()

	spec := openapi.New()
	data, err := spec.JSON()
	if err != nil {
		fmt.Fprintln(os.Stderr, "encode:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %d bytes to %s\n", len(data), *out)
}
