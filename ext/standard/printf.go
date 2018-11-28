package standard

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

//> func string sprintf ( string $format [, mixed $args [, mixed $... ]] )
func fncSprintf(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var fmt phpv.ZString
	n, err := core.Expand(ctx, args, &fmt)
	if err != nil {
		return nil, err
	}

	return core.Zprintf(ctx, fmt, args[n:]...)
}
