package pcre

import (
	"unicode/utf8"

	"github.com/KarpelesLab/gopcre2"
)

// findAllPCRE returns all non-overlapping matches using PCRE2 semantics.
// It works around a bug in gopcre2's FindAllStringSubmatchIndex that causes
// duplicate zero-length matches to appear in the result for certain patterns.
// We implement our own deduplication: skip any match whose start position is
// before our current scan position.
func findAllPCRE(re *gopcre2.Regexp, s string) [][]int {
	return deduplicateMatches(re.FindAllStringSubmatchIndex(s, -1), s)
}

// findAllSubmatchIndexBytes finds all submatch indices in a byte slice,
// applying deduplication to work around gopcre2's duplicate zero-length match
// behavior, and also respecting a limit on the number of matches.
func findAllSubmatchIndexBytes(re *gopcre2.Regexp, b []byte, n int) [][]int {
	s := string(b)
	raw := re.FindAllStringSubmatchIndex(s, -1)
	result := deduplicateMatches(raw, s)
	if n >= 0 && len(result) > n {
		result = result[:n]
	}
	return result
}

// deduplicateMatches removes consecutive duplicate zero-length matches that
// gopcre2 emits due to its iteration bug. It tracks a scan position and skips
// any match whose start is behind the current position.
func deduplicateMatches(raw [][]int, s string) [][]int {
	if raw == nil {
		return nil
	}

	var result [][]int
	pos := 0
	for _, loc := range raw {
		matchStart := loc[0]
		matchEnd := loc[1]

		// Skip matches that start before our current position.
		// These are duplicates from gopcre2's buggy zero-length match handling.
		if matchStart < pos {
			continue
		}

		result = append(result, loc)

		if matchEnd > matchStart {
			pos = matchEnd
		} else {
			// Zero-length match: advance by one rune for the next iteration.
			if pos < len(s) {
				_, size := utf8.DecodeRuneInString(s[pos:])
				if size == 0 {
					size = 1
				}
				pos = matchStart + size
			} else {
				pos = matchStart + 1
			}
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}
