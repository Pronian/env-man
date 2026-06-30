package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_MissingFileReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.yaml")
	f, err := Load(path)
	require.NoError(t, err)
	require.NotNil(t, f)
	assert.Equal(t, CurrentVersion, f.Version)
	assert.Empty(t, f.Layers)
	assert.Empty(t, f.Materialized)
	assert.Equal(t, path, f.Path())
}

func TestLoad_EmptyFileReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.yaml")
	require.NoError(t, os.WriteFile(path, []byte{}, 0o644))
	f, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, CurrentVersion, f.Version)
	assert.Empty(t, f.Layers)
}

func TestLoad_MalformedReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.yaml")
	require.NoError(t, os.WriteFile(path, []byte("layers: [unclosed"), 0o644))
	_, err := Load(path)
	require.Error(t, err)
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.yaml")
	original := New(path)
	original.SetLayers([]string{"dev", "local"})
	original.SetMaterialized([]string{".env", "config/db.yml"})

	require.NoError(t, original.Save())
	assert.FileExists(t, path)

	loaded, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, CurrentVersion, loaded.Version)
	assert.Equal(t, []string{"dev", "local"}, loaded.Layers)
	assert.Equal(t, []string{".env", "config/db.yml"}, loaded.Materialized)
}

func TestSave_WritesVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.yaml")
	f := New(path)
	require.NoError(t, f.Save())
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "version: 1")
}

func TestSetLayers_DoesNotMutateCallerSlice(t *testing.T) {
	f := New(filepath.Join(t.TempDir(), "state.yaml"))
	in := []string{"a", "b"}
	f.SetLayers(in)
	in[0] = "zzz"
	assert.Equal(t, []string{"a", "b"}, f.Layers)
}

func TestSetOrder_DoesNotMutateCallerSlice(t *testing.T) {
	f := New(filepath.Join(t.TempDir(), "state.yaml"))
	in := []string{"a", "b"}
	f.SetOrder(in)
	in[0] = "zzz"
	assert.Equal(t, []string{"a", "b"}, f.Order)
}

func TestSaveLoad_OrderRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.yaml")
	original := New(path)
	original.SetLayers([]string{"dev"})
	original.SetOrder([]string{"dev", "staging", "local"})
	original.SetMaterialized([]string{".env"})

	require.NoError(t, original.Save())

	loaded, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"dev", "staging", "local"}, loaded.Order)
	assert.Equal(t, []string{"dev"}, loaded.Layers)
}

func TestMergeOrder_AppliedReorderedNonAppliedPinned(t *testing.T) {
	// prev: dev staging local prod ; apply local dev
	// non-applied (staging, prod) keep positions; applied slots refilled in
	// CLI priority order (local before dev).
	prev := []string{"dev", "staging", "local", "prod"}
	got := MergeOrder(prev, []string{"local", "dev"})
	assert.Equal(t, []string{"local", "staging", "dev", "prod"}, got)
}

func TestMergeOrder_PreservesPositionsWhenPriorityFlips(t *testing.T) {
	// prev already has local before dev; applying dev local must put dev first
	// while staging/prod stay put.
	prev := []string{"local", "dev", "staging", "prod"}
	got := MergeOrder(prev, []string{"dev", "local"})
	assert.Equal(t, []string{"dev", "local", "staging", "prod"}, got)
}

func TestMergeOrder_SingleAppliedStaysInPlace(t *testing.T) {
	prev := []string{"dev", "staging", "local", "prod"}
	got := MergeOrder(prev, []string{"staging"})
	assert.Equal(t, prev, got)
}

func TestMergeOrder_NewAppliedExtendsList(t *testing.T) {
	// staging is brand-new (not in prev); it leads the applied priority order
	// and fills the first available slot, dev keeps its slot, local is pinned.
	prev := []string{"dev", "local"}
	got := MergeOrder(prev, []string{"staging", "dev"})
	assert.Equal(t, []string{"staging", "local", "dev"}, got)
	// applied subsequence must equal the CLI order.
	assert.Equal(t, []string{"staging", "dev"}, appliedSubsequence(t, got, []string{"staging", "dev"}))
}

func TestMergeOrder_EmptyAppliedKeepsPrev(t *testing.T) {
	// apply with no overlays: nothing applied, prev order untouched.
	prev := []string{"dev", "staging", "local"}
	got := MergeOrder(prev, nil)
	assert.Equal(t, prev, got)
}

func TestMergeOrder_EmptyPrevReturnsApplied(t *testing.T) {
	got := MergeOrder(nil, []string{"dev", "local"})
	assert.Equal(t, []string{"dev", "local"}, got)
}

func TestMergeOrder_DedupsPrevAndApplied(t *testing.T) {
	got := MergeOrder([]string{"dev", "dev", "local"}, []string{"local", "local", "dev"})
	assert.Equal(t, []string{"local", "dev"}, got)
}

// appliedSubsequence returns got entries that appear in applied, preserving
// got's order (used to assert the applied layers end up in priority order).
func appliedSubsequence(t *testing.T, got, applied []string) []string {
	t.Helper()
	set := make(map[string]bool, len(applied))
	for _, l := range applied {
		set[l] = true
	}
	var out []string
	for _, l := range got {
		if set[l] {
			out = append(out, l)
		}
	}
	return out
}

func TestAddMaterialized_Idempotent(t *testing.T) {
	f := New(filepath.Join(t.TempDir(), "state.yaml"))
	f.AddMaterialized(".env")
	f.AddMaterialized(".env")
	f.AddMaterialized("config/db.yml")
	assert.Equal(t, []string{".env", "config/db.yml"}, f.Materialized)
}

func TestSave_EmptyPathErrors(t *testing.T) {
	f := &File{}
	err := f.Save()
	require.Error(t, err)
}

func TestSave_CreatesParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "state.yaml")
	f := New(path)
	f.SetLayers([]string{"dev"})
	require.NoError(t, f.Save())
	assert.FileExists(t, path)
}
