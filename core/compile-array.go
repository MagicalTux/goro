package core

import (
	"io"

	"github.com/MagicalTux/gophp/core/tokenizer"
)

type arrayEntry struct {
	k, v Runnable
}

type runArray struct {
	e []*arrayEntry
	l *Loc
}

func (a runArray) Run(ctx Context) (*ZVal, error) {
	var err error
	array := NewZArray()

	for _, e := range a.e {
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

		array.OffsetSet(ctx, k, v.ZVal())
	}

	return array.ZVal(), nil
}

func (a *runArray) Loc() *Loc {
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
	value  Runnable
	offset Runnable
	l      *Loc
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

func (ac *runArrayAccess) Run(ctx Context) (*ZVal, error) {
	v, err := ac.value.Run(ctx)
	if err != nil {
		return nil, err
	}

	switch v.GetType() {
	case ZtString:
	case ZtArray:
	case ZtObject:
	default:
		v, err = v.As(ctx, ZtArray)
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

	if v.GetType() == ZtString {
		if offset.GetType() != ZtInt {
			// PHP Warning:  Illegal string offset 'abc'
			offset, err = offset.As(ctx, ZtInt)
			if err != nil {
				return nil, err
			}
		}
		str := v.String()
		iofft := int(offset.Value().(ZInt))

		if iofft < 0 {
			iofft = len(str) + iofft
		}

		if iofft < 0 || iofft >= len(str) {
			// PHP Notice:  Uninitialized string offset: 3
			return &ZVal{ZString("")}, nil
		}

		return &ZVal{ZString([]byte{str[iofft]})}, nil
	}

	array := v.Array()
	if array == nil {
		return nil, ac.l.Errorf(ctx, E_WARNING, "Cannot use object of type %s as array", v.GetType())
	}

	// OK...
	return array.OffsetGet(ctx, offset)
}

func (a *runArrayAccess) Loc() *Loc {
	return a.l
}

func (ac *runArrayAccess) WriteValue(ctx Context, value *ZVal) error {
	v, err := ac.value.Run(ctx)
	if err != nil {
		return err
	}

	switch v.GetType() {
	case ZtArray:
	case ZtObject:
	default:
		err = v.CastTo(ctx, ZtArray)
		if err != nil {
			return err
		}
		if wr, ok := ac.value.(Writable); ok {
			wr.WriteValue(ctx, v)
		}
	}

	array := v.Array()
	if array == nil {
		return ac.l.Errorf(ctx, E_WARNING, "Cannot use object of type %s as array", v.GetType())
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
	case ZtResource, ZtFloat:
		offset, err = offset.As(ctx, ZtInt)
		if err != nil {
			return err
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
	return array.OffsetSet(ctx, offset, value)
}

func compileArray(i *tokenizer.Item, c compileCtx) (Runnable, error) {
	res := &runArray{l: MakeLoc(i.Loc())}

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
		var v Runnable
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

func compileArrayAccess(v Runnable, c compileCtx) (Runnable, error) {
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

	l := MakeLoc(i.Loc())

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
