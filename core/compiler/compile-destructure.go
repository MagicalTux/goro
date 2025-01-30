package compiler

import (
	"io"

	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type zList struct {
	elems *phpv.ZArray
}

func (zl *zList) GetType() phpv.ZType { return phpv.ZtArray }
func (zl *zList) ZVal() *phpv.ZVal    { return phpv.NewZVal(zl) }
func (zl *zList) Value() phpv.Val     { return zl }
func (zl *zList) AsVal(ctx phpv.Context, t phpv.ZType) (phpv.Val, error) {
	switch t {
	case phpv.ZtArray:
		return zl, nil
	default:
		return zl.elems.AsVal(ctx, t)
	}
}

func (zl *zList) String() string {
	return "list(...)"
}

func (zl *zList) WriteValue(ctx phpv.Context, value *phpv.ZVal) error {
	if value.GetType() != phpv.ZtArray {
		return nil
	}
	array := value.AsArray(ctx)

	i := -1
	for _, v := range zl.elems.Iterate(ctx) {
		i++
		if v == nil {
			continue
		}
		val, _ := array.OffsetGet(ctx, phpv.ZInt(i))
		if subList, ok := v.Value().(*zList); ok {
			err := subList.WriteValue(ctx, val)
			if err != nil {
				return err
			}
			continue
		}

		if v.GetName() == "" {
			return ctx.Errorf("Assignments can only happen to writable values")
		}

		err := ctx.OffsetSet(ctx, v.GetName(), val.Dup())
		if err != nil {
			return err
		}
	}

	return nil
}

type destructureEntry struct {
	k, v phpv.Runnable
}

type runDestructure struct {
	e []*destructureEntry
	l *phpv.Loc
}

func (rd *runDestructure) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	list := &zList{phpv.NewZArray()}

	var err error
	for _, e := range rd.e {
		var k, v *phpv.ZVal

		if e.k != nil {
			k, err = e.k.Run(ctx)
			if err != nil {
				return nil, err
			}
		}

		if e.v != nil {
			v, err = e.v.Run(ctx)
			if err != nil {
				return nil, err
			}
		}

		list.elems.OffsetSet(ctx, k, v.ZVal())
	}

	if list.elems.Count(ctx) == 0 {
		return nil, ctx.Errorf("Cannot use empty list")
	}

	return list.ZVal(), nil
}

func (a *runDestructure) Dump(w io.Writer) error {
	_, err := w.Write([]byte("list()"))
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
	_, err = w.Write([]byte{')'})
	return err
}

func (a *runDestructure) WriteValue(ctx phpv.Context, value *phpv.ZVal) error {
	val, err := a.Run(ctx)
	if err != nil {
		return err
	}
	list := val.Value().(*zList)
	return list.WriteValue(ctx, value)

}

func compileBaseDestructure(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	lhs, err := compileDestructure(nil, c)
	if err != nil {
		return nil, err
	}

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.Type != tokenizer.Rune('=') {
		return nil, i.Unexpected()
	}

	rhs, err := compileOpExpr(nil, c)
	if err != nil {
		return nil, err
	}

	return spawnOperator(c, i.Type, lhs, rhs, i.Loc())
}

func compileDestructure(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	var err error
	if i == nil {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	if !i.IsSingle('(') {
		return nil, i.Unexpected()
	}

	res := &runDestructure{l: i.Loc()}

	for {
		i, err := c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(')') {
			break
		}

		if i.IsSingle(',') {
			// empty slot is allowed: list($x,)
			res.e = append(res.e, &destructureEntry{v: nil})
			continue
		}

		isList := false
		var k phpv.Runnable
		if i.Type == tokenizer.T_LIST {
			isList = true
			k, err = compileDestructure(nil, c)
		} else {
			k, err = compileExpr(i, c)
		}
		if err != nil {
			return nil, err
		}

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(',') {
			res.e = append(res.e, &destructureEntry{v: k})
			continue
		}

		if i.IsSingle(')') {
			res.e = append(res.e, &destructureEntry{v: k})
			break
		}

		doubleArrow := i.Type == tokenizer.T_DOUBLE_ARROW
		// list() cannot be used as a key: list(list() => $x) // invalid
		if (isList && doubleArrow) || (!isList && !doubleArrow) {
			return nil, i.Unexpected()
		}

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		var v phpv.Runnable
		if i.Type == tokenizer.T_LIST {
			v, err = compileDestructure(nil, c)
		} else {
			v, err = compileExpr(i, c)
		}
		if err != nil {
			return nil, err
		}

		res.e = append(res.e, &destructureEntry{k: k, v: v})

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(',') {
			continue
		}

		if i.IsSingle(')') {
			break
		}
		return nil, i.Unexpected()
	}

	return res, nil
}
