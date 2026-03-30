package pcre

import (
	"unicode/utf8"

	"github.com/KarpelesLab/gopcre2"
)

// findAllPCRE returns all non-overlapping matches using PCRE2 semantics.
// It deduplicates the results from gopcre2's FindAllStringSubmatchIndex,
// which can emit duplicate zero-length matches at the same position before
// advancing its internal pointer. We filter out any match whose start
// position is behind our current scan cursor.
func findAllPCRE(re *gopcre2.Regexp, s string) [][]int {
	raw := re.FindAllStringSubmatchIndex(s, -1)
	return deduplicateMatches(raw, s)
}

// findAllSubmatchIndexBytes finds all submatch indices in a byte slice,
// with deduplication and an optional limit on the number of matches.
func findAllSubmatchIndexBytes(re *gopcre2.Regexp, b []byte, n int) [][]int {
	s := string(b)
	raw := re.FindAllStringSubmatchIndex(s, -1)
	result := deduplicateMatches(raw, s)
	if n >= 0 && len(result) > n {
		result = result[:n]
	}
	return result
}

// deduplicateMatches removes duplicate zero-length matches that gopcre2 emits
// due to an iteration bug in FindAllStringSubmatchIndex. The bug causes the
// same empty-match position to appear twice before the engine advances.
//
// We track a scan cursor `pos`. Any match whose start is < pos is a
// duplicate and gets skipped. After a non-empty match ending at E, pos=E.
// After a zero-length match at P, pos is advanced by one rune from P.
func deduplicateMatches(raw [][]int, s string) [][]int {
	if raw == nil {
		return nil
	}

	var result [][]int
	pos := 0 // monotonic cursor: skip matches that start before pos

	for _, loc := range raw {
		matchStart := loc[0]
		matchEnd := loc[1]

		// Skip matches that start before our cursor (duplicates).
		if matchStart < pos {
			continue
		}

		result = append(result, loc)

		if matchEnd > matchStart {
			pos = matchEnd
		} else {
			// Zero-length match: advance cursor by one rune past matchStart.
			if matchStart < len(s) {
				_, size := utf8.DecodeRuneInString(s[matchStart:])
				if size <= 0 {
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
