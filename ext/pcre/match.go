package pcre

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func int preg_match ( string $pattern , string $subject [, array &$matches [, int $flags = 0 [, int $offset = 0 ]]] )
func pregMatch(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var pattern, subject phpv.ZString
	var matchesArg **phpv.ZArray
	var flagsArg, offsetArg *phpv.ZInt

	_, err := core.Expand(ctx, args, &pattern, &subject, core.Ref(&matchesArg), &flagsArg, &offsetArg)
	if err != nil {
		return phpv.ZFalse.ZVal(), err
	}

	flags := core.Deref(flagsArg, 0)
	offset := core.Deref(offsetArg, 0)

	if flagsArg != nil {
		_ = flags
		panic("TODO: flags")
	}
	if offsetArg != nil {
		_ = offset
		panic("TODO: offset")
	}

	regexp, err := prepareRegexp(string(pattern))
	if err != nil {
		return nil, err
	}

	m := regexp.NewMatcher()

	if !m.MatchString(string(subject), 0) {
		return phpv.ZInt(0).ZVal(), nil
	}

	if matchesArg != nil {
		for _, elem := range m.ExtractString() {
			(*matchesArg).OffsetSet(ctx, nil, phpv.ZStr(elem))
		}
	}

	return phpv.ZInt(1).ZVal(), nil
}
