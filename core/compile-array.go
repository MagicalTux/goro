package core

import (
	"errors"

	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

type arrayEntry struct {
	k, v Runnable
}

type runArray []*arrayEntry

func (a runArray) Run(ctx Context) (*ZVal, error) {
	var err error
	array := NewZArray()

	for _, e := range a {
		var k, v *ZVal

		if e.k != nil {
			k, err = e.k.Run(ctx)
			if err != nil {
				return nil, err
			}
		}
		v, err = e.v.Run(ctx)
		if err != nil {
			return nil, err
		}

		array.OffsetSet(k, v)
	}

	return &ZVal{array}, nil
}

type runArrayAccess struct {
	value  Runnable
	offset Runnable
}

func (ac *runArrayAccess) Run(ctx Context) (*ZVal, error) {
	v, err := ac.value.Run(ctx)
	if err != nil {
		return nil, err
	}

	switch v.GetType() {
	case ZtArray:
	case ZtObject:
	default:
		v, err = v.As(ctx, ZtArray)
		if err != nil {
			return nil, err
		}
	}

	array, ok := v.v.(ZArrayAccess)
	if !ok {
		return nil, errors.New("Cannot use object of type ?? as array") // TODO
	}

	offset, err := ac.offset.Run(ctx)
	if err != nil {
		return nil, err
	}

	switch offset.GetType() {
	case ZtResource, ZtFloat:
		offset, err = offset.As(ctx, ZtInt)
		if err != nil {
			return nil, err
		}
	case ZtString:
	case ZtInt:
	case ZtObject:
		// check if has __toString
		fallthrough
	case ZtArray:
		// Trigger: Illegal offset type
		fallthrough
	default:
		offset, err = offset.As(ctx, ZtString)
	}

	// OK...
	return array.OffsetGet(offset)
}

func compileArray(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	var res runArray

	array_type := '?'

	if i.IsSingle('[') {
		array_type = ']'
	} else if i.Type == tokenizer.T_ARRAY {
		i, err := c.NextItem()
		if err != nil {
			return nil, err
		}

		if !i.IsSingle('(') {
			return nil, i.Unexpected()
		}
		array_type = ')'
	} else {
		return nil, i.Unexpected()
	}

	for {
		i, err := c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(array_type) {
			break
		}

		var k Runnable
		k, err = compileExpr(i, c)
		if err != nil {
			return nil, err
		}

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(',') {
			res = append(res, &arrayEntry{v: k})
			continue
		}

		if i.IsSingle(array_type) {
			res = append(res, &arrayEntry{v: k})
			break
		}

		if i.Type != tokenizer.T_DOUBLE_ARROW {
			return nil, i.Unexpected()
		}

		// ok we got a value now
		var v Runnable
		v, err = compileExpr(nil, c)
		if err != nil {
			return nil, err
		}

		res = append(res, &arrayEntry{k: k, v: v})

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(',') {
			// TODO: append k
			continue
		}

		if i.IsSingle(array_type) {
			// TODO: append k
			break
		}
		return nil, i.Unexpected()
	}

	return res, nil
}

func compileArrayAccess(v Runnable, c *compileCtx) (Runnable, error) {
	// we got a [
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if !i.IsSingle('[') {
		return nil, i.Unexpected()
	}

	for {
		offt, err := compileExpr(nil, c)
		if err != nil {
			return nil, err
		}

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if !i.IsSingle(']') {
			return nil, i.Unexpected()
		}

		v = &runArrayAccess{value: v, offset: offt}

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle('[') {
			continue
		}
		c.backup()

		return v, nil
	}
}
