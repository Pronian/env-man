package merge

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// MergeJSON deep-merges JSON objects across layers (lowest priority first).
// Nested objects are merged recursively; arrays, scalars, and null are
// replaced wholesale by the higher layer. The output is indented with two
// spaces and ends with a single LF newline. Returns nil if every layer is
// empty.
func MergeJSON(contents [][]byte) ([]byte, error) {
	filtered := nonEmpty(contents)
	if len(filtered) == 0 {
		return nil, nil
	}
	var merged any
	for _, c := range filtered {
		var v any
		dec := json.NewDecoder(bytes.NewReader(c))
		dec.UseNumber() // preserve number precision on round-trip
		if err := dec.Decode(&v); err != nil {
			return nil, fmt.Errorf("parse JSON: %w", err)
		}
		merged = deepMergeAny(merged, v)
	}
	out, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal JSON: %w", err)
	}
	return append(out, '\n'), nil
}

// deepMergeAny merges b onto a. When both are JSON objects (map[string]any),
// they are merged recursively with higher-priority keys winning; otherwise b
// replaces a wholesale (covering arrays, scalars, and null).
func deepMergeAny(a, b any) any {
	am, aok := a.(map[string]any)
	bm, bok := b.(map[string]any)
	if !aok || !bok {
		return b
	}
	out := make(map[string]any, len(am))
	for k, v := range am {
		out[k] = v
	}
	for k, v := range bm {
		if existing, present := out[k]; present {
			out[k] = deepMergeAny(existing, v)
		} else {
			out[k] = v
		}
	}
	return out
}
