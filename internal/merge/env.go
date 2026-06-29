package merge

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// MergeEnv parses each env file in priority order and merges its KEY=value
// pairs; higher layers override lower ones. Values are kept literally (no
// $VAR expansion). Output is normalized: keys sorted lexicographically,
// values quoted only when necessary, one entry per line, LF newlines.
// Returns nil if every layer is empty.
func MergeEnv(contents [][]byte) ([]byte, error) {
	filtered := nonEmpty(contents)
	if len(filtered) == 0 {
		return nil, nil
	}
	merged := map[string]string{}
	for _, c := range filtered {
		pairs, err := parseEnv(c)
		if err != nil {
			return nil, err
		}
		for k, v := range pairs {
			merged[k] = v
		}
	}
	return formatEnv(merged), nil
}

// envKeyRe constrains legal env keys (letters, digits, underscore, dot).
var envKeyRe = regexp.MustCompile(`^[A-Za-z0-9_.]+$`)

// ParseEnv parses raw env-file content into a key->value map. It is the
// building block MergeEnv uses; exported so other packages (e.g. diff) can
// parse a single env file without merging.
func ParseEnv(src []byte) (map[string]string, error) {
	return parseEnv(src)
}

// parseEnv parses raw env-file content into a key->value map. Blank lines and
// `#` comment lines are ignored. Supports an optional `export ` prefix and
// single- or double-quoted values. No variable expansion is performed. A
// trailing ` # comment` on an unquoted value is stripped.
func parseEnv(src []byte) (map[string]string, error) {
	src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	out := map[string]string{}
	for _, raw := range bytes.Split(src, []byte("\n")) {
		line := bytes.TrimLeft(raw, " \t")
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		if bytes.HasPrefix(line, []byte("export ")) {
			line = bytes.TrimLeft(line[len("export "):], " \t")
			if len(line) == 0 || line[0] == '#' {
				continue
			}
		}
		sepIdx := bytes.IndexAny(line, "=:")
		if sepIdx < 0 {
			return nil, fmt.Errorf("invalid env line %q: missing '=' or ':'", string(line))
		}
		key := strings.TrimSpace(string(line[:sepIdx]))
		if key == "" {
			return nil, fmt.Errorf("invalid env line: empty key")
		}
		if !envKeyRe.MatchString(key) {
			return nil, fmt.Errorf("invalid env key %q", key)
		}
		val := strings.TrimSpace(string(line[sepIdx+1:]))
		unquoted, err := unquoteEnv(val)
		if err != nil {
			return nil, fmt.Errorf("env value for %q: %w", key, err)
		}
		out[key] = unquoted
	}
	return out, nil
}

// unquoteEnv interprets surrounding quotes. Single quotes are literal; double
// quotes process \n \r \t \" \\ escapes. Unquoted values have a trailing
// whitespace-prefixed comment stripped.
func unquoteEnv(v string) (string, error) {
	if len(v) == 0 {
		return "", nil
	}
	switch v[0] {
	case '"':
		if len(v) < 2 || v[len(v)-1] != '"' {
			return "", fmt.Errorf("unterminated double-quoted value")
		}
		return unescapeDoubleQuoted(v[1 : len(v)-1]), nil
	case '\'':
		if len(v) < 2 || v[len(v)-1] != '\'' {
			return "", fmt.Errorf("unterminated single-quoted value")
		}
		return v[1 : len(v)-1], nil
	default:
		return stripTrailingComment(v), nil
	}
}

// stripTrailingComment removes a trailing `# comment` that is preceded by
// whitespace, matching common .env conventions.
func stripTrailingComment(v string) string {
	for i := 0; i+1 < len(v); i++ {
		if (v[i] == ' ' || v[i] == '\t') && v[i+1] == '#' {
			return strings.TrimRight(v[:i], " \t")
		}
	}
	return v
}

func unescapeDoubleQuoted(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			default:
				b.WriteByte(s[i])
				b.WriteByte(s[i+1])
			}
			i++
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// formatEnv serializes a key->value map as normalized KEY=value lines, sorted
// by key, ending with a single LF newline.
func formatEnv(m map[string]string) []byte {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	for _, k := range keys {
		buf.WriteString(formatEnvLine(k, m[k]))
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

func formatEnvLine(key, value string) string {
	if value == "" {
		return key + "="
	}
	if envNeedsQuoting(value) {
		return key + "=" + doubleQuoteEscape(value)
	}
	return key + "=" + value
}

// envNeedsQuoting reports whether a value requires double-quoting on output.
// Quoting is applied when a value contains characters that would change its
// meaning when the file is re-parsed or sourced: whitespace, quotes, the
// assignment operator, the expansion operator, or backslashes. A bare '#'
// (without preceding whitespace) round-trips safely and is left unquoted.
func envNeedsQuoting(v string) bool {
	for i := 0; i < len(v); i++ {
		switch v[i] {
		case ' ', '\t', '\n', '\r', '"', '\'', '=', '$', '\\':
			return true
		}
	}
	return false
}

func doubleQuoteEscape(v string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(v); i++ {
		switch v[i] {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteByte(v[i])
		}
	}
	b.WriteByte('"')
	return b.String()
}
