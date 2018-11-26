package core

import "errors"

func (c *Global) OffsetExists(ctx Context, name *ZVal) (bool, error) {
	name, err := name.As(ctx, ZtString)
	if err != nil {
		return false, err
	}

	switch name.AsString(ctx) {
	case "GLOBALS":
		return true, nil
	}

	return c.h.HasString(name.AsString(ctx)), nil
}

func (c *Global) OffsetGet(ctx Context, name *ZVal) (*ZVal, error) {
	name, err := name.As(ctx, ZtString)
	if err != nil {
		return nil, err
	}

	switch name.AsString(ctx) {
	case "GLOBALS":
		return (&ZArray{h: c.h}).ZVal(), nil
	}
	return c.h.GetString(name.AsString(ctx)), nil
}

func (c *Global) OffsetSet(ctx Context, name, v *ZVal) error {
	name, err := name.As(ctx, ZtString)
	if err != nil {
		return err
	}

	switch name.AsString(ctx) {
	case "this":
		return errors.New("Cannot re-assign $this")
	}
	return c.h.SetString(name.AsString(ctx), v)
}

func (c *Global) OffsetUnset(ctx Context, name *ZVal) error {
	name, err := name.As(ctx, ZtString)
	if err != nil {
		return err
	}

	switch name.AsString(ctx) {
	case "this":
		return errors.New("Cannot unset $this")
	}
	return c.h.UnsetString(name.AsString(ctx))
}

func (c *Global) Count(ctx Context) ZInt {
	return c.h.count
}

func (c *Global) NewIterator() ZIterator {
	return c.h.NewIterator()
}
