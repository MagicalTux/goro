package pcre

import (
	"fmt"
	"regexp"
	"unicode/utf8"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// validMatchFlags are the valid flags for preg_match/preg_match_all
const validMatchFlags = PREG_PATTERN_ORDER | PREG_SET_ORDER | PREG_OFFSET_CAPTURE | PREG_UNMATCHED_AS_NULL

// makeMatchVal creates a single match value, handling PREG_OFFSET_CAPTURE and PREG_UNMATCHED_AS_NULL.
func makeMatchVal(ctx phpv.Context, elem string, matched bool, loc int, offsetCapture, unmatchedAsNull bool) *phpv.ZVal {
	if !matched {
		if offsetCapture {
			pair := phpv.NewZArray()
			if unmatchedAsNull {
				pair.OffsetSet(ctx, nil, phpv.ZNULL.ZVal())
			} else {
				pair.OffsetSet(ctx, nil, phpv.ZString("").ZVal())
			}
			pair.OffsetSet(ctx, nil, phpv.ZInt(-1).ZVal())
			return pair.ZVal()
		}
		if unmatchedAsNull {
			return phpv.ZNULL.ZVal()
		}
		return phpv.ZString("").ZVal()
	}
	if offsetCapture {
		pair := phpv.NewZArray()
		pair.OffsetSet(ctx, nil, phpv.ZString(elem).ZVal())
		pair.OffsetSet(ctx, nil, phpv.ZInt(loc).ZVal())
		return pair.ZVal()
	}
	return phpv.ZString(elem).ZVal()
}

// addNamedCaptures adds both numeric and named keys to a matches array,
// mimicking PHP's behavior where named groups appear as both numeric and string keys.
// In PHP, for named groups the order is: [0]=full, ["name1"]=group1, [1]=group1, ["name2"]=group2, [2]=group2, ...
func addNamedCaptures(ctx phpv.Context, matches *phpv.ZArray, re *regexp.Regexp, m []string, flags phpv.ZInt, locs []int, baseOffset int) {
	names := re.SubexpNames()
	offsetCapture := flags&phpv.ZInt(PREG_OFFSET_CAPTURE) != 0
	unmatchedAsNull := flags&phpv.ZInt(PREG_UNMATCHED_AS_NULL) != 0

	for i, elem := range m {
		matched := locs == nil || locs[i*2] >= 0
		loc := 0
		if locs != nil && locs[i*2] >= 0 {
			loc = locs[i*2] + baseOffset
		}
		val := makeMatchVal(ctx, elem, matched, loc, offsetCapture, unmatchedAsNull)

		// PHP ordering: for named groups, the string key comes BEFORE the numeric key
		if i < len(names) && names[i] != "" {
			matches.OffsetSet(ctx, phpv.ZString(names[i]).ZVal(), val)
		}
		// Add numeric key
		matches.OffsetSet(ctx, nil, val)
	}
}

// > func int preg_match ( string $pattern , string $subject [, array &$matches [, int $flags = 0 [, int $offset = 0 ]]] )
func pregMatch(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var pattern, subject phpv.ZString
	var matchesArg core.OptionalRef[*phpv.ZArray]
	var flagsArg, offsetArg *phpv.ZInt

	_, err := core.Expand(ctx, args, &pattern, &subject, &matchesArg, &flagsArg, &offsetArg)
	if err != nil {
		return phpv.ZFalse.ZVal(), err
	}

	flags := core.Deref(flagsArg, 0)
	offset := core.Deref(offsetArg, 0)

	re, pcreErr := prepareRegexp(string(pattern))
	if pcreErr != nil {
		ctx.Warn("%s", pcreErr.Warning("preg_match"))
		return phpv.ZBool(false).ZVal(), nil
	}

	subjectStr := string(subject)

	// Handle offset parameter
	var sliceOffset int
	if offset != 0 {
		byteOffset := int64(offset)
		subLen := int64(len(subjectStr))
		if byteOffset < 0 {
			byteOffset = subLen + byteOffset
		}
		if byteOffset < 0 {
			// Check for extreme underflow (e.g., PHP_INT_MIN)
			if int64(offset) < -subLen {
				return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
					fmt.Sprintf("preg_match(): Argument #5 ($offset) must be greater than or equal to %d", -subLen))
			}
			byteOffset = 0
		}
		if byteOffset > subLen {
			return phpv.ZInt(0).ZVal(), nil
		}
		sliceOffset = int(byteOffset)
		subjectStr = subjectStr[sliceOffset:]
	}

	loc := re.FindStringSubmatchIndex(subjectStr)
	if loc == nil {
		if matchesArg.Value != nil {
			matchesArg.Set(ctx, phpv.NewZArray())
		}
		return phpv.ZInt(0).ZVal(), nil
	}

	if matchesArg.Value != nil {
		matches := phpv.NewZArray()

		// Build the match string array from loc
		numGroups := len(loc) / 2
		m := make([]string, numGroups)
		for i := 0; i < numGroups; i++ {
			if loc[i*2] >= 0 {
				m[i] = subjectStr[loc[i*2]:loc[i*2+1]]
			}
		}

		addNamedCaptures(ctx, matches, re, m, flags, loc, sliceOffset)
		matchesArg.Set(ctx, matches)
	}

	return phpv.ZInt(1).ZVal(), nil
}

