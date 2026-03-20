package pcre

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func mixed preg_filter ( mixed $pattern , mixed $replacement , mixed $subject [, int $limit = -1 [, int &$count ]] )
func pregFilter(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var pattern, replacement, subject *phpv.ZVal
	var limit *phpv.ZInt
	var countRef core.OptionalRef[phpv.ZInt]

	_, err := core.Expand(ctx, args, &pattern, &replacement, &subject, &limit, &countRef)
	if err != nil {
		return nil, err
	}

	limitVal := phpv.ZInt(-1)
	if limit != nil {
		limitVal = *limit
	}
	count := new(phpv.ZInt)

	var result *phpv.ZVal

	// If subject is an array, apply preg_replace to each element and return only changed ones
	if subject.GetType() == phpv.ZtArray {
		subjectArr := subject.Value().(*phpv.ZArray)
		resultArr := phpv.NewZArray()
		totalCount := phpv.ZInt(0)

		for k, v := range subjectArr.Iterate(ctx) {
			elemCount := phpv.ZInt(0)
			replaced, err := doPregReplace(ctx, pattern, replacement, v, limitVal, &elemCount)
			if err != nil {
				return nil, err
			}
			totalCount += elemCount
			// Only include entries where a replacement was actually made
			if elemCount > 0 {
				resultArr.OffsetSet(ctx, k, replaced)
			}
		}
		*count = totalCount
		result = resultArr.ZVal()
	} else {
		// For string subjects, do the replace and return null if no replacement was made
		replaced, err := doPregReplace(ctx, pattern, replacement, subject, limitVal, count)
		if err != nil {
			return nil, err
		}

		if *count == 0 {
			result = phpv.ZNULL.ZVal()
		} else {
			result = replaced
		}
	}

	if countRef.HasArg() {
		countRef.Set(ctx, *count)
	}

	return result, nil
}
