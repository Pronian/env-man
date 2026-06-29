package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"env-man/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runIn runs the root command with --cwd=<dir> and the given subcommand args,
// returning captured stdout/stderr and any error.
func runIn(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"--cwd", dir}, args...))
	err := root.Execute()
	return out.String(), err
}

// freshDir returns an empty temp directory.
func freshDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// writeFile writes a file (creating parent dirs) under dir.
func writeFile(t *testing.T, dir, relpath, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(relpath))
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

func exists(t *testing.T, dir, relpath string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(relpath)))
	return err == nil
}

func TestInit_CreatesScaffold(t *testing.T) {
	dir := freshDir(t)
	out, err := runIn(t, dir, "init")
	require.NoError(t, err)
	assert.Contains(t, out, "Initialized env-man")

	assert.True(t, exists(t, dir, config.DirName+"/"+config.StateName))
	assert.True(t, exists(t, dir, config.DirName+"/"+config.BaseName+"/.env"))
}

func TestInit_Idempotent(t *testing.T) {
	dir := freshDir(t)
	_, err := runIn(t, dir, "init")
	require.NoError(t, err)

	out, err := runIn(t, dir, "init")
	require.NoError(t, err)
	assert.Contains(t, out, "already initialized")
}

func TestInit_HealsMissingState(t *testing.T) {
	dir := freshDir(t)
	// .env-man/ and base/.env exist, but state.yaml is missing.
	writeFile(t, dir, config.DirName+"/"+config.BaseName+"/.env", "A=1\n")

	out, err := runIn(t, dir, "init")
	require.NoError(t, err)
	assert.Contains(t, out, "state.yaml")
	assert.True(t, exists(t, dir, config.DirName+"/"+config.StateName))
}

func TestApply_MaterializesFiles(t *testing.T) {
	dir := freshDir(t)
	_, err := runIn(t, dir, "init")
	require.NoError(t, err)
	writeFile(t, dir, config.DirName+"/"+config.BaseName+"/.env", "A=1\nB=base\n")
	writeFile(t, dir, config.DirName+"/dev/.env", "B=dev\nC=2\n")

	out, err := runIn(t, dir, "apply", "dev")
	require.NoError(t, err)
	assert.Contains(t, out, "Applied layers: base -> dev")

	b, err := os.ReadFile(filepath.Join(dir, ".env"))
	require.NoError(t, err)
	assert.Equal(t, "A=1\nB=dev\nC=2\n", string(b))
}

func TestApply_NotConfiguredErrors(t *testing.T) {
	dir := freshDir(t)
	_, err := runIn(t, dir, "apply", "dev")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no .env-man configuration")
}

func TestApply_UnknownLayerErrors(t *testing.T) {
	dir := freshDir(t)
	_, err := runIn(t, dir, "init")
	require.NoError(t, err)

	_, err = runIn(t, dir, "apply", "ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	// nothing materialized
	assert.False(t, exists(t, dir, ".env"))
}

func TestApply_BaseOnly(t *testing.T) {
	dir := freshDir(t)
	_, err := runIn(t, dir, "init")
	require.NoError(t, err)
	writeFile(t, dir, config.DirName+"/"+config.BaseName+"/.env", "A=1\n")

	out, err := runIn(t, dir, "apply")
	require.NoError(t, err)
	assert.Contains(t, out, "Applied layers: base")
	assert.True(t, exists(t, dir, ".env"))
}

func TestDrop_RemovesFiles(t *testing.T) {
	dir := freshDir(t)
	_, err := runIn(t, dir, "init")
	require.NoError(t, err)
	writeFile(t, dir, config.DirName+"/"+config.BaseName+"/.env", "A=1\n")
	writeFile(t, dir, config.DirName+"/dev/.env", "B=2\n")
	_, err = runIn(t, dir, "apply", "dev")
	require.NoError(t, err)
	require.True(t, exists(t, dir, ".env"))

	out, err := runIn(t, dir, "drop")
	require.NoError(t, err)
	assert.Contains(t, out, "Dropped")
	assert.False(t, exists(t, dir, ".env"))
	// .env-man preserved
	assert.True(t, exists(t, dir, config.DirName+"/"+config.StateName))
}

func TestDrop_NothingApplied(t *testing.T) {
	dir := freshDir(t)
	_, err := runIn(t, dir, "init")
	require.NoError(t, err)

	out, err := runIn(t, dir, "drop")
	require.NoError(t, err)
	assert.Contains(t, out, "Nothing to drop")
}

func TestList_ShowsStackAndLayers(t *testing.T) {
	dir := freshDir(t)
	_, err := runIn(t, dir, "init")
	require.NoError(t, err)
	writeFile(t, dir, config.DirName+"/"+config.BaseName+"/.env", "A=1\n")
	writeFile(t, dir, config.DirName+"/dev/.env", "B=2\n")
	writeFile(t, dir, config.DirName+"/staging/.env", "B=3\n")
	_, err = runIn(t, dir, "apply", "dev")
	require.NoError(t, err)

	out, err := runIn(t, dir, "list")
	require.NoError(t, err)
	assert.Contains(t, out, "base")
	assert.Contains(t, out, "dev")
	assert.Contains(t, out, "staging")
	// dev applied (marked), staging available (unmarked)
	assert.Contains(t, out, "* dev")
	assert.Contains(t, out, "  staging")
}

func TestDiff_PreviewsChanges(t *testing.T) {
	dir := freshDir(t)
	_, err := runIn(t, dir, "init")
	require.NoError(t, err)
	writeFile(t, dir, config.DirName+"/"+config.BaseName+"/.env", "A=1\n")
	writeFile(t, dir, config.DirName+"/dev/.env", "B=2\n")

	out, err := runIn(t, dir, "diff", "dev")
	require.NoError(t, err)
	assert.Contains(t, out, "Proposed stack: base -> dev")
	assert.Contains(t, out, ".env")
	// A is added (base), B is added (dev) -> .env shows as added file
	assert.Contains(t, out, "added")
}

func TestDiff_NoChanges(t *testing.T) {
	dir := freshDir(t)
	_, err := runIn(t, dir, "init")
	require.NoError(t, err)
	writeFile(t, dir, config.DirName+"/"+config.BaseName+"/.env", "A=1\n")
	_, err = runIn(t, dir, "apply")
	require.NoError(t, err)

	out, err := runIn(t, dir, "diff")
	require.NoError(t, err)
	assert.Contains(t, out, "No changes.")
}

func TestRootDefault_NoConfigShowsMessage(t *testing.T) {
	dir := freshDir(t)
	out, err := runIn(t, dir) // no subcommand -> TUI default
	require.NoError(t, err)
	assert.Contains(t, out, "No env-man configuration")
	assert.Contains(t, out, "env-man init")
}

func TestDiff_ShowsKeyDeltas(t *testing.T) {
	dir := freshDir(t)
	_, err := runIn(t, dir, "init")
	require.NoError(t, err)
	writeFile(t, dir, config.DirName+"/"+config.BaseName+"/.env", "A=1\nB=2\n")
	_, err = runIn(t, dir, "apply")
	require.NoError(t, err)

	// Change base/.env, then diff against current on-disk state.
	writeFile(t, dir, config.DirName+"/"+config.BaseName+"/.env", "A=1\nB=20\nC=3\n")
	out, err := runIn(t, dir, "diff")
	require.NoError(t, err)
	assert.Contains(t, out, "modified .env")
	assert.Contains(t, out, "~ B")
	assert.Contains(t, out, "+ C")
	assert.False(t, strings.Contains(out, "+ A"))
}
