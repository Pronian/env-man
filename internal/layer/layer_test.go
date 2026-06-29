package layer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	cases := []struct {
		name string
		in   string
		ok   bool
	}{
		{"simple", "dev", true},
		{"alphanumeric", "layer1", true},
		{"with dash", "my-layer", true},
		{"with underscore", "my_layer", true},
		{"uppercase", "DEV", true},
		{"base", "base", true},
		{"empty", "", false},
		{"with space", "my layer", false},
		{"with dot", "a.b", false},
		{"with slash", "a/b", false},
		{"parent traversal", "../etc", false},
		{"with backslash", "a\\b", false},
		{"unicode", "über", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.in)
			if tc.ok {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalidName)
			}
		})
	}
}

func TestIsReserved(t *testing.T) {
	assert.True(t, IsReserved("base"))
	assert.False(t, IsReserved("dev"))
	assert.False(t, IsReserved(""))
	assert.False(t, IsReserved("BASE"))
}

func TestDiscover_IncludesBaseAndLayers(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "base"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "dev"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "local"), 0o755))
	// A plain file (e.g. state.yaml) must be skipped.
	require.NoError(t, os.WriteFile(filepath.Join(root, "state.yaml"), []byte{}, 0o644))
	// A hidden directory must be skipped.
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".cache"), 0o755))

	got, err := Discover(root)
	require.NoError(t, err)
	assert.Equal(t, []string{"base", "dev", "local"}, got)
}

func TestDiscover_IsSorted(t *testing.T) {
	root := t.TempDir()
	for _, n := range []string{"zeta", "alpha", "mike"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, n), 0o755))
	}
	got, err := Discover(root)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "mike", "zeta"}, got)
}

func TestDiscover_MissingDirErrors(t *testing.T) {
	_, err := Discover(filepath.Join(t.TempDir(), "nope"))
	require.Error(t, err)
}

func TestDiscover_EmptyDirReturnsNil(t *testing.T) {
	root := t.TempDir()
	got, err := Discover(root)
	require.NoError(t, err)
	assert.Nil(t, got)
}
