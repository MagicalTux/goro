package pcre

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func mixed preg_replace_callback_array ( array $patterns_and_callbacks , mixed $subject [, int $limit = -1 [, int &$count ]] )
func pregReplaceCallbackArray(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var patternsAndCallbacks *phpv.ZArray
	var subject *phpv.ZVal
	var limitArg *phpv.ZInt
	var count *phpv.ZInt

	_, err := core.Expand(ctx, args, &patternsAndCallbacks, &subject, &limitArg, &count)
	if err != nil {
		return nil, err
	}

	limit := core.Deref(limitArg, -1)
	if count == nil {
		count = new(phpv.ZInt)
	}

	totalCount := phpv.ZInt(0)

	// Current result starts as the subject
	current := subject

	// Iterate over patterns_and_callbacks: key=pattern, value=callback
	for k, v := range patternsAndCallbacks.Iterate(ctx) {
		patternStr := k.AsString(ctx)

		callback, err := core.SpawnCallable(ctx, v)
		if err != nil {
			return nil, err
		}

		re, err := prepareRegexp(string(patternStr))
		if err != nil {
			return nil, err
		}

		// Apply this pattern+callback to current result
		in := []byte(current.AsString(ctx))
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
		totalCount += phpv.ZInt(n)

		current = phpv.ZString(r).ZVal()
	}

	*count = totalCount

	return current, nil
}
