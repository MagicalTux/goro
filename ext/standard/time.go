package standard

import (
	"fmt"
	"time"

	"github.com/MagicalTux/goro/core"
)

//> func mixed microtime ([ bool $get_as_float = FALSE ] )
func fncMicrotime(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var asFloat *core.ZBool
	_, err := core.Expand(ctx, args, &asFloat)
	if err != nil {
		return nil, err
	}

	t := time.Now()
	if asFloat != nil && *asFloat {
		// return as float
		fv := float64(t.Unix()) + float64(t.Nanosecond())/1e9
		return core.ZFloat(fv).ZVal(), nil
	}

	// return as string
	r := fmt.Sprintf("%0.8f %d", float64(t.Nanosecond())/1e9, t.Unix())
	return core.ZString(r).ZVal(), nil
}

//> func int time ( void )
func fncTime(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	return core.ZInt(time.Now().Unix()).ZVal(), nil
}
