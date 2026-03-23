package pcre

import "regexp"

// findAllPCRE emulates PCRE's behavior for finding all matches.
// Unlike Go's FindAllStringSubmatchIndex which skips the character after a
// zero-length match, PCRE tries to match at every position. This means
// zero-length matches can occur at consecutive positions.
//
// For example, with pattern /\d*/ on "ab2c3u":
// Go finds: [0:0], [1:1], [2:3], [4:5], [6:6]  (5 matches)
// PCRE finds: [0:0], [1:1], [2:3], [3:3], [4:5], [5:5], [6:6]  (7 matches)
//
// The difference is that after a non-zero-length match, PCRE also checks
// for a zero-length match at the end position.
func findAllPCRE(re *regexp.Regexp, s string) [][]int {
	var result [][]int
	pos := 0

	for pos <= len(s) {
		// Find the next match starting at pos
		loc := re.FindStringSubmatchIndex(s[pos:])
		if loc == nil {
			break
		}

		// Adjust offsets to be relative to the full string
		for i := range loc {
			if loc[i] >= 0 {
				loc[i] += pos
			}
		}

		matchStart := loc[0]
		matchEnd := loc[1]

		// If the match doesn't start at our current position, there might be
		// a gap. Check if there are zero-length matches at intermediate positions.
		// Actually, for split purposes, we only care about actual matches.
		// The key issue is: after a non-zero-length match, we need to check
		// for a zero-length match at matchEnd.

		result = append(result, loc)

		if matchStart == matchEnd {
			// Zero-length match: advance by one character
			pos = matchStart + 1
		} else {
			// Non-zero-length match: check for zero-length match at matchEnd
			pos = matchEnd

			// Try to find a zero-length match at the end of the non-zero match
			if pos <= len(s) {
				zeroLoc := re.FindStringSubmatchIndex(s[pos:])
				if zeroLoc != nil {
					// Adjust offsets
					for i := range zeroLoc {
						if zeroLoc[i] >= 0 {
							zeroLoc[i] += pos
						}
					}
					// Only add if it's a zero-length match at exactly pos
					if zeroLoc[0] == pos && zeroLoc[1] == pos {
						result = append(result, zeroLoc)
						// Advance past this position since we already handled it
						pos = pos + 1
					}
				}
			}
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}
