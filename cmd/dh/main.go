package main

import (
	"os"

	"github.com/dolthub/cli/internal/app"
)

var version = "dev"

func main() {
	os.Exit(app.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, version))
}
