package merge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeEnv_HigherLayerOverridesKey(t *testing.T) {
	out, err := MergeEnv([][]byte{
		[]byte("A=base\nB=keep\n"),
		[]byte("A=layer\nC=new\n"),
	})
	require.NoError(t, err)
	assert.Equal(t, "A=layer\nB=keep\nC=new\n", string(out))
}

func TestMergeEnv_NormalizesOutput(t *testing.T) {
	// Comments and blank lines dropped; keys sorted; export prefix stripped.
	out, err := MergeEnv([][]byte{
		[]byte("# header comment\n\nexport ZOO = zebra\nALPHA=1\n"),
	})
	require.NoError(t, err)
	assert.Equal(t, "ALPHA=1\nZOO=zebra\n", string(out))
}

func TestMergeEnv_DoesNotExpandVariables(t *testing.T) {
	// $VAR placeholders preserved literally; '$' forces quoting on output.
	out, err := MergeEnv([][]byte{
		[]byte(`URL=https://$HOST:5432` + "\n"),
	})
	require.NoError(t, err)
	assert.Equal(t, `URL="https://$HOST:5432"`+"\n", string(out))
}

func TestMergeEnv_SingleQuoteIsLiteralAndRequoted(t *testing.T) {
	out, err := MergeEnv([][]byte{[]byte(`SINGLE='a b c'` + "\n")})
	require.NoError(t, err)
	assert.Equal(t, "SINGLE=\"a b c\"\n", string(out))
}

func TestMergeEnv_DoubleQuoteProcessesEscapes(t *testing.T) {
	// File content: MULTI="a\nb"   (backslash-n, two chars in file)
	// Parsed value: a<newline>b   (real newline)
	// Output:        MULTI="a\nb"  (newline re-escaped)
	out, err := MergeEnv([][]byte{[]byte(`MULTI="a\nb"` + "\n")})
	require.NoError(t, err)
	assert.Equal(t, "MULTI=\"a\\nb\"\n", string(out))
}

func TestMergeEnv_BareValueStripsTrailingComment(t *testing.T) {
	out, err := MergeEnv([][]byte{[]byte("BARE=value # trailing\n")})
	require.NoError(t, err)
	assert.Equal(t, "BARE=value\n", string(out))
}

func TestMergeEnv_EmptyValue(t *testing.T) {
	out, err := MergeEnv([][]byte{[]byte("EMPTY=\nKEEP=1\n")})
	require.NoError(t, err)
	assert.Equal(t, "EMPTY=\nKEEP=1\n", string(out))
}

func TestMergeEnv_YAMLStyleColonSeparator(t *testing.T) {
	out, err := MergeEnv([][]byte{[]byte("TIMEOUT: 3000\n")})
	require.NoError(t, err)
	assert.Equal(t, "TIMEOUT=3000\n", string(out))
}

func TestMergeEnv_SpaceForcesQuotingButBareHashDoesNot(t *testing.T) {
	out, err := MergeEnv([][]byte{[]byte("SPACE=hello world\nHASH=a#b\n")})
	require.NoError(t, err)
	// "hello world" has a space -> double-quoted; "a#b" has no space before # -> bare.
	assert.Equal(t, "HASH=a#b\nSPACE=\"hello world\"\n", string(out))
}

func TestMergeEnv_ValueWithEqualsSplitsOnFirstSeparator(t *testing.T) {
	out, err := MergeEnv([][]byte{[]byte("CONN=a=b=c\n")})
	require.NoError(t, err)
	// '=' in a value forces quoting (safe but explicit).
	assert.Equal(t, "CONN=\"a=b=c\"\n", string(out))
}

func TestMergeEnv_AllEmptyReturnsNil(t *testing.T) {
	out, err := MergeEnv([][]byte{nil, []byte("")})
	require.NoError(t, err)
	assert.Nil(t, out)
}

func TestMergeEnv_MalformedErrors(t *testing.T) {
	cases := [][]byte{
		[]byte("no separator here\n"),
		[]byte("=missingkey\n"),
		[]byte(`"unterminated` + "\n"),
	}
	for i, c := range cases {
		_, err := MergeEnv([][]byte{c})
		require.Error(t, err, "case %d should error: %q", i, string(c))
	}
}

func TestParseEnv_RejectsInvalidKey(t *testing.T) {
	// Internal whitespace in a key is rejected.
	_, err := parseEnv([]byte("BAD KEY=1\n"))
	require.Error(t, err)
}

func TestParseEnv_CRLFNormalized(t *testing.T) {
	out, err := MergeEnv([][]byte{[]byte("A=1\r\nB=2\r\n")})
	require.NoError(t, err)
	assert.Equal(t, "A=1\nB=2\n", string(out))
}
