package pcre

import "github.com/MagicalTux/goro/core/phpv"

// preg_last_error returns PREG_NO_ERROR since Go's regexp engine
// does not have the same failure modes as PCRE.
func pregLastError(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return PREG_NO_ERROR.ZVal(), nil
}

// preg_last_error_msg returns "" (no error) for the same reason.
func pregLastErrorMsg(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZString("No error").ZVal(), nil
}