// > func int preg_match_all ( string $pattern , string $subject [, array &$matches [, int $flags = 0 [, int $offset = 0 ]]] )
func pregMatchAll(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var pattern, subject phpv.ZString
	var matchesArg core.OptionalRef[*phpv.ZArray]
	var flagsArg, offsetArg *phpv.ZInt

	_, err := core.Expand(ctx, args, &pattern, &subject, &matchesArg, &flagsArg, &offsetArg)
	if err != nil {
		return phpv.ZFalse.ZVal(), err
	}

	flags := core.Deref(flagsArg, 0)
	offset := core.Deref(offsetArg, 0)

	// Validate flags
	if flags & ^validMatchFlags != 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "preg_match_all(): Argument #4 ($flags) must be a PREG_* constant")
	}

	re, pcreErr := prepareRegexp(string(pattern))
	if pcreErr != nil {
		ctx.Warn("%s", pcreErr.Warning("preg_match_all"))
		return phpv.ZBool(false).ZVal(), nil
	}

	subjectStr := string(subject)

	var sliceOffset int
	if offset != 0 {
		byteOffset := int64(offset)
		subLen := int64(len(subjectStr))
		if byteOffset < 0 {
			byteOffset = subLen + byteOffset
		}
		if byteOffset < 0 {
			// Check for extreme underflow (e.g., PHP_INT_MIN)
			if int64(offset) < -subLen {
				return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
					fmt.Sprintf("preg_match_all(): Argument #5 ($offset) must be greater than or equal to %d", -subLen))
			}
			byteOffset = 0
		}
		if byteOffset >= subLen {
			if matchesArg.Value != nil {
				newMatches := phpv.NewZArray()
				if flags&phpv.ZInt(PREG_SET_ORDER) != 0 {
					// No sub-arrays needed
				} else {
					numGroups := re.NumSubexp() + 1
					names := re.SubexpNames()
					for i := 0; i < numGroups; i++ {
						subArr := phpv.NewZArray()
						if i < len(names) && names[i] != "" {
							newMatches.OffsetSet(ctx, phpv.ZString(names[i]).ZVal(), subArr.ZVal())
						}
						newMatches.OffsetSet(ctx, nil, subArr.ZVal())
					}
				}
				matchesArg.Set(ctx, newMatches)
			}
			return phpv.ZInt(0).ZVal(), nil
		}
		sliceOffset = int(byteOffset)
		subjectStr = subjectStr[sliceOffset:]
	}

	names := re.SubexpNames()

	// Find all matches with their indices
	allLocs := re.FindAllStringSubmatchIndex(subjectStr, -1)
	if allLocs == nil {
		if matchesArg.Value != nil {
			newMatches := phpv.NewZArray()
			if flags&phpv.ZInt(PREG_SET_ORDER) != 0 {
				// No sub-arrays needed
			} else {
				// PREG_PATTERN_ORDER: create empty arrays for each group
				numGroups := re.NumSubexp() + 1
				for i := 0; i < numGroups; i++ {
					subArr := phpv.NewZArray()
					if i < len(names) && names[i] != "" {
						newMatches.OffsetSet(ctx, phpv.ZString(names[i]).ZVal(), subArr.ZVal())
					}
					newMatches.OffsetSet(ctx, nil, subArr.ZVal())
				}
			}
			matchesArg.Set(ctx, newMatches)
		}
		return phpv.ZInt(0).ZVal(), nil
	}

	numGroups := len(allLocs[0]) / 2
	offsetCapture := flags&phpv.ZInt(PREG_OFFSET_CAPTURE) != 0
	unmatchedAsNull := flags&phpv.ZInt(PREG_UNMATCHED_AS_NULL) != 0

	if matchesArg.Value != nil {
		matches := phpv.NewZArray()

		if flags&phpv.ZInt(PREG_SET_ORDER) != 0 {
			// PREG_SET_ORDER: each element is an array of all groups for one match
			for _, loc := range allLocs {
				subArr := phpv.NewZArray()
				for i := 0; i < numGroups; i++ {
					s := loc[i*2]
					e := loc[i*2+1]
					var val *phpv.ZVal
					if s < 0 {
						if offsetCapture {
							pair := phpv.NewZArray()
							if unmatchedAsNull {
								pair.OffsetSet(ctx, nil, phpv.ZNULL.ZVal())
							} else {
								pair.OffsetSet(ctx, nil, phpv.ZString("").ZVal())
							}
							pair.OffsetSet(ctx, nil, phpv.ZInt(-1).ZVal())
							val = pair.ZVal()
						} else {
							if unmatchedAsNull {
								val = phpv.ZNULL.ZVal()
							} else {
								val = phpv.ZString("").ZVal()
							}
						}
					} else {
						if offsetCapture {
							pair := phpv.NewZArray()
							pair.OffsetSet(ctx, nil, phpv.ZString(subjectStr[s:e]).ZVal())
							pair.OffsetSet(ctx, nil, phpv.ZInt(s+sliceOffset).ZVal())
							val = pair.ZVal()
						} else {
							val = phpv.ZString(subjectStr[s:e]).ZVal()
						}
					}
					if i < len(names) && names[i] != "" {
						subArr.OffsetSet(ctx, phpv.ZString(names[i]).ZVal(), val)
					}
					subArr.OffsetSet(ctx, nil, val)
				}
				matches.OffsetSet(ctx, nil, subArr.ZVal())
			}
		} else {
			// PREG_PATTERN_ORDER (default): each element is an array of all matches for one group
			groups := make([]*phpv.ZArray, numGroups)
			for i := range groups {
				groups[i] = phpv.NewZArray()
			}
			for _, loc := range allLocs {
				for i := 0; i < numGroups; i++ {
					s := loc[i*2]
					e := loc[i*2+1]
					var val *phpv.ZVal
					if s < 0 {
						if offsetCapture {
							pair := phpv.NewZArray()
							if unmatchedAsNull {
								pair.OffsetSet(ctx, nil, phpv.ZNULL.ZVal())
							} else {
								pair.OffsetSet(ctx, nil, phpv.ZString("").ZVal())
							}
							pair.OffsetSet(ctx, nil, phpv.ZInt(-1).ZVal())
							val = pair.ZVal()
						} else {
							if unmatchedAsNull {
								val = phpv.ZNULL.ZVal()
							} else {
								val = phpv.ZString("").ZVal()
							}
						}
					} else {
						if offsetCapture {
							pair := phpv.NewZArray()
							pair.OffsetSet(ctx, nil, phpv.ZString(subjectStr[s:e]).ZVal())
							pair.OffsetSet(ctx, nil, phpv.ZInt(s+sliceOffset).ZVal())
							val = pair.ZVal()
						} else {
							val = phpv.ZString(subjectStr[s:e]).ZVal()
						}
					}
					groups[i].OffsetSet(ctx, nil, val)
				}
			}
			for i, g := range groups {
				if i < len(names) && names[i] != "" {
					matches.OffsetSet(ctx, phpv.ZString(names[i]).ZVal(), g.ZVal())
				}
				matches.OffsetSet(ctx, nil, g.ZVal())
			}
		}
		matchesArg.Set(ctx, matches)
	}

	return phpv.ZInt(len(allLocs)).ZVal(), nil
}

