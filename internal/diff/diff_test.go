package diff_test

import (
	"os"
	"path/filepath"
	"testing"

	"env-man/internal/config"
	"env-man/internal/diff"
	"env-man/internal/materialize"
	"env-man/internal/state"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setup(t *testing.T) config.Paths {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, config.DirName, config.BaseName), 0o755))
	p := config.New(dir)
	require.NoError(t, state.New(p.StateFile).Save())
	return p
}

func writeFile(t *testing.T, dir, relpath, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(relpath))
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

func writeLayerFile(t *testing.T, p config.Paths, layer, relpath, content string) {
	t.Helper()
	writeFile(t, p.LayerDir(layer), relpath, content)
}

func ensureLayer(t *testing.T, p config.Paths, name string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(p.LayerDir(name), 0o755))
}

func applyLayers(t *testing.T, p config.Paths, layers []string) {
	t.Helper()
	st, err := state.Load(p.StateFile)
	require.NoError(t, err)
	plan, err := materialize.BuildPlan(p, layers)
	require.NoError(t, err)
	_, err = materialize.Apply(p, st, plan)
	require.NoError(t, err)
}

func statusMap(report *diff.Report) map[string]diff.Status {
	out := map[string]diff.Status{}
	for _, f := range report.Files {
		out[f.RelPath] = f.Status
	}
	return out
}

func TestCompute_AddedFiles(t *testing.T) {
	p := setup(t)
	writeLayerFile(t, p, "base", ".env", "A=1\n")
	writeLayerFile(t, p, "base", "config/app.json", `{"a":1}`+"\n")

	report, err := diff.Compute(p, nil)
	require.NoError(t, err)
	sm := statusMap(report)
	assert.Equal(t, diff.StatusAdded, sm[".env"])
	assert.Equal(t, diff.StatusAdded, sm["config/app.json"])
	assert.Equal(t, 2, report.Changed())
}

func TestCompute_UnchangedAfterApply(t *testing.T) {
	p := setup(t)
	writeLayerFile(t, p, "base", ".env", "A=1\n")
	applyLayers(t, p, nil)

	report, err := diff.Compute(p, nil)
	require.NoError(t, err)
	assert.Equal(t, diff.StatusUnchanged, statusMap(report)[".env"])
	assert.Equal(t, 0, report.Changed())
}

func TestCompute_ModifiedEnvKeyDeltas(t *testing.T) {
	p := setup(t)
	writeLayerFile(t, p, "base", ".env", "A=1\nB=2\nC=3\n")
	applyLayers(t, p, nil)

	// change the source layer and re-diff (state still references old apply)
	writeLayerFile(t, p, "base", ".env", "A=1\nB=changed\nC=3\nD=new\n")

	report, err := diff.Compute(p, nil)
	require.NoError(t, err)
	fd := findFile(t, report, ".env")
	assert.Equal(t, diff.StatusModified, fd.Status)

	deltaMap := map[string]diff.Status{}
	for _, d := range fd.KeyDeltas {
		deltaMap[d.Path] = d.Status
	}
	assert.Equal(t, diff.StatusModified, deltaMap["B"])
	assert.Equal(t, diff.StatusAdded, deltaMap["D"])
	_, hasA := deltaMap["A"]
	assert.False(t, hasA, "unchanged key should not appear in deltas")
	_, hasC := deltaMap["C"]
	assert.False(t, hasC, "unchanged key should not appear in deltas")
}

func TestCompute_ModifiedJsonKeyDeltas(t *testing.T) {
	p := setup(t)
	writeLayerFile(t, p, "base", "config/app.json", `{"a":1,"obj":{"x":1,"y":2}}`+"\n")
	applyLayers(t, p, nil)

	writeLayerFile(t, p, "base", "config/app.json", `{"a":1,"obj":{"y":20,"z":30},"new":true}`+"\n")

	report, err := diff.Compute(p, nil)
	require.NoError(t, err)
	fd := findFile(t, report, "config/app.json")
	assert.Equal(t, diff.StatusModified, fd.Status)

	deltaMap := map[string]diff.Status{}
	for _, d := range fd.KeyDeltas {
		deltaMap[d.Path] = d.Status
	}
	assert.Equal(t, diff.StatusModified, deltaMap["obj.y"])
	assert.Equal(t, diff.StatusRemoved, deltaMap["obj.x"])
	assert.Equal(t, diff.StatusAdded, deltaMap["obj.z"])
	assert.Equal(t, diff.StatusAdded, deltaMap["new"])
}

func TestCompute_RemovedOrphan(t *testing.T) {
	p := setup(t)
	writeLayerFile(t, p, "base", ".env", "A=1\n")
	ensureLayer(t, p, "dev")
	writeLayerFile(t, p, "dev", "only-dev.txt", "x\n")
	applyLayers(t, p, []string{"dev"})

	// Now diff applying [] (base only): only-dev.txt becomes an orphan removal.
	report, err := diff.Compute(p, nil)
	require.NoError(t, err)
	// Wait: Compute(nil) uses the CURRENT state layers ([dev]) not [].
	// To preview removing dev, pass explicit [].
	_ = report

	report2, err := diff.Compute(p, []string{})
	require.NoError(t, err)
	assert.Equal(t, diff.StatusRemoved, statusMap(report2)["only-dev.txt"])
	assert.Equal(t, diff.StatusUnchanged, statusMap(report2)[".env"])
}

func TestCompute_NoChangeReturnsCleanReport(t *testing.T) {
	p := setup(t)
	writeLayerFile(t, p, "base", ".env", "A=1\n")
	applyLayers(t, p, nil)

	report, err := diff.Compute(p, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, report.Changed())
}

func TestCompute_YamlKeyDeltas(t *testing.T) {
	p := setup(t)
	writeLayerFile(t, p, "base", "config/app.yaml", "a: 1\nobj:\n  x: 1\n  y: 2\n")
	applyLayers(t, p, nil)
	writeLayerFile(t, p, "base", "config/app.yaml", "a: 9\nobj:\n  y: 20\n  z: 3\n")

	report, err := diff.Compute(p, nil)
	require.NoError(t, err)
	fd := findFile(t, report, "config/app.yaml")
	assert.Equal(t, diff.StatusModified, fd.Status)

	deltaMap := map[string]diff.Status{}
	for _, d := range fd.KeyDeltas {
		deltaMap[d.Path] = d.Status
	}
	assert.Equal(t, diff.StatusModified, deltaMap["a"])
	assert.Equal(t, diff.StatusRemoved, deltaMap["obj.x"])
	assert.Equal(t, diff.StatusModified, deltaMap["obj.y"])
	assert.Equal(t, diff.StatusAdded, deltaMap["obj.z"])
}

func findFile(t *testing.T, r *diff.Report, rel string) diff.FileDiff {
	t.Helper()
	for _, f := range r.Files {
		if f.RelPath == rel {
			return f
		}
	}
	require.Fail(t, "file not in diff report: %s", rel)
	return diff.FileDiff{}
}
