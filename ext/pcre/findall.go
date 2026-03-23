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
// IMPORTANT: This function uses FindAllStringSubmatchIndex on the full string
// to preserve context for assertions like \b, ^, $ etc.
func findAllPCRE(re *regexp.Regexp, s string) [][]int {
	// Get all matches that Go's regexp finds on the full string.
	// This preserves context for assertions like \b.
	goMatches := re.FindAllStringSubmatchIndex(s, -1)
	if goMatches == nil {
		return nil
	}

	// Build result by iterating Go matches and inserting any missing
	// zero-length matches that PCRE would find after non-zero-length matches.
	var result [][]int

	for i, loc := range goMatches {
		matchStart := loc[0]
		matchEnd := loc[1]

		result = append(result, loc)

		// After a non-zero-length match, PCRE checks for a zero-length match
		// at matchEnd. Go skips this position. We need to insert it if:
		// 1. This was a non-zero-length match
		// 2. The next Go match doesn't already start at matchEnd with a zero-length match
		if matchStart != matchEnd && matchEnd <= len(s) {
			// Check if Go's next match is already a zero-length match at matchEnd
			alreadyFound := false
			if i+1 < len(goMatches) {
				nextStart := goMatches[i+1][0]
				nextEnd := goMatches[i+1][1]
				if nextStart == matchEnd && nextEnd == matchEnd {
					alreadyFound = true
				}
			}
			if !alreadyFound {
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
// at exactly position pos in string s.
// Note: we use s[pos:] which may lose left-context for some assertions.
// For assertions like \b, the main matches are found by FindAllStringSubmatchIndex
// on the full string, so this function only needs to find "bonus" zero-length
// matches after non-zero-length ones (e.g., /\d*/ matching "" after "2").
func tryZeroLengthMatch(re *regexp.Regexp, s string, pos int) []int {
	if pos > len(s) {
		return nil
	}
	loc := re.FindStringSubmatchIndex(s[pos:])
	if loc == nil {
		return nil
	}
	// Must be a zero-length match at the very start of the substring
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