// > func array preg_split ( string $pattern , string $subject [, int $limit = -1 [, int $flags = 0 ]] )
func pregSplit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var pattern, subject phpv.ZString
	var limitArg, flagsArg *phpv.ZInt

	_, err := core.Expand(ctx, args, &pattern, &subject, &limitArg, &flagsArg)
	if err != nil {
		return phpv.ZFalse.ZVal(), err
	}

	limit := core.Deref(limitArg, -1)
	flags := core.Deref(flagsArg, 0)

	re, pcreErr := prepareRegexp(string(pattern))
	if pcreErr != nil {
		ctx.Warn("%s", pcreErr.Warning("preg_split"))
		return phpv.ZBool(false).ZVal(), nil
	}

	subjectStr := string(subject)
	noEmpty := flags&phpv.ZInt(PREG_SPLIT_NO_EMPTY) != 0
	delimCapture := flags&phpv.ZInt(PREG_SPLIT_DELIM_CAPTURE) != 0
	offsetCapture := flags&phpv.ZInt(PREG_SPLIT_OFFSET_CAPTURE) != 0

	maxSplits := int(limit)
	if maxSplits == 0 {
		// PHP: limit=0 is same as limit=-1 (unlimited)
		maxSplits = -1
	}

	result := phpv.NewZArray()
	addPart := func(s string, offset int) {
		if noEmpty && s == "" {
			return
		}
		if offsetCapture {
			pair := phpv.NewZArray()
			pair.OffsetSet(ctx, nil, phpv.ZString(s).ZVal())
			pair.OffsetSet(ctx, nil, phpv.ZInt(offset).ZVal())
			result.OffsetSet(ctx, nil, pair.ZVal())
		} else {
			result.OffsetSet(ctx, nil, phpv.ZString(s).ZVal())
		}
	}

	addDelimCaptures := func(loc []int) {
		if delimCapture {
			numGroups := len(loc) / 2
			for i := 1; i < numGroups; i++ {
				s := loc[i*2]
				e := loc[i*2+1]
				if s >= 0 {
					addPart(subjectStr[s:e], s)
				} else {
					// Unmatched capture group
					if !noEmpty {
						addPart("", -1)
					}
				}
			}
		}
	}

	// PHP PCRE behavior: find matches at every position, including zero-length matches.
	// Go's FindAll skips positions after zero-length matches, so we use findAllPCRE
	// to emulate PCRE's behavior of finding zero-length matches at every position.
	allLocs := findAllPCRE(re, subjectStr)
	if allLocs == nil {
		// No matches; return the entire subject
		addPart(subjectStr, 0)
		return result.ZVal(), nil
	}

	nSplits := 0
	pos := 0

	for _, loc := range allLocs {
		if maxSplits > 0 && nSplits >= maxSplits-1 {
			break
		}

		matchStart := loc[0]
		matchEnd := loc[1]

		addPart(subjectStr[pos:matchStart], pos)
		nSplits++
		addDelimCaptures(loc)
		pos = matchEnd
	}

	// Add the remaining part
	if pos <= len(subjectStr) {
		addPart(subjectStr[pos:], pos)
	}

	return result.ZVal(), nil
}

