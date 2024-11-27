package standard

import (
	"fmt"
	"time"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func mixed microtime ([ bool $get_as_float = FALSE ] )
func fncMicrotime(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var asFloat *phpv.ZBool
	_, err := core.Expand(ctx, args, &asFloat)
	if err != nil {
		return nil, err
	}

	t := time.Now()
	if asFloat != nil && *asFloat {
		// return as float
		fv := float64(t.Unix()) + float64(t.Nanosecond())/1e9
		return phpv.ZFloat(fv).ZVal(), nil
	}

	// return as string
	r := fmt.Sprintf("%0.8f %d", float64(t.Nanosecond())/1e9, t.Unix())
	return phpv.ZString(r).ZVal(), nil
}

// > func int time ( void )
func fncTime(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZInt(time.Now().Unix()).ZVal(), nil
}

// > func int mktime ( [ int $hour = date("H") [, int $minute = date("i") [, int $second = date("s") [, int $month = date("n") [, int $day = date("j") [, int $year = date("Y") [, int $is_dst = -1 ]]]]]]] )
func fncMkTime(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var hourArg, minArg, secArg, monthArg, dayArg, yearArg, dstArg *int
	_, err := core.Expand(ctx, args, &hourArg, &minArg, &secArg, &monthArg, &dayArg, &yearArg, &dstArg)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	hour := now.Hour()
	min := now.Minute()
	sec := now.Second()
	month := now.Month()
	day := now.Day()
	year := now.Year()

	if hourArg != nil {
		hour = *hourArg
	}
	if minArg != nil {
		min = *minArg
	}
	if secArg != nil {
		sec = *secArg
	}
	if monthArg != nil {
		month = time.Month(*monthArg)
	}
	if dayArg != nil {
		day = *dayArg
	}
	if yearArg != nil {
		year = *yearArg
	}

	date := time.Date(year, month, day, hour, min, sec, 0, time.UTC)
	return phpv.ZInt(date.Unix()).ZVal(), nil
}
