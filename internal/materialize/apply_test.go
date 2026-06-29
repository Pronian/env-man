package materialize_test

import (
	"os"
	"path/filepath"
	"testing"

	"env-man/internal/config"
	"env-man/internal/materialize"
	"env-man/internal/state"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupWorkspace creates a temp workspace with .env-man/base/ and an empty
// state.yaml. Returns resolved paths.
func setupWorkspace(t *testing.T) config.Paths {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.DirName, config.BaseName), 0o755))
	p := config.New(dir)
	st := state.New(p.StateFile)
	require.NoError(t, st.Save())
	return p
}

// writeLayerFile writes content into <layer>/<relpath> under .env-man/.
func writeLayerFile(t *testing.T, p config.Paths, layerName, relpath, content string) {
	t.Helper()
	full := filepath.Join(p.LayerDir(layerName), filepath.FromSlash(relpath))
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

// ensureLayerDir creates an (empty) layer folder.
func ensureLayerDir(t *testing.T, p config.Paths, layerName string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(p.LayerDir(layerName), 0o755))
}

// wsPath joins a forward-slash workspace-relative path to the workdir.
func wsPath(p config.Paths, relpath string) string {
	return filepath.Join(p.Workdir, filepath.FromSlash(relpath))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

func TestApply_HappyPathMixedTypes(t *testing.T) {
	p := setupWorkspace(t)
	writeLayerFile(t, p, "base", ".env", "A=1\nB=base\n")
	writeLayerFile(t, p, "base", "config/app.json", `{"a": 1}`+"\n")
	writeLayerFile(t, p, "base", "config/app.yml", "a: 1\n")
	writeLayerFile(t, p, "base", "notes.txt", "base notes\n")
	ensureLayerDir(t, p, "dev")
	writeLayerFile(t, p, "dev", ".env", "B=dev\nC=2\n")
	writeLayerFile(t, p, "dev", "config/app.json", `{"b": 2}`+"\n")
	writeLayerFile(t, p, "dev", "notes.txt", "dev notes\n")

	st, err := state.Load(p.StateFile)
	require.NoError(t, err)
	plan, err := materialize.BuildPlan(p, []string{"dev"})
	require.NoError(t, err)
	res, err := materialize.Apply(p, st, plan)
	require.NoError(t, err)

	// .env merged & normalized
	assert.Equal(t, "A=1\nB=dev\nC=2\n", readFile(t, wsPath(p, ".env")))
	// JSON deep-merged, keys sorted
	assert.Equal(t, "{\n  \"a\": 1,\n  \"b\": 2\n}\n", readFile(t, wsPath(p, "config/app.json")))
	// YAML merged
	assert.Contains(t, readFile(t, wsPath(p, "config/app.yml")), "a: 1")
	// bytes: last wins
	assert.Equal(t, "dev notes\n", readFile(t, wsPath(p, "notes.txt")))

	// state.yaml updated
	loaded, err := state.Load(p.StateFile)
	require.NoError(t, err)
	assert.Equal(t, []string{"dev"}, loaded.Layers)
	assert.Equal(t, []string{".env", "config/app.json", "config/app.yml", "notes.txt"}, loaded.Materialized)

	// result: all four created
	assert.ElementsMatch(t, []string{".env", "config/app.json", "config/app.yml", "notes.txt"}, res.Created)
	assert.Empty(t, res.Updated)
}

func TestApply_HigherOnlyFileMaterialized(t *testing.T) {
	p := setupWorkspace(t)
	writeLayerFile(t, p, "base", ".env", "A=1\n")
	ensureLayerDir(t, p, "local")
	writeLayerFile(t, p, "local", "local-only.txt", "only in local\n")

	st, err := state.Load(p.StateFile)
	require.NoError(t, err)
	plan, err := materialize.BuildPlan(p, []string{"local"})
	require.NoError(t, err)
	_, err = materialize.Apply(p, st, plan)
	require.NoError(t, err)

	assert.Equal(t, "only in local\n", readFile(t, wsPath(p, "local-only.txt")))
	assert.Equal(t, "A=1\n", readFile(t, wsPath(p, ".env")))
}

func TestApply_DeepMergeAcrossThreeLayers(t *testing.T) {
	p := setupWorkspace(t)
	writeLayerFile(t, p, "base", "config/app.json", `{"obj": {"a": 1, "b": 2}}`+"\n")
	ensureLayerDir(t, p, "l1")
	ensureLayerDir(t, p, "l2")
	writeLayerFile(t, p, "l1", "config/app.json", `{"obj": {"b": 20, "c": 30}}`+"\n")
	writeLayerFile(t, p, "l2", "config/app.json", `{"obj": {"c": 300}}`+"\n")

	st, err := state.Load(p.StateFile)
	require.NoError(t, err)
	plan, err := materialize.BuildPlan(p, []string{"l1", "l2"})
	require.NoError(t, err)
	_, err = materialize.Apply(p, st, plan)
	require.NoError(t, err)

	assert.Equal(t,
		"{\n  \"obj\": {\n    \"a\": 1,\n    \"b\": 20,\n    \"c\": 300\n  }\n}\n",
		readFile(t, wsPath(p, "config/app.json")))
}

func TestApply_NestedDirectoryStructure(t *testing.T) {
	p := setupWorkspace(t)
	writeLayerFile(t, p, "base", "deep/nested/path/config.yml", "k: v\n")
	ensureLayerDir(t, p, "dev")
	writeLayerFile(t, p, "dev", "deep/nested/path/config.yml", "k2: v2\n")

	st, err := state.Load(p.StateFile)
	require.NoError(t, err)
	plan, err := materialize.BuildPlan(p, []string{"dev"})
	require.NoError(t, err)
	_, err = materialize.Apply(p, st, plan)
	require.NoError(t, err)

	out := readFile(t, wsPath(p, "deep/nested/path/config.yml"))
	assert.Contains(t, out, "k: v")
	assert.Contains(t, out, "k2: v2")
}

func TestApply_UnknownLayerErrors(t *testing.T) {
	p := setupWorkspace(t)
	writeLayerFile(t, p, "base", ".env", "A=1\n")
	_, err := materialize.BuildPlan(p, []string{"nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestApply_BaseCannotBeListedExplicitly(t *testing.T) {
	p := setupWorkspace(t)
	_, err := materialize.BuildPlan(p, []string{"base"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "implicit")
}

func TestApply_DuplicateLayerErrors(t *testing.T) {
	p := setupWorkspace(t)
	ensureLayerDir(t, p, "dev")
	_, err := materialize.BuildPlan(p, []string{"dev", "dev"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "more than once")
}

func TestApply_InvalidLayerNameErrors(t *testing.T) {
	p := setupWorkspace(t)
	_, err := materialize.BuildPlan(p, []string{"bad name"})
	require.Error(t, err)
}

func TestApply_ReplaceStackRemovesOrphans(t *testing.T) {
	p := setupWorkspace(t)
	writeLayerFile(t, p, "base", ".env", "A=1\n")
	ensureLayerDir(t, p, "dev")
	ensureLayerDir(t, p, "staging")
	writeLayerFile(t, p, "dev", "secrets/dev.key", "devsecret\n")

	// apply [dev]
	st, err := state.Load(p.StateFile)
	require.NoError(t, err)
	plan, err := materialize.BuildPlan(p, []string{"dev"})
	require.NoError(t, err)
	_, err = materialize.Apply(p, st, plan)
	require.NoError(t, err)
	require.FileExists(t, wsPath(p, "secrets/dev.key"))

	// apply [staging] -> dev.key orphaned and removed
	st2, err := state.Load(p.StateFile)
	require.NoError(t, err)
	plan2, err := materialize.BuildPlan(p, []string{"staging"})
	require.NoError(t, err)
	res2, err := materialize.Apply(p, st2, plan2)
	require.NoError(t, err)

	assert.NoFileExists(t, wsPath(p, "secrets/dev.key"))
	assert.Contains(t, res2.Removed, "secrets/dev.key")
	// empty parent dirs pruned
	assert.NoDirExists(t, wsPath(p, "secrets"))

	// manifest no longer references the orphan
	loaded, err := state.Load(p.StateFile)
	require.NoError(t, err)
	assert.NotContains(t, loaded.Materialized, "secrets/dev.key")
	assert.Equal(t, []string{".env"}, loaded.Materialized)
}

func TestApply_Idempotent(t *testing.T) {
	p := setupWorkspace(t)
	writeLayerFile(t, p, "base", ".env", "A=1\n")
	ensureLayerDir(t, p, "dev")
	writeLayerFile(t, p, "dev", ".env", "B=2\n")

	apply := func() *materialize.Result {
		st, err := state.Load(p.StateFile)
		require.NoError(t, err)
		plan, err := materialize.BuildPlan(p, []string{"dev"})
		require.NoError(t, err)
		res, err := materialize.Apply(p, st, plan)
		require.NoError(t, err)
		return res
	}

	first := apply()
	assert.ElementsMatch(t, []string{".env"}, first.Created)

	second := apply()
	assert.Empty(t, second.Created)
	assert.Empty(t, second.Updated)
	assert.ElementsMatch(t, []string{".env"}, second.Unchanged)
}

func TestApply_OverwritesPreexistingUserFile(t *testing.T) {
	// A file already at the workspace root (not tracked by env-man) is
	// overwritten silently, per the locked decision.
	p := setupWorkspace(t)
	writeLayerFile(t, p, "base", ".env", "A=1\n")
	require.NoError(t, os.WriteFile(wsPath(p, ".env"), []byte("USER ORIGINAL\n"), 0o644))

	st, err := state.Load(p.StateFile)
	require.NoError(t, err)
	plan, err := materialize.BuildPlan(p, nil)
	require.NoError(t, err)
	res, err := materialize.Apply(p, st, plan)
	require.NoError(t, err)

	assert.Equal(t, "A=1\n", readFile(t, wsPath(p, ".env")))
	assert.Contains(t, res.Updated, ".env")
}

func TestApply_EmptyStackAppliesOnlyBase(t *testing.T) {
	p := setupWorkspace(t)
	writeLayerFile(t, p, "base", ".env", "A=1\n")
	writeLayerFile(t, p, "base", "base.txt", "base\n")

	st, err := state.Load(p.StateFile)
	require.NoError(t, err)
	plan, err := materialize.BuildPlan(p, nil)
	require.NoError(t, err)
	res, err := materialize.Apply(p, st, plan)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{".env", "base.txt"}, res.Created)
	loaded, err := state.Load(p.StateFile)
	require.NoError(t, err)
	assert.Empty(t, loaded.Layers)
	assert.Equal(t, []string{".env", "base.txt"}, loaded.Materialized)
}
