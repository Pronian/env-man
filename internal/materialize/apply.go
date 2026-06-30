// Package materialize resolves a layer stack into concrete file changes and
// writes them into the workspace (outside .env-man/).
//
// BuildPlan computes the merged content for every file across the stack without
// touching the filesystem; it is shared by `apply` and `diff`. Apply executes a
// plan: it writes new/changed files, removes files orphaned by the previous
// stack, prunes empty directories, and updates state.yaml.
package materialize

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"env-man/internal/config"
	"env-man/internal/fileutil"
	"env-man/internal/layer"
	"env-man/internal/merge"
	"env-man/internal/state"
)

// StackEntry is a layer in the resolved stack, lowest priority first.
type StackEntry struct {
	Name string // layer name (base for the implicit bottom layer)
	Dir  string // absolute path to the layer folder
}

// FileChange is a single file to be materialized.
type FileChange struct {
	RelPath  string          // workspace-relative path, forward slashes
	Content  []byte          // merged content
	Strategy merge.Strategy  // merge strategy used
	Sources  int             // number of layers that contributed content
}

// Plan is a resolved, validated set of file changes. It performs no I/O.
type Plan struct {
	Stack []StackEntry
	Files []FileChange // sorted by RelPath
}

// Result reports what Apply changed.
type Result struct {
	Created   []string // files written that did not previously exist
	Updated   []string // files whose content changed
	Unchanged []string // files whose content was already up to date
	Removed   []string // files removed that were orphaned by the new stack
}

// BuildPlan validates the layer names and their folders, then computes the
// merged content for every file across [base, layers...]. It performs no
// writes.
func BuildPlan(p config.Paths, layers []string) (*Plan, error) {
	stack, err := resolveStack(p, layers)
	if err != nil {
		return nil, err
	}

	// Union of relpaths across the whole stack (preserving none; we sort).
	relSet := map[string]bool{}
	for _, se := range stack {
		paths, err := collectRelpaths(se.Dir)
		if err != nil {
			return nil, fmt.Errorf("scan layer %q: %w", se.Name, err)
		}
		for _, rp := range paths {
			relSet[rp] = true
		}
	}
	relpaths := sortedKeys(relSet)

	plan := &Plan{Stack: stack}
	for _, rp := range relpaths {
		var contents [][]byte
		for _, se := range stack {
			data, err := os.ReadFile(filepath.Join(se.Dir, filepath.FromSlash(rp)))
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("read %q in layer %q: %w", rp, se.Name, err)
			}
			contents = append(contents, data)
		}
		merged, err := merge.Merge(rp, contents)
		if err != nil {
			return nil, fmt.Errorf("merge %q: %w", rp, err)
		}
		if len(merged) == 0 {
			continue
		}
		plan.Files = append(plan.Files, FileChange{
			RelPath:  rp,
			Content:  merged,
			Strategy: merge.StrategyFor(rp),
			Sources:  len(nonEmpty(contents)),
		})
	}
	return plan, nil
}

// resolveStack validates the layer names and folders and returns the full stack
// [base, layers...].
func resolveStack(p config.Paths, layers []string) ([]StackEntry, error) {
	stack := make([]StackEntry, 0, len(layers)+1)
	stack = append(stack, StackEntry{Name: config.BaseName, Dir: p.BaseDir})

	seen := map[string]bool{config.BaseName: true}
	for _, name := range layers {
		if err := layer.Validate(name); err != nil {
			return nil, fmt.Errorf("layer %q: %w", name, err)
		}
		if layer.IsReserved(name) {
			return nil, fmt.Errorf("layer %q: base is implicit and cannot be listed explicitly", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("layer %q: listed more than once", name)
		}
		dir := p.LayerDir(name)
		info, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("layer %q: folder not found at %s", name, dir)
			}
			return nil, fmt.Errorf("layer %q: %w", name, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("layer %q: %s is not a directory", name, dir)
		}
		seen[name] = true
		stack = append(stack, StackEntry{Name: name, Dir: dir})
	}
	return stack, nil
}

// collectRelpaths returns the forward-slash relative paths of every regular
// file under dir. A missing dir yields an empty slice and no error (the layer
// contributes nothing).
func collectRelpaths(dir string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	return paths, err
}

// Apply writes the plan's files into the workspace, removes files that the new
// stack no longer produces (orphans from the previous manifest), prunes empty
// directories, and persists the new layer stack and manifest to state.yaml.
func Apply(p config.Paths, st *state.File, plan *Plan) (*Result, error) {
	res := &Result{}

	newSet := map[string]bool{}
	for _, fc := range plan.Files {
		newSet[fc.RelPath] = true
		target := filepath.Join(p.Workdir, filepath.FromSlash(fc.RelPath))
		existed := pathExists(target)

		changed, err := writeFileIfChanged(target, fc.Content, 0o644)
		if err != nil {
			return nil, fmt.Errorf("write %q: %w", fc.RelPath, err)
		}
		switch {
		case !existed:
			res.Created = append(res.Created, fc.RelPath)
		case !changed:
			res.Unchanged = append(res.Unchanged, fc.RelPath)
		default:
			res.Updated = append(res.Updated, fc.RelPath)
		}
	}

	// Remove orphans: in the previous manifest but not produced by this plan.
	for _, old := range st.Materialized {
		if newSet[old] {
			continue
		}
		target := filepath.Join(p.Workdir, filepath.FromSlash(old))
		if err := os.Remove(target); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("remove orphan %q: %w", old, err)
		}
		res.Removed = append(res.Removed, old)
		fileutil.PruneEmptyDirs(p.Workdir, filepath.Dir(target))
	}

	st.SetLayers(layerNamesExcludingBase(plan.Stack))
	st.SetMaterialized(sortedKeys(newSet))
	if err := st.Save(); err != nil {
		return nil, fmt.Errorf("save state: %w", err)
	}
	return res, nil
}

// writeFileIfChanged writes content to path only when it differs from the
// current content, returning whether a write occurred.
func writeFileIfChanged(path string, content []byte, perm os.FileMode) (bool, error) {
	if existing, err := os.ReadFile(path); err == nil {
		if bytes.Equal(existing, content) {
			return false, nil
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if err := fileutil.WriteFileAtomic(path, content, perm); err != nil {
		return false, err
	}
	return true, nil
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func layerNamesExcludingBase(stack []StackEntry) []string {
	var names []string
	for _, se := range stack {
		if se.Name == config.BaseName {
			continue
		}
		names = append(names, se.Name)
	}
	return names
}

func nonEmpty(in [][]byte) [][]byte {
	out := make([][]byte, 0, len(in))
	for _, c := range in {
		if len(c) > 0 {
			out = append(out, c)
		}
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
