package root

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/cthonicasoftware/astrolabe-cli/internal/cliout"
	"github.com/cthonicasoftware/astrolabe-cli/internal/tui"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "astrolabe",
	Short: "Standardized capture & upload of QA artifacts",
	Long:  "Astrolabe captures, normalizes, caches, and uploads QA test run data.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If no subcommand provided, launch TUI
		return tuiCmd.RunE(cmd, args)
	},
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.astrolabe/connection.yml)")
	rootCmd.PersistentFlags().Bool("json", false, "emit machine-readable JSON output")
	if err := viper.BindPFlag("json", rootCmd.PersistentFlags().Lookup("json")); err != nil {
		panic(fmt.Sprintf("bind json flag: %v", err))
	}

	// attach subcommands
	rootCmd.AddCommand(captureCmd)
	rootCmd.AddCommand(uploadCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(versionCmd)

	rootCmd.SetHelpFunc(renderStyledHelp)
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return fmt.Errorf("invalid arguments: %w (run '%s --help')", err, cmd.CommandPath())
	})
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(home + "/.astrolabe")
		}
		viper.SetConfigName("connection")
		viper.SetConfigType("yaml")
	}
	viper.SetEnvPrefix("ASTROLABE")
	viper.AutomaticEnv()
	_ = viper.ReadInConfig()
}

func Execute() error {
	err := rootCmd.Execute()
	if err != nil {
		out := cliout.DefaultPrinter(viper.GetBool("json"))
		// Validation flow already renders detailed styled errors before returning a sentinel.
		if err != errValidationFailed {
			out.Error(err.Error())
		}
	}
	return err
}

type helpRenderer struct {
	out *cliout.Printer
}

func newHelpRenderer(w io.Writer) helpRenderer {
	return helpRenderer{
		out: cliout.NewPrinter(w, false),
	}
}

func (h helpRenderer) title(text string) string {
	return h.out.StyledHeader(text)
}

func (h helpRenderer) muted(text string) string {
	return h.out.StyledMuted(text)
}

func (h helpRenderer) item(text string) string {
	return h.out.StyledItem(text)
}

func (h helpRenderer) section(title string) string {
	prefix := h.out.StyledIcon(tui.IconStatusInfo)
	if prefix != "" {
		prefix += " "
	}
	return h.out.StyledHeader(prefix + strings.ToUpper(title))
}

func renderStyledHelp(cmd *cobra.Command, _ []string) {
	w := cmd.OutOrStdout()
	hr := newHelpRenderer(w)
	fmt.Fprintln(w, hr.title(cmd.CommandPath()))
	if short := strings.TrimSpace(cmd.Short); short != "" {
		fmt.Fprintln(w, hr.muted(short))
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, hr.section("Usage"))
	fmt.Fprintf(w, "  %s\n", cmd.UseLine())

	if ex := strings.TrimSpace(cmd.Example); ex != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, hr.section("Examples"))
		for _, line := range strings.Split(ex, "\n") {
			if strings.TrimSpace(line) == "" {
				fmt.Fprintln(w)
				continue
			}
			fmt.Fprintf(w, "  %s\n", line)
		}
	}

	renderCommandList(w, hr, cmd)
	renderFlags(w, hr, "Flags", cmd.NonInheritedFlags())
	renderFlags(w, hr, "Global Flags", cmd.InheritedFlags())
}

func renderCommandList(w io.Writer, hr helpRenderer, cmd *cobra.Command) {
	commands := cmd.Commands()
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Name() < commands[j].Name()
	})

	type row struct {
		name  string
		short string
	}
	rows := make([]row, 0, len(commands))
	maxName := 0
	for _, c := range commands {
		if !c.IsAvailableCommand() || c.Hidden {
			continue
		}
		name := c.Name()
		if len(name) > maxName {
			maxName = len(name)
		}
		rows = append(rows, row{name: name, short: c.Short})
	}

	if len(rows) == 0 {
		return
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, hr.section("Commands"))
	for _, r := range rows {
		name := hr.item(r.name)
		fmt.Fprintf(w, "  %-*s  %s\n", maxName, name, strings.TrimSpace(r.short))
	}
	fmt.Fprintf(w, "  %s  %s\n", hr.item("help"), "Help about any command")
}

func renderFlags(w io.Writer, hr helpRenderer, heading string, fs *pflag.FlagSet) {
	if fs == nil || !fs.HasAvailableFlags() {
		return
	}

	type row struct {
		left  string
		right string
	}
	rows := []row{}
	maxLeft := 0
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		left := "--" + f.Name
		if f.Shorthand != "" {
			left = "-" + f.Shorthand + ", " + left
		}
		if f.Value.Type() != "bool" {
			left += " " + f.Value.Type()
		}
		if len(left) > maxLeft {
			maxLeft = len(left)
		}
		right := strings.TrimSpace(f.Usage)
		if f.DefValue != "" && f.DefValue != "false" {
			right += fmt.Sprintf(" (default %q)", f.DefValue)
		}
		rows = append(rows, row{left: left, right: right})
	})
	if len(rows) == 0 {
		return
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, hr.section(heading))
	for _, r := range rows {
		left := hr.item(r.left)
		fmt.Fprintf(w, "  %-*s  %s\n", maxLeft, left, r.right)
	}
}
