package pcre

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

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

	re, err := prepareRegexp(string(pattern))
	if err != nil {
		return nil, err
	}

	subjectStr := string(subject)

	// Handle offset parameter
	if offset > 0 {
		if int(offset) >= len(subjectStr) {
			return phpv.ZInt(0).ZVal(), nil
		}
		subjectStr = subjectStr[int(offset):]
	}

	if flags&phpv.ZInt(PREG_OFFSET_CAPTURE) != 0 {
		// PREG_OFFSET_CAPTURE: each match element is [match, offset]
		loc := re.FindStringSubmatchIndex(subjectStr)
		if loc == nil {
			return phpv.ZInt(0).ZVal(), nil
		}

		if matchesArg.Value != nil {
			matches := *matchesArg.Get()
			for i := 0; i < len(loc); i += 2 {
				pair := phpv.NewZArray()
				if loc[i] < 0 {
					pair.OffsetSet(ctx, nil, phpv.ZString("").ZVal())
					pair.OffsetSet(ctx, nil, phpv.ZInt(-1).ZVal())
				} else {
					pair.OffsetSet(ctx, nil, phpv.ZString(subjectStr[loc[i]:loc[i+1]]).ZVal())
					pair.OffsetSet(ctx, nil, phpv.ZInt(loc[i]+int(offset)).ZVal())
				}
				matches.OffsetSet(ctx, nil, pair.ZVal())
			}
		}

		return phpv.ZInt(1).ZVal(), nil
	}

	m := re.FindStringSubmatch(subjectStr)
	if m == nil {
		return phpv.ZInt(0).ZVal(), nil
	}

	if matchesArg.Value != nil {
		matches := *matchesArg.Get()
		for _, elem := range m {
			matches.OffsetSet(ctx, nil, phpv.ZStr(elem))
		}
	}

	_ = flags

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

	re, err := prepareRegexp(string(pattern))
	if err != nil {
		return nil, err
	}

	subjectStr := string(subject)

	if offset > 0 {
		if int(offset) >= len(subjectStr) {
			return phpv.ZInt(0).ZVal(), nil
		}
		subjectStr = subjectStr[int(offset):]
	}

	allMatches := re.FindAllStringSubmatch(subjectStr, -1)
	if allMatches == nil {
		if matchesArg.Value != nil {
			// Initialize empty matches array
			matches := *matchesArg.Get()
			subArr := phpv.NewZArray()
			matches.OffsetSet(ctx, nil, subArr.ZVal())
		}
		return phpv.ZInt(0).ZVal(), nil
	}

	if matchesArg.Value != nil {
		matches := *matchesArg.Get()

		if flags&phpv.ZInt(PREG_SET_ORDER) != 0 {
			// PREG_SET_ORDER: each element is an array of all groups for one match
			for _, m := range allMatches {
				subArr := phpv.NewZArray()
				for _, elem := range m {
					subArr.OffsetSet(ctx, nil, phpv.ZStr(elem))
				}
				matches.OffsetSet(ctx, nil, subArr.ZVal())
			}
		} else {
			// PREG_PATTERN_ORDER (default): each element is an array of all matches for one group
			numGroups := len(allMatches[0])
			groups := make([]*phpv.ZArray, numGroups)
			for i := range groups {
				groups[i] = phpv.NewZArray()
			}
			for _, m := range allMatches {
				for i, elem := range m {
					groups[i].OffsetSet(ctx, nil, phpv.ZStr(elem))
				}
			}
			for _, g := range groups {
				matches.OffsetSet(ctx, nil, g.ZVal())
			}
		}
	}

	return phpv.ZInt(len(allMatches)).ZVal(), nil
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

	re, err := prepareRegexp(string(pattern))
	if err != nil {
		return nil, err
	}

	n := -1
	if limit >= 0 {
		n = int(limit)
	}

	parts := re.Split(string(subject), n)

	result := phpv.NewZArray()
	for _, part := range parts {
		if flags&phpv.ZInt(PREG_SPLIT_NO_EMPTY) != 0 && part == "" {
			continue
		}
		result.OffsetSet(ctx, nil, phpv.ZString(part).ZVal())
	}

	return result.ZVal(), nil
}

// > func string preg_replace_callback ( mixed $pattern , callable $callback , mixed $subject [, int $limit = -1 [, int &$count ]] )
func pregReplaceCallback(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var pattern *phpv.ZVal
	var callback phpv.Callable
	var subject *phpv.ZVal
	var limitArg *phpv.ZInt
	var count *phpv.ZInt

	_, err := core.Expand(ctx, args, &pattern, &callback, &subject, &limitArg, &count)
	if err != nil {
		return nil, err
	}

	limit := core.Deref(limitArg, -1)
	if count == nil {
		count = new(phpv.ZInt)
	}

	re, err := prepareRegexp(string(pattern.AsString(ctx)))
	if err != nil {
		return nil, err
	}

	in := []byte(subject.AsString(ctx))
	var r []byte
	n := 0
	maxReplacements := int(limit)

	for {
		if maxReplacements >= 0 && n >= maxReplacements {
			break
		}

		loc := re.FindSubmatchIndex(in)
		if loc == nil {
			break
		}

		// Extract submatches
		matchArr := phpv.NewZArray()
		for i := 0; i < len(loc); i += 2 {
			if loc[i] < 0 {
				matchArr.OffsetSet(ctx, nil, phpv.ZString("").ZVal())
			} else {
				matchArr.OffsetSet(ctx, nil, phpv.ZString(in[loc[i]:loc[i+1]]).ZVal())
			}
		}

		// Call the callback with the matches array
		result, err := callback.Call(ctx, []*phpv.ZVal{matchArr.ZVal()})
		if err != nil {
			return nil, err
		}

		r = append(r, in[:loc[0]]...)
		r = append(r, []byte(result.AsString(ctx))...)
		in = in[loc[1]:]
		n++

		// Prevent infinite loop on zero-length matches
		if loc[0] == loc[1] {
			if len(in) == 0 {
				break
			}
			r = append(r, in[0])
			in = in[1:]
		}
	}
	r = append(r, in...)

	*count = phpv.ZInt(n)

	return phpv.ZString(r).ZVal(), nil
}
