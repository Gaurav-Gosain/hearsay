//go:build !(js && wasm)

// Command wasm is the browser build. It only builds for GOOS=js GOARCH=wasm.
// Run wasm/build.sh.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "This is the browser build. Run wasm/build.sh to build it.")
	os.Exit(2)
}
