package merge

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeBytes_LastNonEmptyWins(t *testing.T) {
	out := MergeBytes([][]byte{[]byte("a"), []byte(""), []byte("b")})
	assert.Equal(t, "b", string(out))
}

func TestMergeBytes_AllEmptyReturnsNil(t *testing.T) {
	assert.Nil(t, MergeBytes([][]byte{nil, []byte{}}))
	assert.Nil(t, MergeBytes(nil))
}

func TestMergeBytes_ResultIsACopy(t *testing.T) {
	src := []byte("hello")
	out := MergeBytes([][]byte{src})
	out[0] = 'X'
	assert.Equal(t, byte('h'), src[0], "MergeBytes must not alias its input")
}

func TestMergeBytes_SingleEntry(t *testing.T) {
	out := MergeBytes([][]byte{[]byte("only")})
	assert.Equal(t, "only", string(out))
}
