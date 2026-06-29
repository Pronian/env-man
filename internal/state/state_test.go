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
