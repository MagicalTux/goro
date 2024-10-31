//go:build !linux && !darwin
// +build !linux,!darwin

package standard

import (
	"os"
	"runtime"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// fallback uname

func fncUname(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var arg string
	_, err := core.Expand(ctx, args, &arg)
	if err != nil {
		return nil, err
	}

	switch arg {
	case "s":
		return phpv.ZString(runtime.GOOS).ZVal(), nil
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
		return phpv.ZString(runtime.GOOS + " " + n + " " + runtime.GOARCH).ZVal(), nil
	}
}
