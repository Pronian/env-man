// Package state reads and writes the .env-man/state.yaml file.
//
// The state file records the currently applied layer stack and the manifest of
// files env-man has materialized outside .env-man/ (used by `drop`).
package state

import (
	"fmt"
	"os"

	"env-man/internal/fileutil"

	"gopkg.in/yaml.v3"
)

// CurrentVersion is the state.yaml schema version written by this build.
const CurrentVersion = 1

// File is the in-memory representation of .env-man/state.yaml.
type File struct {
	Version      int      `yaml:"version"`
	Layers       []string `yaml:"layers,omitempty"`
	Order        []string `yaml:"order,omitempty"`
	Materialized []string `yaml:"materialized,omitempty"`

	path string `yaml:"-"` // where this file was loaded from / saves to
}

// New returns an empty state file pinned to a save path.
func New(path string) *File {
	return &File{Version: CurrentVersion, path: path}
}

// Load reads and parses the state file at path. A missing or empty file yields
// a valid empty state (version set) and no error, so a freshly-initialized
// workspace works without an explicit state file.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return New(path), nil
		}
		return nil, fmt.Errorf("read state file %q: %w", path, err)
	}
	f := &File{path: path}
	if len(data) == 0 {
		f.Version = CurrentVersion
		return f, nil
	}
	if err := yaml.Unmarshal(data, f); err != nil {
		return nil, fmt.Errorf("parse state file %q: %w", path, err)
	}
	if f.Version == 0 {
		f.Version = CurrentVersion
	}
	return f, nil
}

// Path returns the absolute path this file was loaded from / will save to.
func (f *File) Path() string { return f.path }

// SetPath pins the save path (used when constructing a File in memory).
func (f *File) SetPath(path string) { f.path = path }

// SetLayers replaces the applied layer stack.
func (f *File) SetLayers(layers []string) {
	f.Layers = append([]string(nil), layers...)
}

// SetOrder replaces the full saved layer ordering (every known overlay layer,
// including disabled ones, lowest priority first). The TUI persists this so
// disabling a layer does not lose its position; it is display metadata only
// and does not affect merge priority (which comes from Layers).
func (f *File) SetOrder(order []string) {
	f.Order = append([]string(nil), order...)
}

// SetMaterialized replaces the materialized-file manifest.
func (f *File) SetMaterialized(paths []string) {
	f.Materialized = append([]string(nil), paths...)
}

// AddMaterialized appends a path to the manifest if not already present.
func (f *File) AddMaterialized(path string) {
	for _, p := range f.Materialized {
		if p == path {
			return
		}
	}
	f.Materialized = append(f.Materialized, path)
}

// MergeOrder combines a previously saved layer ordering (prev) with a new set of
// applied layers (applied, in priority order) using a stable preserve-positions
// strategy: non-applied layers keep their exact positions in prev, while the
// applied layers are re-sequenced into their CLI priority order and placed into
// the remaining slots. Brand-new applied layers (not present in prev) extend the
// list.
//
// The applied layers always end up in their CLI relative order in the result, so
// a subsequent no-op TUI apply reproduces the same priority. This is used by
// `env-man apply` to overwrite the saved order while retaining a resemblance of
// the TUI ordering for layers it does not touch. It is pure and symmetric with
// respect to base (callers must exclude the reserved base layer).
func MergeOrder(prev, applied []string) []string {
	// Dedup applied, preserving priority order.
	appliedSet := make(map[string]bool, len(applied))
	A := make([]string, 0, len(applied))
	for _, l := range applied {
		if appliedSet[l] {
			continue
		}
		appliedSet[l] = true
		A = append(A, l)
	}

	// uniquePrev: prev deduped (first occurrence), preserving order.
	prevSeen := make(map[string]bool, len(prev))
	uniquePrev := make([]string, 0, len(prev))
	for _, l := range prev {
		if prevSeen[l] {
			continue
		}
		prevSeen[l] = true
		uniquePrev = append(uniquePrev, l)
	}

	// Brand-new applied layers (not in prev) extend the list.
	newCount := 0
	for _, l := range A {
		if !prevSeen[l] {
			newCount++
		}
	}
	size := len(uniquePrev) + newCount

	result := make([]string, size)
	filled := make([]bool, size)

	// Pin non-applied prev layers to their prev positions; applied layers leave
	// their slots empty to be refilled in priority order below.
	for i, l := range uniquePrev {
		if !appliedSet[l] {
			result[i] = l
			filled[i] = true
		}
	}

	// Sweep empty slots in increasing index and drop the applied layers in.
	j := 0
	for _, l := range A {
		for filled[j] {
			j++
		}
		result[j] = l
		filled[j] = true
	}
	return result
}

// Save writes the state file atomically at its configured path.
func (f *File) Save() error {
	if f.path == "" {
		return fmt.Errorf("state file path is empty")
	}
	if f.Version == 0 {
		f.Version = CurrentVersion
	}
	out, err := yaml.Marshal(f)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	if err := fileutil.WriteFileAtomic(f.path, out, 0o644); err != nil {
		return fmt.Errorf("write state file: %w", err)
	}
	return nil
}
