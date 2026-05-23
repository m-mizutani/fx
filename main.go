// Package main is the fx command-line entry point. All real logic lives in
// internal/cli; this file is just an exit-code shim around it so tests can
// drive the same code path without an os.Exit.
package main

import (
	"context"
	"os"

	"github.com/m-mizutani/fx/internal/cli"
)

func main() {
	os.Exit(cli.Run(context.Background(), os.Args, os.Stdin, os.Stdout, os.Stderr))
}
