package pcre

import (
	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpv"
)

// > func array preg_grep ( string $pattern , array $input [, int $flags = 0 ] )
func pregGrep(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var patternVal *phpv.ZVal
	var input *phpv.ZArray
	var flagsArg *phpv.ZInt

	_, err := core.Expand(ctx, args, &patternVal, &input, &flagsArg)
	if err != nil {
		return phpv.ZFalse.ZVal(), err
	}

	if err := checkStringArg(ctx, patternVal, "preg_grep", 1, "$pattern"); err != nil {
		return nil, err
	}

	pattern := patternVal.AsString(ctx)
	flags := core.Deref(flagsArg, 0)

	re, pcreErr := prepareRegexp(string(pattern))
	if pcreErr != nil {
		ctx.Warn("%s", pcreErr.Warning("preg_grep"))
		setLastPCREError(ctx, pcreInternalError)
		return phpv.ZBool(false).ZVal(), nil
	}

	invert := flags&phpv.ZInt(PREG_GREP_INVERT) != 0

	result := phpv.NewZArray()
	for k, v := range input.Iterate(ctx) {
		str := v.AsString(ctx)
		matched := re.MatchString(string(str))

		if (matched && !invert) || (!matched && invert) {
			result.OffsetSet(ctx, k, v)
		}
	}

	setLastPCREError(ctx, pcreNoError)
	return result.ZVal(), nil
}
