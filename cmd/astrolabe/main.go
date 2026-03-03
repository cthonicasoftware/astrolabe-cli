package main

import (
	"os"

	"github.com/cthonicasoftware/astrolabe-cli/cmd/astrolabe/root"
)

func main() {
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
