package date

import (
	"time"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/KarpelesLab/strftime"
)

//> func string strftime ( string $format [, int $timestamp = time() ] )
func fncStrftime(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var f phpv.ZString
	var ts *phpv.ZInt
	_, err := core.Expand(ctx, args, &f, &ts)
	if err != nil {
		return nil, err
	}

	var t time.Time
	if ts != nil {
		t = time.Unix(int64(*ts), 0)
	} else {
		t = time.Now()
	}

	// TODO support locales, timezones, etc
	return phpv.ZString(strftime.EnFormat(string(f), t)).ZVal(), nil
}
