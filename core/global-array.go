package core

import (
	"errors"

	"github.com/MagicalTux/goro/core/phpv"
)

func (c *Global) OffsetExists(ctx phpv.Context, name *phpv.ZVal) (bool, error) {
	name, err := name.As(ctx, phpv.ZtString)
	if err != nil {
		return false, err
	}

	switch name.AsString(ctx) {
	case "GLOBALS":
		return true, nil
	}

	return c.h.HasString(name.AsString(ctx)), nil
}

func (c *Global) OffsetGet(ctx phpv.Context, name *phpv.ZVal) (*phpv.ZVal, error) {
	name, err := name.As(ctx, phpv.ZtString)
	if err != nil {
		return nil, err
	}

	switch name.AsString(ctx) {
	case "GLOBALS":
		return c.h.Array().ZVal(), nil
	}
	return c.h.GetString(name.AsString(ctx)), nil
}

func (c *Global) OffsetSet(ctx phpv.Context, name, v *phpv.ZVal) error {
	name, err := name.As(ctx, phpv.ZtString)
	if err != nil {
		return err
	}

	switch name.AsString(ctx) {
	case "this":
		return errors.New("Cannot re-assign $this")
	}
	return c.h.SetString(name.AsString(ctx), v)
}

func (c *Global) OffsetUnset(ctx phpv.Context, name *phpv.ZVal) error {
	name, err := name.As(ctx, phpv.ZtString)
	if err != nil {
		return err
	}

	switch name.AsString(ctx) {
	case "this":
		return errors.New("Cannot unset $this")
	}
	return c.h.UnsetString(name.AsString(ctx))
}

func (c *Global) Count(ctx phpv.Context) phpv.ZInt {
	return c.h.Count()
}

func (c *Global) NewIterator() phpv.ZIterator {
	return c.h.NewIterator()
}
