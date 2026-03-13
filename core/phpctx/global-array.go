package phpctx

import (
	"errors"
	"fmt"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

func (c *Global) OffsetExists(ctx phpv.Context, name phpv.Val) (bool, error) {
	nameStr, ok := name.(phpv.ZString)
	if !ok {
		var err error
		name, err = name.AsVal(ctx, phpv.ZtString)
		if err != nil {
			return false, err
		}
		nameStr = name.(phpv.ZString)
	}

	switch nameStr {
	case "GLOBALS":
		return true, nil
	}

	return c.h.HasString(nameStr), nil
}

func (c *Global) OffsetCheck(ctx phpv.Context, name phpv.Val) (*phpv.ZVal, bool, error) {
	nameStr, ok := name.(phpv.ZString)
	if !ok {
		var err error
		name, err = name.AsVal(ctx, phpv.ZtString)
		if err != nil {
			return nil, false, err
		}
		nameStr = name.(phpv.ZString)
	}

	switch nameStr {
	case "GLOBALS":
		return c.h.Array().ZVal(), true, nil
	}

	v, found := c.h.GetStringB(nameStr)
	if !found {
		return nil, false, nil
	}
	return v, true, nil
}

func (c *Global) OffsetGet(ctx phpv.Context, name phpv.Val) (*phpv.ZVal, error) {
	nameStr, ok := name.(phpv.ZString)
	if !ok {
		var err error
		name, err = name.AsVal(ctx, phpv.ZtString)
		if err != nil {
			return nil, err
		}
		nameStr = name.(phpv.ZString)
	}

	switch nameStr {
	case "GLOBALS":
		return c.h.Array().ZVal(), nil
	}
	return c.h.GetString(nameStr), nil
}

func (c *Global) OffsetSet(ctx phpv.Context, name phpv.Val, v *phpv.ZVal) error {
	nameStr, ok := name.(phpv.ZString)
	if !ok {
		var err error
		name, err = name.AsVal(ctx, phpv.ZtString)
		if err != nil {
			return err
		}
		nameStr = name.(phpv.ZString)
	}

	switch nameStr {
	case "this":
		return errors.New("Cannot re-assign $this")
	}

	// Eagerly call __destruct when overwriting a variable that holds an object.
	if old := c.h.GetString(nameStr); old != nil && old.GetType() == phpv.ZtObject {
		if obj, ok := old.Value().(phpv.ZObject); ok {
			if m, hasDestructor := obj.GetClass().GetMethod("__destruct"); hasDestructor {
				if canCallDestructor(ctx, m, obj) {
					err := c.h.SetString(nameStr, v)
					if err != nil {
						return err
					}
					if destructable, ok2 := obj.(interface {
						CallDestructor(phpv.Context) error
					}); ok2 {
						destructable.CallDestructor(ctx)
					}
					return nil
				}
				// PHP 8: inaccessible destructor throws Error
				scope := "global scope"
				if callerClass := ctx.Class(); callerClass != nil {
					scope = "scope of class " + string(callerClass.GetName())
				}
				visibility := "protected"
				if m.Modifiers.IsPrivate() {
					visibility = "private"
				}
				// Unregister from shutdown destructors to prevent duplicate call
				ctx.Global().UnregisterDestructor(obj)
				return phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("Call to %s %s::__destruct() from %s",
						visibility, obj.GetClass().GetName(), scope))
			}
		}
	}

	return c.h.SetString(nameStr, v)
}

func (c *Global) OffsetUnset(ctx phpv.Context, name phpv.Val) error {
	nameStr, ok := name.(phpv.ZString)
	if !ok {
		var err error
		name, err = name.AsVal(ctx, phpv.ZtString)
		if err != nil {
			return err
		}
		nameStr = name.(phpv.ZString)
	}

	switch nameStr {
	case "this":
		return errors.New("Cannot unset $this")
	}
	return c.h.UnsetString(nameStr)
}

func (c *Global) Count(ctx phpv.Context) phpv.ZInt {
	return c.h.Count()
}

func (c *Global) NewIterator() phpv.ZIterator {
	return c.h.NewIterator()
}
