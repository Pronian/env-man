package cli

import (
	"fmt"

	"env-man/internal/config"
	"env-man/internal/drop"

	"github.com/spf13/cobra"
)

func newDropCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "drop",
		Short: "Remove every file env-man materialized outside .env-man/",
		Long: `Deletes every file that env-man has written outside the .env-man/
folder (tracked via the manifest in state.yaml) and clears the applied layer
stack. The .env-man/ folder itself is left untouched.`,
		Args: cobra.NoArgs,
		RunE: runDrop,
	}
}

func runDrop(cmd *cobra.Command, _ []string) error {
	p, err := config.Require(flagCWD)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()

	res, err := drop.Run(p)
	if err != nil {
		return err
	}
	if len(res.Removed) == 0 && len(res.Missing) == 0 {
		fmt.Fprintln(out, "Nothing to drop (no layers currently applied).")
		return nil
	}
	fmt.Fprintf(out, "Dropped %d file(s):\n", len(res.Removed))
	for _, r := range res.Removed {
		fmt.Fprintf(out, "  removed  %s\n", r)
	}
	for _, m := range res.Missing {
		fmt.Fprintf(out, "  missing  %s (already gone)\n", m)
	}
	return nil
}
