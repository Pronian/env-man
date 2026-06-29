package fileutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFileAtomic_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")
	require.NoError(t, WriteFileAtomic(target, []byte("hello"), 0o644))

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got))
}

func TestWriteFileAtomic_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "nested", "deep", "out.txt")
	require.NoError(t, WriteFileAtomic(target, []byte("x"), 0o644))
	assert.FileExists(t, target)
}

func TestWriteFileAtomic_ReplacesExisting(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o644))

	require.NoError(t, WriteFileAtomic(target, []byte("new content"), 0o644))
	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "new content", string(got))
}

func TestWriteFileAtomic_DoesNotLeaveTempFiles(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")
	require.NoError(t, WriteFileAtomic(target, []byte("data"), 0o644))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.False(t, len(e.Name()) > 0 && e.Name()[0] == '.',
			"unexpected leftover temp entry: %q", e.Name())
	}
}

func TestPruneEmptyDirs_RemovesEmptyAncestors(t *testing.T) {
	base := t.TempDir()
	deep := filepath.Join(base, "a", "b", "c")
	require.NoError(t, os.MkdirAll(deep, 0o755))

	PruneEmptyDirs(base, deep)

	assert.NoDirExists(t, filepath.Join(base, "a"))
	assert.NoDirExists(t, filepath.Join(base, "a", "b"))
}

func TestPruneEmptyDirs_StopsAtNonEmpty(t *testing.T) {
	base := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(base, "a", "b"), 0o755))
	// keep a file in a/ so a/ is non-empty
	require.NoError(t, os.WriteFile(filepath.Join(base, "a", "keep.txt"), []byte("x"), 0o644))

	PruneEmptyDirs(base, filepath.Join(base, "a", "b"))

	assert.NoDirExists(t, filepath.Join(base, "a", "b"))
	assert.DirExists(t, filepath.Join(base, "a"))
}

func TestPruneEmptyDirs_NeverRemovesBase(t *testing.T) {
	base := t.TempDir()
	PruneEmptyDirs(base, base)
	_, err := os.Stat(base)
	assert.NoError(t, err)
}

func TestPruneEmptyDirs_OutsideBaseIsNoop(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()
	PruneEmptyDirs(base, outside) // must not touch outside
	_, err := os.Stat(outside)
	assert.NoError(t, err)
}
