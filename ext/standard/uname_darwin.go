// +build darwin

package standard

import (
	"os"
	"runtime"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// this is an approximate estimation of php_uname()
func fncUname(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var arg string
	_, err := core.Expand(ctx, args, &arg)
	if err != nil {
		return nil, err
	}

	switch arg {
	case "s":
		return phpv.ZString("Darwin").ZVal(), nil
	case "n":
		n, err := os.Hostname()
		if err != nil {
			return nil, err
		}
		return phpv.ZString(n).ZVal(), nil
	case "r":
		return phpv.ZString("?").ZVal(), nil
	case "m":
		return phpv.ZString(runtime.GOARCH).ZVal(), nil
	default:
		fallthrough
	case "a":
		n, err := os.Hostname()
		if err != nil {
			return nil, err
		}
		// return full uname, ie "s n r v m"
		return phpv.ZString("Darwin " + n + " " + runtime.GOARCH).ZVal(), nil
	}
}
