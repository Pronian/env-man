// Package drop removes every file env-man has materialized outside .env-man/.
//
// Drop reads the manifest from state.yaml, deletes each tracked file, prunes
// newly-empty directories, and resets the applied layer stack and manifest. The
// saved layer ordering (state.File.Order) is intentionally preserved so the TUI
// remembers your preferred ordering with everything disabled. The .env-man/
// folder itself is left untouched.
package drop

import (
	"fmt"
	"os"
	"path/filepath"

	"env-man/internal/config"
	"env-man/internal/fileutil"
	"env-man/internal/state"
)

// Result reports what drop removed.
type Result struct {
	Removed []string // files that were on disk and deleted
	Missing []string // manifest entries that were already gone
}

// Run deletes every file in the state manifest and clears the applied layer
// stack. A manifest entry that is already absent from disk is reported as
// Missing (not an error), so drop stays robust against manual edits.
func Run(p config.Paths) (*Result, error) {
	st, err := state.Load(p.StateFile)
	if err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	res := &Result{}
	for _, rel := range st.Materialized {
		target := filepath.Join(p.Workdir, filepath.FromSlash(rel))
		if err := os.Remove(target); err != nil {
			if os.IsNotExist(err) {
				res.Missing = append(res.Missing, rel)
				continue
			}
			return nil, fmt.Errorf("remove %q: %w", rel, err)
		}
		res.Removed = append(res.Removed, rel)
		fileutil.PruneEmptyDirs(p.Workdir, filepath.Dir(target))
	}

	st.SetLayers(nil)
	st.SetMaterialized(nil)
	if err := st.Save(); err != nil {
		return nil, fmt.Errorf("save state: %w", err)
	}
	return res, nil
}
