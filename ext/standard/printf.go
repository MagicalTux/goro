package standard

import "github.com/MagicalTux/goro/core"

//> func string sprintf ( string $format [, mixed $args [, mixed $... ]] )
func fncSprintf(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var fmt core.ZString
	n, err := core.Expand(ctx, args, &fmt)
	if err != nil {
		return nil, err
	}

	return core.Zprintf(ctx, fmt, args[n:]...)
}
