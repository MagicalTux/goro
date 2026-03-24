package core

import (
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// PHP assert option constants
const (
	ASSERT_ACTIVE    phpv.ZInt = 1
	ASSERT_WARNING   phpv.ZInt = 2
	ASSERT_CALLBACK  phpv.ZInt = 3
	ASSERT_BAIL      phpv.ZInt = 4
	ASSERT_QUIET     phpv.ZInt = 5
	ASSERT_EXCEPTION phpv.ZInt = 6
)

// Maps assert option constants to their INI setting names
var assertOptionToIni = map[phpv.ZInt]phpv.ZString{
	ASSERT_ACTIVE:    "assert.active",
	ASSERT_WARNING:   "assert.warning",
	ASSERT_BAIL:      "assert.bail",
	ASSERT_QUIET:     "assert.quiet_eval",
	ASSERT_EXCEPTION: "assert.exception",
}

// Default values for each assert option
var assertOptionDefaults = map[phpv.ZInt]phpv.ZInt{
	ASSERT_ACTIVE:    1,
	ASSERT_WARNING:   1,
	ASSERT_BAIL:      0,
	ASSERT_QUIET:     0,
	ASSERT_EXCEPTION: 1,
}

// > func mixed assert_options ( int $option [, mixed $value ] )
func fncAssertOptions(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var option phpv.ZInt
	var value Optional[*phpv.ZVal]
	_, err := Expand(ctx, args, &option, &value)
	if err != nil {
		return nil, err
	}

	// assert_options() is deprecated since PHP 8.3
	if err := ctx.Deprecated("Function assert_options() is deprecated since 8.3", logopt.NoFuncName(true)); err != nil {
		return nil, err
	}

	// Validate option - must be one of the ASSERT_* constants
	if _, ok := assertOptionToIni[option]; !ok && option != ASSERT_CALLBACK {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "assert_options(): Argument #1 ($option) must be an ASSERT_* constant")
	}

	if option == ASSERT_CALLBACK {
		// ASSERT_CALLBACK is stored as a special INI-like config
		iniName := phpv.ZString("assert.callback")
		old := ctx.GetConfig(iniName, phpv.ZNULL.ZVal())
		if value.HasArg() {
			ctx.Global().SetLocalConfig(iniName, value.Get())
		}
		return old, nil
	}

	iniName, ok := assertOptionToIni[option]
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}

	// Get the current value
	def := assertOptionDefaults[option]
	old := ctx.GetConfig(iniName, def.ZVal())
	oldInt := old.AsInt(ctx)

	if value.HasArg() {
		// Set the new value
		newVal := value.Get()
		ctx.Global().SetLocalConfig(iniName, newVal)
	}

	return phpv.ZInt(oldInt).ZVal(), nil
}
