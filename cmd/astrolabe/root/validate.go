package root

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/cliout"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/tui"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/validate"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate <run_dir>",
	Short: "Check file/manifest consistency before upload",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonMode, _ := cmd.Flags().GetBool("json")
		runDir := args[0]

		v := validate.New(runDir)
		result := v.Run()

		if jsonMode {
			return outputJSON(result)
		}
		return outputStyled(result)
	},
}

func outputJSON(result validate.Result) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return fmt.Errorf("encode result: %w", err)
	}
	if !result.Valid {
		os.Exit(1)
	}
	return nil
}

func outputStyled(result validate.Result) error {
	out := cliout.DefaultPrinter(false)

	out.Step(fmt.Sprintf("Validating run directory: %s", result.RunDir))
	out.Blank()

	for _, check := range result.Checks {
		printCheck(out, check)
	}

	if len(result.Warnings) > 0 {
		out.Blank()
		for _, warn := range result.Warnings {
			out.Warning(warn)
		}
	}

	out.Blank()
	if result.Valid {
		runID := result.RunID
		if runID == "" {
			runID = "unknown"
		}
		out.Success(fmt.Sprintf("Validation passed for run %s", runID))
	} else {
		out.Error("Validation failed")
		for _, err := range result.Errors {
			out.Muted(fmt.Sprintf("  %s", err))
		}
		os.Exit(1)
	}

	return nil
}

func printCheck(out *cliout.Printer, check validate.Check) {
	var icon, label string

	switch check.Status {
	case validate.StatusPassed:
		icon = tui.StyleSuccess.Render(tui.IconStatusSuccess)
		label = check.Name
	case validate.StatusFailed:
		icon = tui.StyleError.Render(tui.IconStatusError)
		label = fmt.Sprintf("%s: %s", check.Name, check.Error)
	case validate.StatusSkipped:
		icon = tui.StyleMuted.Render("·")
		label = tui.StyleMuted.Render(fmt.Sprintf("%s (skipped)", check.Name))
	}

	out.Println(fmt.Sprintf("%s %s", icon, label))
}
