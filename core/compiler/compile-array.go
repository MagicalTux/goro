package compiler

import (
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type arrayEntry struct {
	k, v phpv.Runnable
}

type runArray struct {
	e []*arrayEntry
	l *phpv.Loc
}

func (a runArray) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	var err error
	array := phpv.NewZArray()

	for _, e := range a.e {
		var k, v *phpv.ZVal

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

		array.OffsetSet(ctx, k, v.ZVal())
	}

	return array.ZVal(), nil
}

func (a *runArray) Loc() *phpv.Loc {
	return a.l
}

func (a runArray) Dump(w io.Writer) error {
	_, err := w.Write([]byte{'['})
	if err != nil {
		return err
	}
	for _, s := range a.e {
		if s.k != nil {
			err = s.k.Dump(w)
			if err != nil {
				return err
			}
			_, err = w.Write([]byte("=>"))
			if err != nil {
				return err
			}
		}
		err = s.v.Dump(w)
		if err != nil {
			return err
		}
	}
	_, err = w.Write([]byte{']'})
	return err
}

type runArrayAccess struct {
	value  phpv.Runnable
	offset phpv.Runnable
	l      *phpv.Loc
}

func (r *runArrayAccess) Dump(w io.Writer) error {
	err := r.value.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{'['})
	if err != nil {
		return err
	}
	if r.offset != nil {
		err = r.offset.Dump(w)
		if err != nil {
			return err
		}
	}
	_, err = w.Write([]byte{']'})
	return err
}

func (ac *runArrayAccess) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	v, err := ac.value.Run(ctx)
	if err != nil {
		return nil, err
	}

	switch v.GetType() {
	case phpv.ZtString:
	case phpv.ZtArray:
	case phpv.ZtObject:
	default:
		v, err = v.As(ctx, phpv.ZtArray)
		if err != nil {
			return nil, err
		}
	}

	if ac.offset == nil {
		return nil, nil // FIXME PHP Fatal error:  Cannot use [] for reading
	}

	offset, err := ac.offset.Run(ctx)
	if err != nil {
		return nil, err
	}

	switch offset.GetType() {
	case phpv.ZtResource, phpv.ZtFloat:
		offset, err = offset.As(ctx, phpv.ZtInt)
		if err != nil {
			return nil, err
		}
	case phpv.ZtString:
	case phpv.ZtInt:
	case phpv.ZtObject:
		// check if has __toString
		fallthrough
	case phpv.ZtArray:
		// Trigger: Illegal offset type
		fallthrough
	default:
		offset, err = offset.As(ctx, phpv.ZtString)
	}

	if v.GetType() == phpv.ZtString {
		if offset.GetType() != phpv.ZtInt {
			// PHP Warning:  Illegal string offset 'abc'
			offset, err = offset.As(ctx, phpv.ZtInt)
			if err != nil {
				return nil, err
			}
		}
		str := v.String()
		iofft := int(offset.Value().(phpv.ZInt))

		if iofft < 0 {
			iofft = len(str) + iofft
		}

		if iofft < 0 || iofft >= len(str) {
			// PHP Notice:  Uninitialized string offset: 3
			return phpv.ZString("").ZVal(), nil
		}

		return phpv.ZString([]byte{str[iofft]}).ZVal(), nil
	}

	array := v.Array()
	if array == nil {
		err := fmt.Errorf("Cannot use object of type %s as array", v.GetType())
		return nil, ctx.Error(err, phpv.E_WARNING)
	}

	// OK...
	return array.OffsetGet(ctx, offset)
}

func (a *runArrayAccess) Loc() *phpv.Loc {
	return a.l
}

func (ac *runArrayAccess) WriteValue(ctx phpv.Context, value *phpv.ZVal) error {
	v, err := ac.value.Run(ctx)
	if err != nil {
		return err
	}

	switch v.GetType() {
	case phpv.ZtArray:
	case phpv.ZtObject:
	default:
		err = v.CastTo(ctx, phpv.ZtArray)
		if err != nil {
			return err
		}
		if wr, ok := ac.value.(phpv.Writable); ok {
			wr.WriteValue(ctx, v)
		}
	}

	array := v.Array()
	if array == nil {
		err := fmt.Errorf("Cannot use object of type %s as array", v.GetType())
		return ac.l.Error(err, phpv.E_WARNING)
	}

	if ac.offset == nil {
		// append
		return array.OffsetSet(ctx, nil, value)
	}

	offset, err := ac.offset.Run(ctx)
	if err != nil {
		return err
	}

	switch offset.GetType() {
	case phpv.ZtResource, phpv.ZtFloat:
		offset, err = offset.As(ctx, phpv.ZtInt)
		if err != nil {
			return err
		}
	case phpv.ZtString:
	case phpv.ZtInt:
	case phpv.ZtObject:
		// check if has __toString
		fallthrough
	case phpv.ZtArray:
		// Trigger: Illegal offset type
		fallthrough
	default:
		offset, err = offset.As(ctx, phpv.ZtString)
	}

	// OK...
	return array.OffsetSet(ctx, offset, value)
}

func compileArray(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	res := &runArray{l: i.Loc()}

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

		var k phpv.Runnable
		k, err = compileExpr(i, c)
		if err != nil {
			return nil, err
		}

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(',') {
			res.e = append(res.e, &arrayEntry{v: k})
			continue
		}

		if i.IsSingle(array_type) {
			res.e = append(res.e, &arrayEntry{v: k})
			break
		}

		if i.Type != tokenizer.T_DOUBLE_ARROW {
			return nil, i.Unexpected()
		}

		// ok we got a value now
		var v phpv.Runnable
		v, err = compileExpr(nil, c)
		if err != nil {
			return nil, err
		}

		res.e = append(res.e, &arrayEntry{k: k, v: v})

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

func compileArrayAccess(v phpv.Runnable, c compileCtx) (phpv.Runnable, error) {
	// we got a [
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	var endc rune
	switch i.Rune() {
	case '[':
		endc = ']'
	case '{':
		endc = '}'
	default:
		return nil, i.Unexpected()
	}

	l := i.Loc()

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	if i.IsSingle(endc) {
		v = &runArrayAccess{value: v, offset: nil, l: l}
		return v, nil
	}
	c.backup()

	// don't really need this loop anymore?
	offt, err := compileExpr(nil, c)
	if err != nil {
		return nil, err
	}

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	if !i.IsSingle(endc) {
		return nil, i.Unexpected()
	}

	v = &runArrayAccess{value: v, offset: offt, l: l}

	return v, nil
}
