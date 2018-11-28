package standard

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

//> func string getcwd ( void )
func fncGetcwd(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	cwd := ctx.Global().(*core.Global).Getwd()
	if cwd == "" {
		return phpv.ZBool(false).ZVal(), nil
	}

	return cwd.ZVal(), nil
}

//> func bool chdir ( string $directory )
func fncChdir(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var p phpv.ZString
	_, err := core.Expand(ctx, args, &p)
	if err != nil {
		return nil, err
	}

	return nil, ctx.Global().(*core.Global).Chdir(p)
}
