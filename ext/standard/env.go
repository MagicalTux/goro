package standard

import (
	"strings"

	"github.com/MagicalTux/gophp/core"
)

//> func string getenv ( string $varname [, bool $local_only = FALSE ] )
func getenv(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var varname core.ZString
	var local_only *core.ZBool

	_, err := core.Expand(ctx, args, &varname, &local_only)
	if err != nil {
		return nil, err
	}

	v, ok := ctx.GetGlobal().Getenv(string(varname))
	if !ok {
		return core.ZBool(false).ZVal(), nil
	}

	return core.ZString(v).ZVal(), nil
}

//> func bool putenv ( string $setting )
func putenv(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var setting string
	_, err := core.Expand(ctx, args, &setting)
	if err != nil {
		return nil, err
	}

	pos := strings.IndexByte(setting, '=')
	if pos == -1 {
		// unset
		ctx.GetGlobal().Unsetenv(setting)
	} else {
		ctx.GetGlobal().Setenv(setting[:pos], setting[pos+1:])
	}
	return core.ZBool(true).ZVal(), nil
}
