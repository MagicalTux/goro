//go:build linux
// +build linux

package standard

import (
	"syscall"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

func fncUnameHelperToString(v [65]int8) phpv.ZString {
	out := make([]byte, len(v))
	for i := 0; i < len(v); i++ {
		if v[i] == 0 {
			return phpv.ZString(out[:i-1])
		}
		out[i] = byte(v[i])
	}
	return phpv.ZString(out)
}

// > func string php_uname ([ string $mode = "a" ] )
func fncUname(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var arg string
	_, err := core.Expand(ctx, args, &arg)
	if err != nil {
		return nil, err
	}

	var name syscall.Utsname
	if err := syscall.Uname(&name); err != nil {
		return nil, err
	}

	switch arg {
	case "s":
		return fncUnameHelperToString(name.Sysname).ZVal(), nil
	case "n":
		return (fncUnameHelperToString(name.Nodename) + "." + fncUnameHelperToString(name.Domainname)).ZVal(), nil
	case "r":
		return fncUnameHelperToString(name.Release).ZVal(), nil
	case "v":
		return fncUnameHelperToString(name.Version).ZVal(), nil
	case "m":
		return fncUnameHelperToString(name.Machine).ZVal(), nil
	default:
		fallthrough
	case "a":
		// return full uname, ie "s n r v m"
		return (fncUnameHelperToString(name.Sysname) + " " + fncUnameHelperToString(name.Nodename) + "." + fncUnameHelperToString(name.Domainname) + " " + fncUnameHelperToString(name.Release) + " " + fncUnameHelperToString(name.Version) + " " + fncUnameHelperToString(name.Machine)).ZVal(), nil
	}
}
