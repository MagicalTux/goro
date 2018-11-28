// +build !linux,!darwin

package standard

import (
	"os"
	"runtime"

	"github.com/MagicalTux/goro/core"
)

// fallback uname

func fncUname(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var arg string
	_, err := core.Expand(ctx, args, &arg)
	if err != nil {
		return nil, err
	}

	switch arg {
	case "s":
		return core.ZString(runtime.GOOS).ZVal(), nil
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
		return core.ZString(runtime.GOOS + " " + n + " " + runtime.GOARCH).ZVal(), nil
	}
}
