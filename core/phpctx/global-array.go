package phpctx

import (
	"errors"

	"github.com/MagicalTux/goro/core/phpv"
)

func (c *Global) OffsetExists(ctx phpv.Context, name phpv.Val) (bool, error) {
	name, err := name.AsVal(ctx, phpv.ZtString)
	if err != nil {
		return false, err
	}

	switch name.(phpv.ZString) {
	case "GLOBALS":
		return true, nil
	}

	return c.h.HasString(name.(phpv.ZString)), nil
}

func (c *Global) OffsetCheck(ctx phpv.Context, name phpv.Val) (*phpv.ZVal, bool, error) {
	name, err := name.AsVal(ctx, phpv.ZtString)
	if err != nil {
		return nil, false, err
	}
	switch name.(phpv.ZString) {
	case "GLOBALS":
		return c.h.Array().ZVal(), true, nil
	}

	if !c.h.HasString(name.(phpv.ZString)) {
		return nil, false, err
	}
	return c.h.GetString(name.(phpv.ZString)), true, nil
}

func (c *Global) OffsetGet(ctx phpv.Context, name phpv.Val) (*phpv.ZVal, error) {
	name, err := name.AsVal(ctx, phpv.ZtString)
	if err != nil {
		return nil, err
	}

	switch name.String() {
	case "GLOBALS":
		return c.h.Array().ZVal(), nil
	}
	return c.h.GetString(name.(phpv.ZString)), nil
}

func (c *Global) OffsetSet(ctx phpv.Context, name phpv.Val, v *phpv.ZVal) error {
	name, err := name.AsVal(ctx, phpv.ZtString)
	if err != nil {
		return err
	}

	switch name.Value().(phpv.ZString) {
	case "this":
		return errors.New("Cannot re-assign $this")
	}
	return c.h.SetString(name.(phpv.ZString), v)
}

func (c *Global) OffsetUnset(ctx phpv.Context, name phpv.Val) error {
	name, err := name.AsVal(ctx, phpv.ZtString)
	if err != nil {
		return err
	}

	switch name.(phpv.ZString) {
	case "this":
		return errors.New("Cannot unset $this")
	}
	return c.h.UnsetString(name.(phpv.ZString))
}

func (c *Global) Count(ctx phpv.Context) phpv.ZInt {
	return c.h.Count()
}

func (c *Global) NewIterator() phpv.ZIterator {
	return c.h.NewIterator()
}
