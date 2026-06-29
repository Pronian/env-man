// Package merge combines file contents across layers.
//
// Each file type uses one of four strategies, selected by the StrategyFor
// dispatcher based on the file's basename and extension:
//
//   - .env / .env.*         -> MergeEnv   (KEY=value map merge, normalized)
//   - .json                 -> MergeJSON  (deep object merge, arrays replaced)
//   - .yaml / .yml          -> MergeYAML  (deep mapping merge, order preserved)
//   - anything else         -> MergeBytes (last non-empty wins)
//
// Higher-priority layers come later in the input slice. Merge operations are
// pure functions of their inputs and never touch the filesystem.
package merge

import (
	"path/filepath"
	"strings"
)

// Strategy identifies how a file type is combined across layers.
type Strategy int

const (
	// StrategyBytes copies the highest-priority non-empty content verbatim.
	StrategyBytes Strategy = iota
	// StrategyEnv merges KEY=value pairs (no variable expansion).
	StrategyEnv
	// StrategyJSON deep-merges JSON objects; arrays and scalars are replaced.
	StrategyJSON
	// StrategyYAML deep-merges YAML mappings, preserving key order.
	StrategyYAML
)

// StrategyFor returns the merge strategy for a relative file path.
func StrategyFor(relpath string) Strategy {
	base := filepath.Base(relpath)
	if base == ".env" || strings.HasPrefix(base, ".env.") {
		return StrategyEnv
	}
	switch strings.ToLower(filepath.Ext(base)) {
	case ".json":
		return StrategyJSON
	case ".yaml", ".yml":
		return StrategyYAML
	default:
		return StrategyBytes
	}
}

// Merge combines the ordered contents (lowest priority first) for relpath
// using the appropriate strategy. Returns nil content if every layer is empty.
func Merge(relpath string, contents [][]byte) ([]byte, error) {
	switch StrategyFor(relpath) {
	case StrategyEnv:
		return MergeEnv(contents)
	case StrategyJSON:
		return MergeJSON(contents)
	case StrategyYAML:
		return MergeYAML(contents)
	default:
		return MergeBytes(contents), nil
	}
}

// nonEmpty drops nil and zero-length entries, preserving order.
func nonEmpty(in [][]byte) [][]byte {
	out := make([][]byte, 0, len(in))
	for _, c := range in {
		if len(c) > 0 {
			out = append(out, c)
		}
	}
	return out
}
