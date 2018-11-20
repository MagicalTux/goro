package standard

import "github.com/MagicalTux/gophp/core"

//> func string getcwd ( void )
func fncGetcwd(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	cwd := ctx.Global().Getwd()
	if cwd == "" {
		return core.ZBool(false).ZVal(), nil
	}

	return cwd.ZVal(), nil
}
