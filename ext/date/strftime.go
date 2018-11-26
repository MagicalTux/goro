package date

import (
	"time"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/strftime"
)

//> func string strftime ( string $format [, int $timestamp = time() ] )
func fncStrftime(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var f core.ZString
	var ts *core.ZInt
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
	return core.ZString(strftime.EnFormat(string(f), t)).ZVal(), nil
}
