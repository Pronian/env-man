package merge

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// asYAML parses bytes back into a generic map for structural comparison.
func asYAML(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, yaml.Unmarshal(b, &m), "parse: %q", string(b))
	return m
}

// mappingKeys re-parses bytes into a yaml.Node and returns the top-level
// mapping's key sequence, to assert key-order preservation.
func mappingKeys(t *testing.T, b []byte) []string {
	t.Helper()
	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal(b, &doc))
	require.Equal(t, yaml.DocumentNode, doc.Kind)
	require.NotEmpty(t, doc.Content)
	root := doc.Content[0]
	require.Equal(t, yaml.MappingNode, root.Kind)
	var keys []string
	for i := 0; i+1 < len(root.Content); i += 2 {
		keys = append(keys, root.Content[i].Value)
	}
	return keys
}

func TestMergeYAML_DeepMergeStructural(t *testing.T) {
	out, err := MergeYAML([][]byte{
		[]byte("a: 1\nobj:\n  x: 1\n  nested:\n    k: v\nb: 2\n"),
		[]byte("obj:\n  y: 2\n  nested:\n    k: override\nc: 3\n"),
	})
	require.NoError(t, err)

	got := asYAML(t, out)
	assert.EqualValues(t, 1, got["a"])
	assert.EqualValues(t, 2, got["b"])
	assert.EqualValues(t, 3, got["c"])
	obj, ok := got["obj"].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, 1, obj["x"])
	assert.EqualValues(t, 2, obj["y"])
	nested := obj["nested"].(map[string]any)
	assert.Equal(t, "override", nested["k"])
}

func TestMergeYAML_PreservesBaseKeyOrderAppendsNew(t *testing.T) {
	out, err := MergeYAML([][]byte{
		[]byte("zebra: 1\nalpha: 2\nmango: 3\n"),     // base order: zebra, alpha, mango
		[]byte("zebra: override\nbeta: 4\n"),          // override zebra, add beta
	})
	require.NoError(t, err)

	// Expected order: base order preserved (zebra, alpha, mango), beta appended.
	assert.Equal(t, []string{"zebra", "alpha", "mango", "beta"}, mappingKeys(t, out))
}

func TestMergeYAML_NestedOrderPreserved(t *testing.T) {
	out, err := MergeYAML([][]byte{
		[]byte("obj:\n  z: 1\n  a: 2\n"),
		[]byte("obj:\n  m: 3\n"),
	})
	require.NoError(t, err)

	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal(out, &doc))
	obj := doc.Content[0].Content[1] // key "obj" value
	var keys []string
	for i := 0; i+1 < len(obj.Content); i += 2 {
		keys = append(keys, obj.Content[i].Value)
	}
	// base order (z, a) preserved, m appended.
	assert.Equal(t, []string{"z", "a", "m"}, keys)
}

func TestMergeYAML_SequencesReplaced(t *testing.T) {
	out, err := MergeYAML([][]byte{
		[]byte("arr:\n  - 1\n  - 2\nkeep: true\n"),
		[]byte("arr:\n  - 9\n"),
	})
	require.NoError(t, err)
	got := asYAML(t, out)
	assert.Equal(t, []any{9}, got["arr"])
	assert.Equal(t, true, got["keep"])
}

func TestMergeYAML_ScalarReplaced(t *testing.T) {
	out, err := MergeYAML([][]byte{
		[]byte("n: 1\n"),
		[]byte("n: 2\n"),
	})
	require.NoError(t, err)
	got := asYAML(t, out)
	assert.EqualValues(t, 2, got["n"])
}

func TestMergeYAML_TopLevelScalarReplaced(t *testing.T) {
	out, err := MergeYAML([][]byte{
		[]byte("hello\n"),
		[]byte("world\n"),
	})
	require.NoError(t, err)
	assert.Equal(t, "world\n", string(out))
}

func TestMergeYAML_MalformedErrors(t *testing.T) {
	_, err := MergeYAML([][]byte{[]byte(":\n  : bad\n :")})
	require.Error(t, err)
}

func TestMergeYAML_MultiDocumentErrors(t *testing.T) {
	_, err := MergeYAML([][]byte{[]byte("a: 1\n---\nb: 2\n")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multi-document")
}

func TestMergeYAML_AllEmptyReturnsNil(t *testing.T) {
	out, err := MergeYAML([][]byte{nil, []byte("")})
	require.NoError(t, err)
	assert.Nil(t, out)
}

func TestMergeYAML_OutputHasTrailingNewline(t *testing.T) {
	out, err := MergeYAML([][]byte{[]byte("a: 1\n")})
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(string(out), "\n"))
}
