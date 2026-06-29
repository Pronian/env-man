package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRootRegistersSubcommands ensures the full command surface is wired.
func TestRootRegistersSubcommands(t *testing.T) {
	root := NewRootCmd()
	want := []string{"init", "apply", "drop", "list", "diff"}
	got := map[string]bool{}
	for _, c := range root.Commands() {
		got[c.Name()] = true
	}
	for _, w := range want {
		assert.True(t, got[w], "expected subcommand %q to be registered", w)
	}
}

// TestRootHelpRenders verifies help output contains identifying text.
func TestRootHelpRenders(t *testing.T) {
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--help"})
	require.NoError(t, root.Execute())
	assert.Contains(t, out.String(), "env-man")
	assert.Contains(t, out.String(), ".env-man/")
}

// TestRootVersionFlag verifies the version flag is wired.
func TestRootVersionFlag(t *testing.T) {
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--version"})
	require.NoError(t, root.Execute())
	assert.Contains(t, out.String(), Version)
}

