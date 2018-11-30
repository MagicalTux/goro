package standard

import (
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

//> func string getenv ( string $varname [, bool $local_only = FALSE ] )
func getenv(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var varname phpv.ZString
	var local_only *phpv.ZBool

	_, err := core.Expand(ctx, args, &varname, &local_only)
	if err != nil {
		return nil, err
	}

	v, ok := ctx.Global().Getenv(string(varname))
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZString(v).ZVal(), nil
}

//> func bool putenv ( string $setting )
func putenv(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var setting string
	_, err := core.Expand(ctx, args, &setting)
	if err != nil {
		return nil, err
	}

	pos := strings.IndexByte(setting, '=')
	if pos == -1 {
		// unset
		ctx.Global().Unsetenv(setting)
	} else {
		ctx.Global().Setenv(setting[:pos], setting[pos+1:])
	}
	return phpv.ZBool(true).ZVal(), nil
}
