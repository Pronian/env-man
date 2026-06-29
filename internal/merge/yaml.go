package merge

import (
	"bytes"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// MergeYAML deep-merges YAML documents across layers (lowest priority first).
// Mappings are merged recursively, preserving the key order of the base layer
// and appending keys introduced by higher layers. Sequences, scalars, and
// non-mapping nodes are replaced wholesale by the higher layer. Returns nil if
// every layer is empty.
//
// Only single-document YAML files are supported; a multi-document input yields
// an error.
func MergeYAML(contents [][]byte) ([]byte, error) {
	filtered := nonEmpty(contents)
	if len(filtered) == 0 {
		return nil, nil
	}
	var merged *yaml.Node
	for _, c := range filtered {
		// Decode via a Decoder so we can detect (and reject) multi-document
		// YAML. yaml.Unmarshal into a single node silently reads only the
		// first document, which would drop data.
		dec := yaml.NewDecoder(bytes.NewReader(c))
		var doc yaml.Node
		if err := dec.Decode(&doc); err != nil {
			return nil, fmt.Errorf("parse YAML: %w", err)
		}
		var extra yaml.Node
		switch err := dec.Decode(&extra); {
		case err == io.EOF:
			// single document, expected
		case err != nil:
			return nil, fmt.Errorf("parse YAML: %w", err)
		default:
			return nil, fmt.Errorf("multi-document YAML is not supported")
		}
		merged = mergeNode(merged, unwrapDoc(&doc))
	}
	if merged == nil {
		return nil, nil
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(merged); err != nil {
		return nil, fmt.Errorf("marshal YAML: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("flush YAML: %w", err)
	}
	return buf.Bytes(), nil
}

// unwrapDoc returns the node inside a DocumentNode, or the node itself.
func unwrapDoc(n *yaml.Node) *yaml.Node {
	if n != nil && n.Kind == yaml.DocumentNode {
		if len(n.Content) == 0 {
			return nil
		}
		return n.Content[0]
	}
	return n
}

// mergeNode merges b onto a. Both-nil yields nil. Two mappings are merged
// recursively; any other combination yields b (cloned).
func mergeNode(a, b *yaml.Node) *yaml.Node {
	switch {
	case a == nil && b == nil:
		return nil
	case a == nil:
		return cloneNode(b)
	case b == nil:
		return cloneNode(a)
	case a.Kind == yaml.MappingNode && b.Kind == yaml.MappingNode:
		return mergeMapping(a, b)
	default:
		return cloneNode(b)
	}
}

// mergeMapping merges b's keys onto a's, preserving a's key order and appending
// b's new keys. Shared keys are merged recursively.
func mergeMapping(a, b *yaml.Node) *yaml.Node {
	out := &yaml.Node{
		Kind:        yaml.MappingNode,
		Tag:         a.Tag,
		Style:       a.Style,
		HeadComment: a.HeadComment,
		FootComment: a.FootComment,
	}
	index := map[string]int{}
	pairs := [][2]*yaml.Node{}
	for i := 0; i+1 < len(a.Content); i += 2 {
		k, v := a.Content[i], a.Content[i+1]
		pairs = append(pairs, [2]*yaml.Node{cloneNode(k), cloneNode(v)})
		index[k.Value] = len(pairs) - 1
	}
	for i := 0; i+1 < len(b.Content); i += 2 {
		k, v := b.Content[i], b.Content[i+1]
		if idx, ok := index[k.Value]; ok {
			pairs[idx][1] = mergeNode(pairs[idx][1], v)
		} else {
			pairs = append(pairs, [2]*yaml.Node{cloneNode(k), cloneNode(v)})
			index[k.Value] = len(pairs) - 1
		}
	}
	for _, p := range pairs {
		out.Content = append(out.Content, p[0], p[1])
	}
	return out
}

// cloneNode returns a deep copy of n, recursively cloning Content.
func cloneNode(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	c := *n
	c.Content = nil
	for _, child := range n.Content {
		c.Content = append(c.Content, cloneNode(child))
	}
	return &c
}
