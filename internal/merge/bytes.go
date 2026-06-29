package merge

// MergeBytes returns the highest-priority non-empty content, copied. If every
// layer is empty, it returns nil.
func MergeBytes(contents [][]byte) []byte {
	filtered := nonEmpty(contents)
	if len(filtered) == 0 {
		return nil
	}
	last := filtered[len(filtered)-1]
	return append([]byte(nil), last...)
}
