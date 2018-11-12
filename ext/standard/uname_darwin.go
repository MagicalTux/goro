// +build darwin

package standard

import (
	"context"
	"os"
	"runtime"

	"github.com/MagicalTux/gophp/core"
)

// this is an approximate estimation of php_uname()
func fncUname(ctx context.Context, params Values, out WritableValue) error {
	var arg string
	_, err := core.Expand(ctx, args, &arg)
	if err != nil {
		return nil, err
	}

	switch arg {
	case "s":
		return core.ZString("Darwin").ZVal(), nil
	case "n":
		n, err := os.Hostname()
		if err != nil {
			return nil, err
		}
		return core.ZString(n).ZVal(), nil
	case "r":
		return core.ZString("?").ZVal(), nil
	case "m":
		return core.ZString(runtime.GOARCH).ZVal(), nil
	default:
		fallthrough
	case "a":
		n, err := os.Hostname()
		if err != nil {
			return nil, err
		}
		// return full uname, ie "s n r v m"
		return core.ZString("Darwin " + n + " " + runtime.GOARCH).ZVal(), nil
	}
}
