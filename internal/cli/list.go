package cli

import (
	"env-man/internal/config"
	"env-man/internal/layer"
	"env-man/internal/state"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show the current stack and all available layers",
		Long:  `Lists the currently applied layer stack and every layer folder found under .env-man/, marking which ones are applied.`,
		Args:  cobra.NoArgs,
		RunE:  runList,
	}
}

func runList(cmd *cobra.Command, _ []string) error {
	p, err := config.Require(flagCWD)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()

	st, err := state.Load(p.StateFile)
	if err != nil {
		return err
	}
	all, err := layer.Discover(p.Root)
	if err != nil {
		return err
	}

	applied := make(map[string]bool, len(st.Layers))
	for _, l := range st.Layers {
		applied[l] = true
	}

	writef(out, "Workspace: %s\n\n", p.Workdir)

	writeln(out, "Current stack (low -> high priority):")
	writef(out, "  * %-16s (locked)\n", config.BaseName)
	for _, l := range st.Layers {
		writef(out, "  * %s\n", l)
	}
	if len(st.Layers) == 0 {
		writeln(out, "    (no overlay layers applied)")
	}

	writeln(out)
	writeln(out, "Available layers:")
	count := 0
	for _, l := range all {
		if l == config.BaseName {
			continue
		}
		count++
		mark := " "
		if applied[l] {
			mark = "*"
		}
		writef(out, "  %s %s\n", mark, l)
	}
	if count == 0 {
		writeln(out, "  (no overlay layer folders)")
	}
	return nil
}
