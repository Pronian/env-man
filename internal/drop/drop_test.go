package drop_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"env-man/internal/config"
	"env-man/internal/drop"
	"env-man/internal/materialize"
	"env-man/internal/state"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupWorkspace(t *testing.T) config.Paths {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.DirName, config.BaseName), 0o755))
	p := config.New(dir)
	require.NoError(t, state.New(p.StateFile).Save())
	return p
}

func writeLayerFile(t *testing.T, p config.Paths, layerName, relpath, content string) {
	t.Helper()
	full := filepath.Join(p.LayerDir(layerName), filepath.FromSlash(relpath))
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

func ensureLayerDir(t *testing.T, p config.Paths, layerName string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(p.LayerDir(layerName), 0o755))
}

func wsPath(p config.Paths, relpath string) string {
	return filepath.Join(p.Workdir, filepath.FromSlash(relpath))
}

// applyOnce is a helper that loads fresh state, builds a plan, and applies it.
func applyOnce(t *testing.T, p config.Paths, layers []string) {
	t.Helper()
	st, err := state.Load(p.StateFile)
	require.NoError(t, err)
	plan, err := materialize.BuildPlan(p, layers)
	require.NoError(t, err)
	_, err = materialize.Apply(p, st, plan)
	require.NoError(t, err)
}

func TestDrop_RemovesAllMaterializedFiles(t *testing.T) {
	p := setupWorkspace(t)
	writeLayerFile(t, p, "base", ".env", "A=1\n")
	writeLayerFile(t, p, "base", "config/app.json", `{"a":1}`+"\n")
	ensureLayerDir(t, p, "dev")
	writeLayerFile(t, p, "dev", "config/app.json", `{"b":2}`+"\n")
	writeLayerFile(t, p, "dev", "only-dev.txt", "dev\n")
	applyOnce(t, p, []string{"dev"})

	res, err := drop.Run(p)
	require.NoError(t, err)
	sort.Strings(res.Removed)
	assert.Equal(t, []string{".env", "config/app.json", "only-dev.txt"}, res.Removed)

	assert.NoFileExists(t, wsPath(p, ".env"))
	assert.NoFileExists(t, wsPath(p, "config/app.json"))
	assert.NoFileExists(t, wsPath(p, "only-dev.txt"))
	// empty dirs pruned
	assert.NoDirExists(t, wsPath(p, "config"))
}

func TestDrop_PreservesEnvManFolder(t *testing.T) {
	p := setupWorkspace(t)
	writeLayerFile(t, p, "base", ".env", "A=1\n")
	applyOnce(t, p, nil)

	_, err := drop.Run(p)
	require.NoError(t, err)

	// .env-man/ untouched
	assert.DirExists(t, p.Root)
	assert.DirExists(t, p.BaseDir)
	assert.FileExists(t, p.StateFile)
	assert.FileExists(t, filepath.Join(p.BaseDir, ".env"))
}

func TestDrop_ClearsState(t *testing.T) {
	p := setupWorkspace(t)
	writeLayerFile(t, p, "base", ".env", "A=1\n")
	ensureLayerDir(t, p, "dev")
	writeLayerFile(t, p, "dev", ".env", "B=2\n")
	applyOnce(t, p, []string{"dev"})

	_, err := drop.Run(p)
	require.NoError(t, err)

	loaded, err := state.Load(p.StateFile)
	require.NoError(t, err)
	assert.Empty(t, loaded.Layers)
	assert.Empty(t, loaded.Materialized)
	assert.Equal(t, state.CurrentVersion, loaded.Version)
}

func TestDrop_ToleratesMissingFiles(t *testing.T) {
	p := setupWorkspace(t)
	writeLayerFile(t, p, "base", ".env", "A=1\n")
	applyOnce(t, p, nil)

	// manually delete the materialized file (simulating user cleanup)
	require.NoError(t, os.Remove(wsPath(p, ".env")))

	res, err := drop.Run(p)
	require.NoError(t, err)
	assert.Empty(t, res.Removed)
	assert.Equal(t, []string{".env"}, res.Missing)

	// state still cleared
	loaded, err := state.Load(p.StateFile)
	require.NoError(t, err)
	assert.Empty(t, loaded.Materialized)
}

func TestDrop_OnNothingAppliedIsNoop(t *testing.T) {
	p := setupWorkspace(t)
	res, err := drop.Run(p)
	require.NoError(t, err)
	assert.Empty(t, res.Removed)
	assert.Empty(t, res.Missing)
}

func TestDrop_RoundTripStability(t *testing.T) {
	// apply -> capture -> drop -> re-apply must yield byte-identical files.
	p := setupWorkspace(t)
	writeLayerFile(t, p, "base", ".env", "A=1\nB=base\n")
	writeLayerFile(t, p, "base", "config/db.yaml", "host: localhost\nport: 5432\n")
	ensureLayerDir(t, p, "dev")
	writeLayerFile(t, p, "dev", ".env", "B=dev\nC=2\n")
	writeLayerFile(t, p, "dev", "config/db.yaml", "port: 6543\nssl: true\n")

	applyOnce(t, p, []string{"dev"})

	paths := []string{".env", "config/db.yaml"}
	snapshot := map[string]string{}
	for _, rp := range paths {
		b, err := os.ReadFile(wsPath(p, rp))
		require.NoError(t, err)
		snapshot[rp] = string(b)
	}

	_, err := drop.Run(p)
	require.NoError(t, err)
	for _, rp := range paths {
		assert.NoFileExists(t, wsPath(p, rp))
	}

	applyOnce(t, p, []string{"dev"})
	for _, rp := range paths {
		b, err := os.ReadFile(wsPath(p, rp))
		require.NoError(t, err)
		assert.Equal(t, snapshot[rp], string(b), "re-apply changed content of %q", rp)
	}
}
