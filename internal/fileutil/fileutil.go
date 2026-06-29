// Package fileutil provides small filesystem helpers shared across packages.
package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteFileAtomic writes data to filename atomically: a temp file is created in
// the same directory as filename and then renamed into place. Keeping the temp
// file in the same directory ensures os.Rename works on both Windows and Linux
// without cross-device rename errors. Existing files are replaced.
func WriteFileAtomic(filename string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %q: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".env-man-tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file in %q: %w", dir, err)
	}
	tmpName := tmp.Name()

	abort := func(reason error) error {
		tmp.Close()
		_ = os.Remove(tmpName)
		return reason
	}

	if _, err := tmp.Write(data); err != nil {
		return abort(fmt.Errorf("write temp file: %w", err))
	}
	if err := tmp.Sync(); err != nil {
		return abort(fmt.Errorf("sync temp file: %w", err))
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, filename); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename temp file to %q: %w", filename, err)
	}
	return nil
}

// PruneEmptyDirs removes empty directories starting from dir and walking
// upwards, stopping at (and never removing) base. A non-empty directory stops
// the walk. Errors are swallowed: this is best-effort cleanup.
//
// dir must be inside base; if not, the call is a no-op.
func PruneEmptyDirs(base, dir string) {
	for {
		if dir == base || dir == "" {
			return
		}
		// Guard against reaching the filesystem root (Dir(x) == x).
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		rel, err := filepath.Rel(base, dir)
		if err != nil || strings.HasPrefix(rel, "..") || rel == "." {
			return // dir is at or outside base
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = parent
	}
}
