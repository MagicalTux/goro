package pcre

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func mixed preg_replace_callback_array ( array $patterns_and_callbacks , mixed $subject [, int $limit = -1 [, int &$count [, int $flags = 0 ]]] )
func pregReplaceCallbackArray(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var patternsAndCallbacks *phpv.ZArray
	var subject *phpv.ZVal
	var limitArg *phpv.ZInt
	var countRef core.OptionalRef[phpv.ZInt]
	var flagsArg *phpv.ZInt

	_, err := core.Expand(ctx, args, &patternsAndCallbacks, &subject, &limitArg, &countRef, &flagsArg)
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
			res, err := doCallbackArray(ctx, patternsAndCallbacks, v, limit, &c, flags)
			if err != nil {
				return nil, err
			}
			totalCount += c
			resultArr.OffsetSet(ctx, k, res)
		}
		*count = totalCount
		result = resultArr.ZVal()
	} else {
		result, err = doCallbackArray(ctx, patternsAndCallbacks, subject, limit, count, flags)
		if err != nil {
			return nil, err
		}
	}

	if countRef.HasArg() {
		countRef.Set(ctx, *count)
	}

	return result, nil
}

func doCallbackArray(ctx phpv.Context, patternsAndCallbacks *phpv.ZArray, subject *phpv.ZVal, limit phpv.ZInt, count *phpv.ZInt, flags phpv.ZInt) (*phpv.ZVal, error) {
	totalCount := phpv.ZInt(0)
	current := subject

	// Iterate over patterns_and_callbacks: key=pattern, value=callback
	for k, v := range patternsAndCallbacks.Iterate(ctx) {
		patternStr := k.AsString(ctx)

		callback, err := core.SpawnCallable(ctx, v)
		if err != nil {
			return nil, err
		}

		patternVal := phpv.ZString(patternStr).ZVal()
		c := phpv.ZInt(0)
		result, err := doReplaceCallback(ctx, patternVal, callback, current, limit, &c, flags)
		if err != nil {
			return nil, err
		}
		totalCount += c
		current = result
	}

	*count = totalCount
	return current, nil
}
