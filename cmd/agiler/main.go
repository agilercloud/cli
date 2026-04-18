package main

import (
	"context"
	"fmt"
	"os"

	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/cli"
)

// Version is set via -ldflags "-X main.Version=..." at build time.
var Version = "dev"

func main() {
	a, err := app.Wire(Version)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(cli.Run(a, context.Background(), os.Args[1:]))
}
