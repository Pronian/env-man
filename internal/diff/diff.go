// Package diff computes a dry-run preview of what an apply would change,
// without writing anything. It powers the `env-man diff` command.
package diff

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"env-man/internal/config"
	"env-man/internal/materialize"
	"env-man/internal/merge"
	"env-man/internal/state"

	"gopkg.in/yaml.v3"
)

// Status is the change classification for a file or a key.
type Status string

const (
	StatusAdded     Status = "added"
	StatusModified  Status = "modified"
	StatusUnchanged Status = "unchanged"
	StatusRemoved   Status = "removed"
)

// KeyDelta is a single dotted-path change inside a mergeable file.
type KeyDelta struct {
	Path   string
	Status Status
}

// FileDiff is the diff of a single workspace-relative file.
type FileDiff struct {
	RelPath   string
	Status    Status
	Strategy  merge.Strategy
	Old, New  []byte
	KeyDeltas []KeyDelta // populated only for modified mergeable files
}

// HasChanged reports whether the file is added, modified, or removed.
func (f FileDiff) HasChanged() bool {
	return f.Status != StatusUnchanged
}

// Report is the full diff result.
type Report struct {
	Plan  *materialize.Plan
	Files []FileDiff // sorted by RelPath
}

// Changed reports how many files have a non-unchanged status.
func (r Report) Changed() int {
	n := 0
	for _, f := range r.Files {
		if f.HasChanged() {
			n++
		}
	}
	return n
}

// Compute builds a dry-run report for applying `layers` (the current state's
// layers if layers is nil). No files are written.
func Compute(p config.Paths, layers []string) (*Report, error) {
	plan, err := materialize.BuildPlan(p, layers)
	if err != nil {
		return nil, err
	}
	st, err := state.Load(p.StateFile)
	if err != nil {
		return nil, err
	}

	report := &Report{Plan: plan}
	proposed := map[string]bool{}
	for _, fc := range plan.Files {
		proposed[fc.RelPath] = true
		target := filepath.Join(p.Workdir, filepath.FromSlash(fc.RelPath))
		old, err := os.ReadFile(target)
		exists := true
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("read %q: %w", fc.RelPath, err)
			}
			exists = false
			old = nil
		}
		fd := FileDiff{RelPath: fc.RelPath, Strategy: fc.Strategy, New: fc.Content, Old: old}
		switch {
		case !exists:
			fd.Status = StatusAdded
		case bytes.Equal(old, fc.Content):
			fd.Status = StatusUnchanged
		default:
			fd.Status = StatusModified
			fd.KeyDeltas = computeKeyDeltas(fc.RelPath, fc.Strategy, old, fc.Content)
		}
		report.Files = append(report.Files, fd)
	}

	// Files in the current manifest that the new stack no longer produces.
	for _, rel := range st.Materialized {
		if proposed[rel] {
			continue
		}
		report.Files = append(report.Files, FileDiff{
			RelPath:  rel,
			Status:   StatusRemoved,
			Strategy: merge.StrategyFor(rel),
		})
	}
	sort.Slice(report.Files, func(i, j int) bool {
		return report.Files[i].RelPath < report.Files[j].RelPath
	})
	return report, nil
}

func computeKeyDeltas(relpath string, strat merge.Strategy, old, new []byte) []KeyDelta {
	switch strat {
	case merge.StrategyEnv:
		return envKeyDeltas(old, new)
	case merge.StrategyJSON:
		return structuredKeyDeltas(old, new, flattenJSON)
	case merge.StrategyYAML:
		return structuredKeyDeltas(old, new, flattenYAML)
	default:
		return nil
	}
}

func envKeyDeltas(oldB, newB []byte) []KeyDelta {
	oldMap, err := merge.ParseEnv(oldB)
	if err != nil || len(oldMap) == 0 && len(newB) == 0 {
		oldMap = map[string]string{}
	}
	newMap, err := merge.ParseEnv(newB)
	if err != nil {
		return nil
	}
	return deltaStringMaps(oldMap, newMap)
}

func structuredKeyDeltas(oldB, newB []byte, flatten func([]byte) (map[string]string, error)) []KeyDelta {
	oldMap, err := flatten(oldB)
	if err != nil {
		return nil
	}
	newMap, err := flatten(newB)
	if err != nil {
		return nil
	}
	return deltaStringMaps(oldMap, newMap)
}

func deltaStringMaps(oldMap, newMap map[string]string) []KeyDelta {
	var deltas []KeyDelta
	keys := unionKeys(oldMap, newMap)
	for _, k := range keys {
		o, okO := oldMap[k]
		n, okN := newMap[k]
		switch {
		case okN && !okO:
			deltas = append(deltas, KeyDelta{Path: k, Status: StatusAdded})
		case okO && !okN:
			deltas = append(deltas, KeyDelta{Path: k, Status: StatusRemoved})
		case o != n:
			deltas = append(deltas, KeyDelta{Path: k, Status: StatusModified})
		}
	}
	return deltas
}

func unionKeys(maps ...map[string]string) []string {
	set := map[string]bool{}
	for _, m := range maps {
		for k := range m {
			set[k] = true
		}
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// flattenJSON decodes a JSON document and flattens nested objects into dotted
// paths; arrays and scalars serialize to a stable string for comparison.
func flattenJSON(b []byte) (map[string]string, error) {
	if len(bytes.TrimSpace(b)) == 0 {
		return map[string]string{}, nil
	}
	var v any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	out := map[string]string{}
	if m, ok := v.(map[string]any); ok {
		flattenAny("", m, out)
	} else {
		out[""] = serializeLeaf(v)
	}
	return out, nil
}

// flattenYAML decodes a YAML document and flattens nested mappings similarly.
func flattenYAML(b []byte) (map[string]string, error) {
	if len(bytes.TrimSpace(b)) == 0 {
		return map[string]string{}, nil
	}
	var v any
	if err := yaml.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	out := map[string]string{}
	if m, ok := v.(map[string]any); ok {
		flattenAny("", m, out)
	} else {
		out[""] = serializeLeaf(v)
	}
	return out, nil
}

func flattenAny(prefix string, m map[string]any, out map[string]string) {
	for k, val := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if nested, ok := val.(map[string]any); ok {
			flattenAny(key, nested, out)
		} else {
			out[key] = serializeLeaf(val)
		}
	}
}

// serializeLeaf renders a scalar/array/null to a comparison string.
func serializeLeaf(v any) string {
	if v == nil {
		return "null"
	}
	return fmt.Sprintf("%v", v)
}
