package standard

import "github.com/MagicalTux/goro/core"

//> func string getcwd ( void )
func fncGetcwd(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	cwd := ctx.Global().Getwd()
	if cwd == "" {
		return core.ZBool(false).ZVal(), nil
	}

	return cwd.ZVal(), nil
}

//> func bool chdir ( string $directory )
func fncChdir(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var p core.ZString
	_, err := core.Expand(ctx, args, &p)
	if err != nil {
		return nil, err
	}

	return nil, ctx.Global().Chdir(p)
}
