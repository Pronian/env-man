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
