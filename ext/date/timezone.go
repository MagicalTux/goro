package date

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func bool date_default_timezone_set ( string $timezoneId )
func fncDateDefaultTimezoneSet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var tz phpv.ZString
	_, err := core.Expand(ctx, args, &tz)
	if err != nil {
		return nil, err
	}

	// Store timezone in global config (stub for now, actual timezone handling TODO)
	ctx.Global().SetLocalConfig("date.timezone", tz.ZVal())
	return phpv.ZBool(true).ZVal(), nil
}

// > func string date_default_timezone_get ( void )
func fncDateDefaultTimezoneGet(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	tz := ctx.GetConfig("date.timezone", phpv.ZString("UTC").ZVal())
	return tz.As(ctx, phpv.ZtString)
}
