package main

import (
	"os"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/cmd/astrolabe/root"
)

func main() {
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
