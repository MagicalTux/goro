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
