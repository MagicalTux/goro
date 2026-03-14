package pcre

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func mixed preg_filter ( mixed $pattern , mixed $replacement , mixed $subject [, int $limit = -1 [, int &$count ]] )
func pregFilter(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var pattern, replacement, subject *phpv.ZVal
	var limit *phpv.ZInt
	var count *phpv.ZInt

	_, err := core.Expand(ctx, args, &pattern, &replacement, &subject, &limit, &count)
	if err != nil {
		return nil, err
	}

	if limit == nil {
		limit = new(phpv.ZInt)
		*limit = -1
	}
	if count == nil {
		count = new(phpv.ZInt)
	}

	// If subject is an array, apply preg_replace to each element and return only changed ones
	if subject.GetType() == phpv.ZtArray {
		subjectArr := subject.Value().(*phpv.ZArray)
		result := phpv.NewZArray()
		totalCount := phpv.ZInt(0)

		for k, v := range subjectArr.Iterate(ctx) {
			elemCount := phpv.ZInt(0)
			replaced, err := doPregReplace(ctx, pattern, replacement, v, *limit, &elemCount)
			if err != nil {
				return nil, err
			}
			totalCount += elemCount
			// Only include entries where a replacement was actually made
			if elemCount > 0 {
				result.OffsetSet(ctx, k, replaced)
			}
		}
		*count = totalCount
		return result.ZVal(), nil
	}

	// For string subjects, do the replace and return null if no replacement was made
	replaced, err := doPregReplace(ctx, pattern, replacement, subject, *limit, count)
	if err != nil {
		return nil, err
	}

	if *count == 0 {
		return phpv.ZNULL.ZVal(), nil
	}

	return replaced, nil
}
