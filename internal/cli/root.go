// Package cli wires the env-man cobra command tree.
package cli

import (
	"fmt"

	"env-man/internal/config"
	"env-man/internal/layer"
	"env-man/internal/materialize"
	"env-man/internal/state"
	"env-man/internal/tui"

	"github.com/spf13/cobra"
)

// Version is the current env-man build version. Overridden at link time via
// -ldflags "-X env-man/internal/cli.Version=...".
var Version = "0.1.0-dev"

// Global flags shared by all subcommands.
var (
	flagCWD     string
	flagNoColor bool
)

const rootShort = "Layered environment and config file manager"

const rootLong = `env-man manages environment variables and other configuration files
using a base layer plus any number of named overlay layers stored under .env-man/.

Layers are merged (for .env/.json/.yaml/.yml) or overwritten (everything else)
into the workspace outside .env-man/.

Run without a subcommand to launch the interactive TUI.`

// NewRootCmd builds the root command tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "env-man",
		Short:         rootShort,
		Long:          rootLong,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       Version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUI(cmd)
		},
	}

	root.PersistentFlags().StringVar(&flagCWD, "cwd", "", "working directory (defaults to the current directory)")
	root.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "disable colored output")

	root.AddCommand(newInitCmd())
	root.AddCommand(newApplyCmd())
	root.AddCommand(newDropCmd())
	root.AddCommand(newListCmd())
	root.AddCommand(newDiffCmd())

	return root
}

// Execute builds and runs the root command.
func Execute() error {
	return NewRootCmd().Execute()
}

// runTUI is the default action. It launches the interactive layer-toggle
// screen. When env-man is not configured in the working directory it prints a
// friendly message instead of erroring.
func runTUI(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()

	p, err := config.Require(flagCWD)
	if err != nil {
		if err == config.ErrNotConfigured {
			writeln(out, "No env-man configuration in this folder.")
			writeln(out, "Run `env-man init` to get started.")
			return nil
		}
		return err
	}
	st, err := state.Load(p.StateFile)
	if err != nil {
		return err
	}
	all, err := layer.Discover(p.Root)
	if err != nil {
		return err
	}

	final, err := tui.Run(p, st, all)
	if err != nil {
		return fmt.Errorf("run TUI: %w", err)
	}
	if !final.Applied() {
		return nil
	}

	// Persist the full TUI ordering (enabled + disabled) so disabling a layer
	// does not lose its position. The enabled subset drives the actual stack.
	st.SetOrder(final.Order())

	// The user confirmed: apply the selected layers.
	plan, err := materialize.BuildPlan(p, final.Selected())
	if err != nil {
		return err
	}
	res, err := materialize.Apply(p, st, plan)
	if err != nil {
		return err
	}
	printApplySummary(out, plan.Stack, res)
	return nil
}
