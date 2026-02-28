package main

import (
	"github.com/LostinTimeandspaceYT/qa_cli_agent/cmd/astrolabe/root"
	"github.com/spf13/cobra"
)

func main() {
	cobra.CheckErr(root.Execute())
}
