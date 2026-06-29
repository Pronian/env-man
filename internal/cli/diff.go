package cli

import (
	"fmt"
	"strings"

	"env-man/internal/config"
	"env-man/internal/diff"

	"github.com/spf13/cobra"
)

func newDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff [layer...]",
		Short: "Preview (dry-run) what an apply would change",
		Long: `Computes the changes that "apply <layer...>" would make without
writing anything. With no layer arguments, previews the currently applied
stack. Reports per-file status (added/modified/unchanged/removed) and per-key
deltas for mergeable file types.`,
		Args: cobra.ArbitraryArgs,
		RunE: runDiff,
	}
	return cmd
}

func runDiff(cmd *cobra.Command, args []string) error {
	p, err := config.Require(flagCWD)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()

	var layers []string
	if len(args) > 0 {
		layers = args
	}
	report, err := diff.Compute(p, layers)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Proposed stack: %s\n\n", strings.Join(stackNames(report.Plan.Stack), " -> "))
	if report.Changed() == 0 {
		fmt.Fprintln(out, "No changes.")
		return nil
	}
	fmt.Fprintf(out, "Changes (%d):\n", report.Changed())
	for _, f := range report.Files {
		if !f.HasChanged() {
			continue
		}
		switch f.Status {
		case diff.StatusAdded:
			fmt.Fprintf(out, "  + added    %s\n", f.RelPath)
		case diff.StatusRemoved:
			fmt.Fprintf(out, "  - removed  %s\n", f.RelPath)
		case diff.StatusModified:
			fmt.Fprintf(out, "  ~ modified %s\n", f.RelPath)
			for _, d := range f.KeyDeltas {
				switch d.Status {
				case diff.StatusAdded:
					fmt.Fprintf(out, "      + %s\n", d.Path)
				case diff.StatusRemoved:
					fmt.Fprintf(out, "      - %s\n", d.Path)
				case diff.StatusModified:
					fmt.Fprintf(out, "      ~ %s\n", d.Path)
				}
			}
		}
	}
	return nil
}
