package main

import (
	"os"

	"github.com/s4na/gh-patch/internal/prx"
)

func main() {
	os.Exit(prx.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, prx.NewGHCLI()))
}
