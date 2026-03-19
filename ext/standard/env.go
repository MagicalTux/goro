package standard

import (
	"os"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func string|array|false getenv ([ string $varname [, bool $local_only = FALSE ]] )
func getenv(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var varname *phpv.ZString
	var local_only *phpv.ZBool

	_, err := core.Expand(ctx, args, &varname, &local_only)
	if err != nil {
		return nil, err
	}

	if varname == nil {
		// Return all environment variables as an array
		result := phpv.NewZArray()
		for _, envVar := range os.Environ() {
			pos := strings.IndexByte(envVar, '=')
			if pos == -1 {
				continue
			}
			k := envVar[:pos]
			v := envVar[pos+1:]
			result.OffsetSet(ctx, phpv.ZString(k).ZVal(), phpv.ZString(v).ZVal())
		}
		return result.ZVal(), nil
	}

	v, ok := ctx.Global().Getenv(string(*varname))
	if !ok {
		return phpv.ZBool(false).ZVal(), nil
	}

	return phpv.ZString(v).ZVal(), nil
}

// > func bool putenv ( string $setting )
func putenv(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var setting string
	_, err := core.Expand(ctx, args, &setting)
	if err != nil {
		return nil, err
	}

	// Validate: empty string or starts with '=' are invalid
	if setting == "" || setting[0] == '=' {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			"putenv(): Argument #1 ($assignment) must have a valid syntax")
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
