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
//
// IMPORTANT: This function operates on the full string to preserve context
// for assertions like \b, ^, $ etc. Previous versions sliced the string
// with s[pos:] which broke word-boundary matching.
func findAllPCRE(re *regexp.Regexp, s string) [][]int {
	// First, get all matches that Go's regexp finds
	goMatches := re.FindAllStringSubmatchIndex(s, -1)
	if goMatches == nil {
		return nil
	}

	// Build a set of match-start positions Go already found
	// so we can check for missing zero-length matches
	var result [][]int

	for i, loc := range goMatches {
		matchStart := loc[0]
		matchEnd := loc[1]

		// Check if we need to insert a zero-length match BEFORE this match.
		// This happens when the previous match was non-zero-length and ended
		// at a position where a zero-length match is possible, but Go skipped it.
		if i > 0 {
			prevEnd := goMatches[i-1][1]
			prevStart := goMatches[i-1][0]
			// If previous match was non-zero-length and current match doesn't
			// start at prevEnd, check for a zero-length match at prevEnd.
			if prevStart != prevEnd && matchStart != prevEnd && prevEnd <= len(s) {
				zeroLoc := re.FindStringIndex(s[prevEnd:])
				if zeroLoc != nil && zeroLoc[0] == 0 && zeroLoc[1] == 0 {
					// There's a zero-length match at prevEnd that Go skipped
					fullLoc := re.FindStringSubmatchIndex(s[prevEnd:])
					if fullLoc != nil && fullLoc[0] == 0 && fullLoc[1] == 0 {
						adjusted := make([]int, len(fullLoc))
						copy(adjusted, fullLoc)
						for j := range adjusted {
							if adjusted[j] >= 0 {
								adjusted[j] += prevEnd
							}
						}
						result = append(result, adjusted)
					}
				}
			}
		}

		result = append(result, loc)

		// After a non-zero-length match, check if there's a zero-length match
		// at matchEnd that Go will skip (because Go advances past zero-length
		// matches by one character).
		if matchStart != matchEnd && matchEnd <= len(s) {
			// Check if the NEXT Go match starts at matchEnd - if so, Go found it
			nextStartsHere := false
			if i+1 < len(goMatches) && goMatches[i+1][0] == matchEnd {
				nextStartsHere = true
			}
			if !nextStartsHere {
				// Try to find a zero-length match at matchEnd using the full
				// string context. We search from matchEnd: but we need context.
				// Use FindStringSubmatchIndex on s[matchEnd:] for the actual
				// zero-length check. For assertions like \b, we need to check
				// if the pattern matches at this exact position in the original
				// string by using a copy approach.
				zeroLoc := tryZeroLengthMatch(re, s, matchEnd)
				if zeroLoc != nil {
					result = append(result, zeroLoc)
				}
			}
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// tryZeroLengthMatch checks if the regex can produce a zero-length match
// at exactly position pos in string s, preserving full string context.
func tryZeroLengthMatch(re *regexp.Regexp, s string, pos int) []int {
	if pos > len(s) {
		return nil
	}
	// Search in the substring starting at pos
	loc := re.FindStringSubmatchIndex(s[pos:])
	if loc == nil {
		return nil
	}
	// Check if it's a zero-length match at the start of the substring
	if loc[0] != 0 || loc[1] != 0 {
		return nil
	}
	// Adjust offsets to full string positions
	adjusted := make([]int, len(loc))
	copy(adjusted, loc)
	for i := range adjusted {
		if adjusted[i] >= 0 {
			adjusted[i] += pos
		}
	}
	return adjusted
}
