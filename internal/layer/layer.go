// Package layer validates and discovers env-man layer folders.
package layer

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// NamePattern is the regular expression a legal layer folder name must match.
const NamePattern = `^[a-zA-Z0-9_-]+$`

// BaseName is the reserved always-on bottom layer.
const BaseName = "base"

var nameRe = regexp.MustCompile(NamePattern)

// ErrInvalidName is returned by Validate for an illegal layer name.
var ErrInvalidName = errors.New("invalid layer name")

// Validate returns an error if name is not a legal layer folder name.
// The reserved base name also satisfies this pattern.
func Validate(name string) error {
	if name == "" {
		return fmt.Errorf("%w: name is empty", ErrInvalidName)
	}
	if !nameRe.MatchString(name) {
		return fmt.Errorf("%w: %q must match %s", ErrInvalidName, name, NamePattern)
	}
	return nil
}

// IsReserved reports whether name is a reserved layer name (currently "base").
func IsReserved(name string) bool {
	return name == BaseName
}

// Discover returns the sorted names of every layer folder directly under
// envManDir (including base). Non-directory entries and hidden folders (those
// beginning with '.') are skipped. The result is sorted and deduplicated.
func Discover(envManDir string) ([]string, error) {
	entries, err := os.ReadDir(envManDir)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", envManDir, err)
	}
	seen := map[string]bool{}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}