// > func string preg_replace_callback ( mixed $pattern , callable $callback , mixed $subject [, int $limit = -1 [, int &$count [, int $flags = 0 ]]] )
func pregReplaceCallback(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var pattern *phpv.ZVal
	var callback phpv.Callable
	var subject *phpv.ZVal
	var limitArg *phpv.ZInt
	var countRef core.OptionalRef[phpv.ZInt]
	var flagsArg *phpv.ZInt

	_, err := core.Expand(ctx, args, &pattern, &callback, &subject, &limitArg, &countRef, &flagsArg)
	if err != nil {
		return nil, err
	}

	limit := core.Deref(limitArg, -1)
	flags := core.Deref(flagsArg, 0)
	count := new(phpv.ZInt)

	var result *phpv.ZVal

	// Handle array subject
	if subject.GetType() == phpv.ZtArray {
		subjectArr := subject.Value().(*phpv.ZArray)
		resultArr := phpv.NewZArray()
		totalCount := phpv.ZInt(0)
		for k, v := range subjectArr.Iterate(ctx) {
			c := phpv.ZInt(0)
			r, err := doReplaceCallback(ctx, pattern, callback, v, limit, &c, flags)
			if err != nil {
				return nil, err
			}
			totalCount += c
			resultArr.OffsetSet(ctx, k, r)
		}
		*count = totalCount
		result = resultArr.ZVal()
	} else {
		result, err = doReplaceCallback(ctx, pattern, callback, subject, limit, count, flags)
		if err != nil {
			return nil, err
		}
	}

	if countRef.HasArg() {
		countRef.Set(ctx, *count)
	}

	return result, nil
}

