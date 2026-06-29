package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"env-man/internal/materialize"
)

// fileExists reports whether a non-directory entry exists at p.
func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// stackNames returns the layer names of a plan stack in order.
func stackNames(stack []materialize.StackEntry) []string {
	names := make([]string, 0, len(stack))
	for _, se := range stack {
		names = append(names, se.Name)
	}
	return names
}

// printApplySummary writes a human-readable summary of an Apply result.
func printApplySummary(out io.Writer, stack []materialize.StackEntry, res *materialize.Result) {
	fmt.Fprintf(out, "Applied layers: %s\n", strings.Join(stackNames(stack), " -> "))
	section := func(label string, items []string) {
		if len(items) == 0 {
			return
		}
		fmt.Fprintf(out, "  %-9s %s\n", label+":", items[0])
		for _, x := range items[1:] {
			fmt.Fprintf(out, "  %9s %s\n", "", x)
		}
	}
	section("created", res.Created)
	section("updated", res.Updated)
	section("removed", res.Removed)
	section("unchanged", res.Unchanged)
	if len(res.Created)+len(res.Updated)+len(res.Removed)+len(res.Unchanged) == 0 {
		fmt.Fprintln(out, "  (no files)")
	}
}
