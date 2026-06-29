package merge

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStrategyFor(t *testing.T) {
	cases := []struct {
		path string
		want Strategy
	}{
		{".env", StrategyEnv},
		{".env.local", StrategyEnv},
		{".env.production", StrategyEnv},
		{"config/.env", StrategyEnv},
		// "app.env" does NOT match the .env/.env.* rule (no leading .env prefix)
		{"config/app.env", StrategyBytes},
		{"app.json", StrategyJSON},
		{"config/db.json", StrategyJSON},
		{"app.yaml", StrategyYAML},
		{"app.yml", StrategyYAML},
		{"config/app.YAML", StrategyYAML}, // case-insensitive ext
		{"README.md", StrategyBytes},
		{"logo.png", StrategyBytes},
		{"config/credentials.txt", StrategyBytes},
		{"Makefile", StrategyBytes},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			assert.Equal(t, tc.want, StrategyFor(tc.path))
		})
	}
}

func TestMerge_DispatchesByStrategy(t *testing.T) {
	// .env -> normalized merge
	got, err := Merge(".env", [][]byte{
		[]byte("A=1\nB=base\n"),
		[]byte("B=layer\nC=2\n"),
	})
	assert.NoError(t, err)
	assert.Equal(t, "A=1\nB=layer\nC=2\n", string(got))

	// .json -> deep merge
	got, err = Merge("app.json", [][]byte{
		[]byte(`{"a": 1, "obj": {"x": 1}}`),
		[]byte(`{"obj": {"y": 2}, "b": 2}`),
	})
	assert.NoError(t, err)
	assert.Equal(t, "{\n  \"a\": 1,\n  \"b\": 2,\n  \"obj\": {\n    \"x\": 1,\n    \"y\": 2\n  }\n}\n", string(got))

	// unknown -> last-wins
	got, err = Merge("notes.txt", [][]byte{[]byte("base"), []byte("layer")})
	assert.NoError(t, err)
	assert.Equal(t, "layer", string(got))
}

func TestMerge_AllEmptyReturnsNil(t *testing.T) {
	out, err := Merge("app.json", [][]byte{nil, []byte{}})
	assert.NoError(t, err)
	assert.Nil(t, out)
}
