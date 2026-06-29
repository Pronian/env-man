package merge

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeJSON_DeepMergeObjects(t *testing.T) {
	out, err := MergeJSON([][]byte{
		[]byte(`{"a": 1, "obj": {"x": 1, "nested": {"k": "v"}}}`),
		[]byte(`{"obj": {"y": 2, "nested": {"k": "override", "new": 1}}, "b": 2}`),
	})
	require.NoError(t, err)

	want := `{
  "a": 1,
  "b": 2,
  "obj": {
    "nested": {
      "k": "override",
      "new": 1
    },
    "x": 1,
    "y": 2
  }
}
`
	assert.Equal(t, want, string(out))
}

func TestMergeJSON_ArraysReplacedNotConcatenated(t *testing.T) {
	out, err := MergeJSON([][]byte{
		[]byte(`{"arr": [1, 2, 3], "keep": true}`),
		[]byte(`{"arr": [9]}`),
	})
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	assert.Equal(t, []any{float64(9)}, got["arr"])
	assert.Equal(t, true, got["keep"])
}

func TestMergeJSON_ScalarReplaced(t *testing.T) {
	out, err := MergeJSON([][]byte{
		[]byte(`{"n": 1, "s": "old"}`),
		[]byte(`{"n": 2, "s": "new"}`),
	})
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	assert.EqualValues(t, 2, got["n"])
	assert.Equal(t, "new", got["s"])
}

func TestMergeJSON_NullOverrides(t *testing.T) {
	out, err := MergeJSON([][]byte{
		[]byte(`{"a": 1, "b": 2}`),
		[]byte(`{"a": null}`),
	})
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	assert.Nil(t, got["a"])
	assert.EqualValues(t, 2, got["b"])
}

func TestMergeJSON_TopLevelArrayReplaced(t *testing.T) {
	out, err := MergeJSON([][]byte{
		[]byte(`[1, 2]`),
		[]byte(`[3]`),
	})
	require.NoError(t, err)
	assert.Equal(t, "[\n  3\n]\n", string(out))
}

func TestMergeJSON_NumberPrecisionPreserved(t *testing.T) {
	// UseNumber must keep large integers lossless (no float conversion).
	out, err := MergeJSON([][]byte{
		[]byte(`{"big": 12345678901234567890}`),
	})
	require.NoError(t, err)
	assert.Contains(t, string(out), `"big": 12345678901234567890`)
}

func TestMergeJSON_MalformedErrors(t *testing.T) {
	_, err := MergeJSON([][]byte{[]byte(`{"a": }`)})
	require.Error(t, err)
}

func TestMergeJSON_TrailingNewline(t *testing.T) {
	out, err := MergeJSON([][]byte{[]byte(`{"a": 1}`)})
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(string(out), "\n"))
}

func TestMergeJSON_AllEmptyReturnsNil(t *testing.T) {
	out, err := MergeJSON([][]byte{nil, []byte("")})
	require.NoError(t, err)
	assert.Nil(t, out)
}
