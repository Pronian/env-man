package cli

import (
	"env-man/internal/config"
	"env-man/internal/materialize"
	"env-man/internal/state"

	"github.com/spf13/cobra"
)

func newApplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply [layer...]",
		Short: "Replace the layer stack and materialize the merged files",
		Long: `Replaces the currently applied layer stack with the named layers
(lowest priority leftmost, highest rightmost) and writes the merged/overlaid
files into the workspace outside .env-man/.

The base layer is always implicit at the bottom of the stack. With no layer
arguments, applies base only (clearing any overlays).`,
		Args: cobra.ArbitraryArgs,
		RunE: runApply,
	}
	return cmd
}

func runApply(cmd *cobra.Command, args []string) error {
	p, err := config.Require(flagCWD)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()

	st, err := state.Load(p.StateFile)
	if err != nil {
		return err
	}
	plan, err := materialize.BuildPlan(p, args)
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
