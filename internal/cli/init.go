package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"env-man/internal/config"
	"env-man/internal/fileutil"
	"env-man/internal/state"

	"github.com/spf13/cobra"
)

// baseEnvPlaceholder is written into .env-man/base/.env on init.
const baseEnvPlaceholder = `# env-man base environment layer.
# Values here are always applied at the lowest priority.
# Overlay layers live in sibling folders under .env-man/.

`

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create the .env-man/ scaffold in the working directory",
		Long: `Creates the .env-man/ folder with a state.yaml file and a base/.env
placeholder. If the structure already exists, missing pieces are healed and
existing content is never destroyed.`,
		Args: cobra.NoArgs,
		RunE: runInit,
	}
}

func runInit(cmd *cobra.Command, _ []string) error {
	wd, err := config.ResolveWorkdir(flagCWD)
	if err != nil {
		return err
	}
	p := config.New(wd)
	out := cmd.OutOrStdout()

	// Ensure .env-man/base/ exists (creates .env-man/ implicitly).
	if err := os.MkdirAll(p.BaseDir, 0o755); err != nil {
		return fmt.Errorf("create %q: %w", p.BaseDir, err)
	}

	var created []string
	if !fileExists(p.StateFile) {
		if err := state.New(p.StateFile).Save(); err != nil {
			return err
		}
		created = append(created, config.DirName+"/"+config.StateName)
	}
	baseEnv := filepath.Join(p.BaseDir, ".env")
	if !fileExists(baseEnv) {
		if err := fileutil.WriteFileAtomic(baseEnv, []byte(baseEnvPlaceholder), 0o644); err != nil {
			return err
		}
		created = append(created, config.DirName+"/"+config.BaseName+"/.env")
	}

	if len(created) == 0 {
		fmt.Fprintf(out, "env-man is already initialized in %s\n", wd)
	} else {
		fmt.Fprintf(out, "Initialized env-man in %s\n", wd)
		for _, c := range created {
			fmt.Fprintf(out, "  created  %s\n", c)
		}
	}
	return nil
}
