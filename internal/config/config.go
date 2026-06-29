// Package config resolves the on-disk layout of an env-man workspace.
//
// env-man never searches upward: .env-man/ must live in the working directory
// (overridable via the global --cwd flag).
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Well-known names within the .env-man/ directory.
const (
	DirName   = ".env-man"  // the configuration folder
	StateName = "state.yaml" // applied-layer state file
	BaseName  = "base"      // the always-on bottom layer
)

// Paths describes the resolved env-man layout for a workspace.
type Paths struct {
	Workdir   string // absolute path to the workspace directory
	Root      string // <workdir>/.env-man
	BaseDir   string // <workdir>/.env-man/base
	StateFile string // <workdir>/.env-man/state.yaml
}

// ErrNotConfigured is returned when the working directory has no .env-man/.
var ErrNotConfigured = errors.New("no .env-man configuration in this directory (run `env-man init` first)")

// ResolveWorkdir returns the absolute working directory, honoring an explicit
// override. If cwd is empty, the process working directory is used.
func ResolveWorkdir(cwd string) (string, error) {
	if cwd == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("determine working directory: %w", err)
		}
		cwd = wd
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path of %q: %w", cwd, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("working directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("working directory %q is not a directory", abs)
	}
	return abs, nil
}

// New builds Paths for an absolute working directory.
func New(absWorkdir string) Paths {
	root := filepath.Join(absWorkdir, DirName)
	return Paths{
		Workdir:   absWorkdir,
		Root:      root,
		BaseDir:   filepath.Join(root, BaseName),
		StateFile: filepath.Join(root, StateName),
	}
}

// Require resolves the workspace and returns Paths, or an error if .env-man/ is
// missing. cwd follows the same rules as ResolveWorkdir.
func Require(cwd string) (Paths, error) {
	wd, err := ResolveWorkdir(cwd)
	if err != nil {
		return Paths{}, err
	}
	p := New(wd)
	info, err := os.Stat(p.Root)
	if err != nil {
		if os.IsNotExist(err) {
			return p, ErrNotConfigured
		}
		return p, fmt.Errorf("stat %q: %w", p.Root, err)
	}
	if !info.IsDir() {
		return p, ErrNotConfigured
	}
	return p, nil
}

// LayerDir returns the on-disk path of a layer folder by name.
func (p Paths) LayerDir(name string) string {
	return filepath.Join(p.Root, name)
}
