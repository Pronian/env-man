package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveWorkdir_DefaultUsesCWD(t *testing.T) {
	wd, err := ResolveWorkdir("")
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(wd))
	info, err := os.Stat(wd)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestResolveWorkdir_ExplicitOverride(t *testing.T) {
	dir := t.TempDir()
	wd, err := ResolveWorkdir(dir)
	require.NoError(t, err)
	assert.Equal(t, dir, wd)
}

func TestResolveWorkdir_MissingDirErrors(t *testing.T) {
	_, err := ResolveWorkdir(filepath.Join(t.TempDir(), "does-not-exist"))
	require.Error(t, err)
}

func TestNew_BuildsExpectedPaths(t *testing.T) {
	dir := t.TempDir()
	p := New(dir)
	assert.Equal(t, dir, p.Workdir)
	assert.Equal(t, filepath.Join(dir, DirName), p.Root)
	assert.Equal(t, filepath.Join(dir, DirName, BaseName), p.BaseDir)
	assert.Equal(t, filepath.Join(dir, DirName, StateName), p.StateFile)
}

func TestLayerDir(t *testing.T) {
	p := New(t.TempDir())
	assert.Equal(t, filepath.Join(p.Root, "dev"), p.LayerDir("dev"))
}

func TestRequire_MissingReturnsNotConfigured(t *testing.T) {
	dir := t.TempDir()
	_, err := Require(dir)
	require.ErrorIs(t, err, ErrNotConfigured)
}

func TestRequire_PresentSucceeds(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, DirName), 0o755))

	p, err := Require(dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, DirName), p.Root)
}

func TestRequire_FileAtEnvManPathIsNotConfigured(t *testing.T) {
	dir := t.TempDir()
	// A regular file named .env-man is not a configuration directory.
	require.NoError(t, os.WriteFile(filepath.Join(dir, DirName), []byte("x"), 0o644))
	_, err := Require(dir)
	require.ErrorIs(t, err, ErrNotConfigured)
}
