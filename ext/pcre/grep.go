package pcre

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func array preg_grep ( string $pattern , array $input [, int $flags = 0 ] )
func pregGrep(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var pattern phpv.ZString
	var input *phpv.ZArray
	var flagsArg *phpv.ZInt

	_, err := core.Expand(ctx, args, &pattern, &input, &flagsArg)
	if err != nil {
		return phpv.ZFalse.ZVal(), err
	}

	flags := core.Deref(flagsArg, 0)

	re, pcreErr := prepareRegexp(string(pattern))
	if pcreErr != nil {
		ctx.Warn("%s", pcreErr.Warning("preg_grep"))
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

	return result.ZVal(), nil
}
