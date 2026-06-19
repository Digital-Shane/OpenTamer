//go:build !darwin || !cgo

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "OpenTamer requires macOS 14 or newer with cgo enabled. Build and run it as a macOS app bundle on a supported Mac.")
	os.Exit(1)
}
