package serialize

// EqualIgnoringStyle reports whether two lines match once whitespace is stripped away.
// For indentation-sensitive languages (Python), leading whitespace must match exactly.
// For delimiter-based languages (Go, Rust), leading indentation shifts are ignored.
func EqualIgnoringStyle(s, d string, significantWhitespace bool) bool {
	i, j := 0, 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	for j < len(d) && (d[j] == ' ' || d[j] == '\t') {
		j++
	}

	if significantWhitespace && s[:i] != d[:j] {
		return false
	}

	for i < len(s) || j < len(d) {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\r' || s[i] == '\n') {
			i++
		}
		for j < len(d) && (d[j] == ' ' || d[j] == '\t' || d[j] == '\r' || d[j] == '\n') {
			j++
		}
		if i >= len(s) && j >= len(d) {
			return true
		}
		if i >= len(s) || j >= len(d) {
			return false
		}
		startI, startJ := i, j
		for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '\r' && s[i] != '\n' {
			i++
		}
		for j < len(d) && d[j] != ' ' && d[j] != '\t' && d[j] != '\r' && d[j] != '\n' {
			j++
		}
		if s[startI:i] != d[startJ:j] {
			return false
		}
	}
	return true
}
