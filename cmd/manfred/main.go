package main

import (
	"os"

	"github.com/mpm/manfred/internal/cli"
)

// version is set at build time via -ldflags "-X main.version=X.Y.Z"
var version = "dev"

func main() {
	cli.SetVersion(version)
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