func doReplaceCallback(ctx phpv.Context, pattern *phpv.ZVal, callback phpv.Callable, subject *phpv.ZVal, limit phpv.ZInt, count *phpv.ZInt, flags phpv.ZInt) (*phpv.ZVal, error) {
	re, pcreErr := prepareRegexp(string(pattern.AsString(ctx)))
	if pcreErr != nil {
		ctx.Warn("%s", pcreErr.Warning("preg_replace_callback"))
		return phpv.ZNULL.ZVal(), nil
	}

	names := re.SubexpNames()
	offsetCapture := flags&phpv.ZInt(PREG_OFFSET_CAPTURE) != 0
	unmatchedAsNull := flags&phpv.ZInt(PREG_UNMATCHED_AS_NULL) != 0

	in := []byte(subject.AsString(ctx))

	maxReplacements := int(limit)
	if maxReplacements < 0 {
		maxReplacements = -1
	}

	// Find all matches at once on the original string to preserve anchor semantics
	allLocs := re.FindAllSubmatchIndex(in, maxReplacements)
	if allLocs == nil {
		*count = 0
		return phpv.ZString(in).ZVal(), nil
	}

	var r []byte
	pos := 0
	for _, loc := range allLocs {
		// Extract submatches and build matches array with named captures
		matchArr := phpv.NewZArray()
		numGroups := len(loc) / 2
		for i := 0; i < numGroups; i++ {
			var val *phpv.ZVal
			if loc[i*2] < 0 {
				if offsetCapture {
					pair := phpv.NewZArray()
					if unmatchedAsNull {
						pair.OffsetSet(ctx, nil, phpv.ZNULL.ZVal())
					} else {
						pair.OffsetSet(ctx, nil, phpv.ZString("").ZVal())
					}
					pair.OffsetSet(ctx, nil, phpv.ZInt(-1).ZVal())
					val = pair.ZVal()
				} else {
					if unmatchedAsNull {
						val = phpv.ZNULL.ZVal()
					} else {
						val = phpv.ZString("").ZVal()
					}
				}
			} else {
				if offsetCapture {
					pair := phpv.NewZArray()
					pair.OffsetSet(ctx, nil, phpv.ZString(in[loc[i*2]:loc[i*2+1]]).ZVal())
					pair.OffsetSet(ctx, nil, phpv.ZInt(loc[i*2]).ZVal())
					val = pair.ZVal()
				} else {
					val = phpv.ZString(in[loc[i*2]:loc[i*2+1]]).ZVal()
				}
			}
			if i < len(names) && names[i] != "" {
				matchArr.OffsetSet(ctx, phpv.ZString(names[i]).ZVal(), val)
			}
			matchArr.OffsetSet(ctx, nil, val)
		}

		// Call the callback with the matches array
		result, err := callback.Call(ctx, []*phpv.ZVal{matchArr.ZVal()})
		if err != nil {
			return nil, err
		}

		r = append(r, in[pos:loc[0]]...)
		r = append(r, []byte(result.AsString(ctx))...)
		pos = loc[1]

		// For zero-length matches, advance by one rune to avoid infinite loop
		if loc[0] == loc[1] && pos < len(in) {
			_, size := utf8.DecodeRune(in[pos:])
			if size == 0 {
				size = 1
			}
			r = append(r, in[pos:pos+size]...)
			pos += size
		}
	}
	r = append(r, in[pos:]...)

	*count = phpv.ZInt(len(allLocs))

	return phpv.ZString(r).ZVal(), nil
}
