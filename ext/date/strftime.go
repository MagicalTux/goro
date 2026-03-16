package date

import (
	"time"

	"github.com/KarpelesLab/strftime"
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string strftime ( string $format [, int $timestamp = time() ] )
func fncStrftime(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var f phpv.ZString
	var ts *phpv.ZInt
	_, err := core.Expand(ctx, args, &f, &ts)
	if err != nil {
		return nil, err
	}

	ctx.Deprecated("Function strftime() is deprecated since 8.1, use IntlDateFormatter::format() instead", logopt.NoFuncName(true))

	if f == "" {
		return phpv.ZFalse.ZVal(), nil
	}

	loc := getTimezone(ctx)
	var t time.Time
	if ts != nil {
		t = time.Unix(int64(*ts), 0).In(loc)
	} else {
		t = time.Now().In(loc)
	}

	return phpv.ZString(strftime.EnFormat(string(f), t)).ZVal(), nil
}

// > func string gmstrftime ( string $format [, int $timestamp = time() ] )
func fncGmstrftime(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var f phpv.ZString
	var ts *phpv.ZInt
	_, err := core.Expand(ctx, args, &f, &ts)
	if err != nil {
		return nil, err
	}

	ctx.Deprecated("Function gmstrftime() is deprecated since 8.1, use IntlDateFormatter::format() instead", logopt.NoFuncName(true))

	if f == "" {
		return phpv.ZFalse.ZVal(), nil
	}

	var t time.Time
	if ts != nil {
		t = time.Unix(int64(*ts), 0).UTC()
	} else {
		t = time.Now().UTC()
	}

	return phpv.ZString(strftime.EnFormat(string(f), t)).ZVal(), nil
}
