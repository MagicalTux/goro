package core

import (
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

// > func mixed assert_options ( int $option [, mixed $value ] )
func fncAssertOptions(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var option phpv.ZInt
	var value Optional[*phpv.ZVal]
	_, err := Expand(ctx, args, &option, &value)
	if err != nil {
		return nil, err
	}

	// PHP 8 deprecated most assert_options functionality.
	// Return the "current" value as a stub.
	switch option {
	case ASSERT_ACTIVE:
		// assert.active is on by default
		return phpv.ZInt(1).ZVal(), nil
	case ASSERT_WARNING:
		// assert.warning is on by default
		return phpv.ZInt(1).ZVal(), nil
	case ASSERT_BAIL:
		// assert.bail is off by default
		return phpv.ZInt(0).ZVal(), nil
	case ASSERT_QUIET:
		// assert.quiet_eval was removed in PHP 8
		return phpv.ZInt(0).ZVal(), nil
	case ASSERT_EXCEPTION:
		// assert.exception is on by default in PHP 8
		return phpv.ZInt(1).ZVal(), nil
	default:
		return phpv.ZFalse.ZVal(), nil
	}
}
