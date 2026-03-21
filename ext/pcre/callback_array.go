package pcre

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
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

	// Validate all keys are strings and all values are valid callbacks BEFORE executing
	var pairs []patCallback
	for k, v := range patternsAndCallbacks.Iterate(ctx) {
		// Check key is a string (not numeric)
		if k.GetType() == phpv.ZtInt {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				"preg_replace_callback_array(): Argument #1 ($pattern) must contain only string patterns as keys")
		}
		patternStr := string(k.AsString(ctx))

		callback, cbErr := core.SpawnCallable(ctx, v)
		if cbErr != nil {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				"preg_replace_callback_array(): Argument #1 ($pattern) must contain only valid callbacks")
		}
		pairs = append(pairs, patCallback{pattern: patternStr, callback: callback})
	}

	var result *phpv.ZVal

	// Handle array subject
	if subject.GetType() == phpv.ZtArray {
		subjectArr := subject.Value().(*phpv.ZArray)
		resultArr := phpv.NewZArray()
		totalCount := phpv.ZInt(0)
		for k, v := range subjectArr.Iterate(ctx) {
			c := phpv.ZInt(0)
			res, err := doCallbackArrayPairs(ctx, pairs, v, limit, &c, flags)
			if err != nil {
				return nil, err
			}
			totalCount += c
			resultArr.OffsetSet(ctx, k, res)
		}
		*count = totalCount
		result = resultArr.ZVal()
	} else {
		result, err = doCallbackArrayPairs(ctx, pairs, subject, limit, count, flags)
		if err != nil {
			return nil, err
		}
	}

	if countRef.HasArg() {
		countRef.Set(ctx, *count)
	}

	return result, nil
}

type patCallback struct {
	pattern  string
	callback phpv.Callable
}

func doCallbackArrayPairs(ctx phpv.Context, pairs []patCallback, subject *phpv.ZVal, limit phpv.ZInt, count *phpv.ZInt, flags phpv.ZInt) (*phpv.ZVal, error) {
	totalCount := phpv.ZInt(0)
	current := subject

	for _, pair := range pairs {
		patternVal := phpv.ZString(pair.pattern).ZVal()
		c := phpv.ZInt(0)
		result, err := doReplaceCallback(ctx, patternVal, pair.callback, current, limit, &c, flags)
		if err != nil {
			return nil, err
		}
		totalCount += c
		current = result
	}

	*count = totalCount
	return current, nil
}
