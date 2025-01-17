package compiler

import (
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runnableForeach struct {
	src  phpv.Runnable
	code phpv.Runnable
	lv   phpv.Runnable
	k, v phpv.ZString
	l    *phpv.Loc
}

func (r *runnableForeach) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	z, err := r.src.Run(ctx)
	if err != nil {
		return nil, err
	}

	it := z.NewIterator()
	if it == nil {
		return nil, nil
	}

	var list *zList
	if r.lv != nil {
		val, err := r.lv.Run(ctx)
		if err != nil {
			return nil, err
		}
		list = val.Value().(*zList)
	}

	for {
		err = ctx.Tick(ctx, r.l)
		if err != nil {
			return nil, err
		}

		if !it.Valid(ctx) {
			break
		}

		if r.k != "" {
			k, err := it.Key(ctx)
			if err != nil {
				return nil, err
			}
			ctx.OffsetSet(ctx, r.k.ZVal(), k.Dup())
		}

		v, err := it.Current(ctx)
		if err != nil {
			return nil, err
		}

		if list != nil {
			err = list.WriteValue(ctx, v)
			if err != nil {
				return nil, err
			}
		} else {
			ctx.OffsetSet(ctx, r.v.ZVal(), v.Dup())
		}

		if r.code != nil {
			_, err = r.code.Run(ctx)
			if err != nil {
				e := r.l.Error(err)
				switch br := e.Err.(type) {
				case *phperr.PhpBreak:
					if br.Intv > 1 {
						br.Intv -= 1
						return nil, br
					}
					return nil, nil
				case *phperr.PhpContinue:
					if br.Intv > 1 {
						br.Intv -= 1
						return nil, br
					}
					it.Next(ctx)
					continue
				}
				return nil, e
			}
		}

		it.Next(ctx)
	}
	return nil, nil
}

func (r *runnableForeach) Dump(w io.Writer) error {
	_, err := w.Write([]byte("foreach("))
	if err != nil {
		return err
	}
	err = r.src.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(" as "))
	if err != nil {
		return err
	}
	if r.k == "" {
		_, err = fmt.Fprintf(w, "$%s) {", r.v)
	} else {
		_, err = fmt.Fprintf(w, "$%s => $%s) {", r.k, r.v)
	}
	if err != nil {
		return err
	}
	err = r.code.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{'}'})
	return err
}

func compileForeach(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// T_FOREACH (expression T_AS T_VARIABLE [=> T_VARIABLE]) ...?
	l := i.Loc()

	// parse while expression
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}
	if !i.IsSingle('(') {
		return nil, i.Unexpected()
	}

	r := &runnableForeach{l: l}
	r.src, err = compileExpr(nil, c)
	if err != nil {
		return nil, err
	}

	// check for T_AS
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	if i.Type != tokenizer.T_AS {
		return nil, i.Unexpected()
	}

	// check for T_VARIABLE or T_LIST
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	switch i.Type {
	case tokenizer.T_LIST:
		r.lv, err = compileDestructure(nil, c)
		if err != nil {
			return nil, err
		}
	case tokenizer.T_VARIABLE:
		// store in r.k or r.v ?
		r.v = phpv.ZString(i.Data[1:]) // remove $
	default:
		return nil, i.Unexpected()
	}

	// check for ) or =>
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.Type == tokenizer.T_DOUBLE_ARROW {
		if r.lv != nil {
			// foreach($arr as list(...) => $x) is invalid
			return nil, i.Unexpected()
		}

		// check for T_VARIABLE or T_LIST again
		r.k = r.v

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		switch i.Type {
		case tokenizer.T_LIST:
			r.lv, err = compileDestructure(nil, c)
			if err != nil {
				return nil, err
			}
		case tokenizer.T_VARIABLE:
			r.v = phpv.ZString(i.Data[1:])
		default:
			return nil, i.Unexpected()
		}

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	if !i.IsSingle(')') {
		return nil, i.Unexpected()
	}

	// check for ;
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	if i.IsSingle(';') {
		return r, nil
	}

	altForm := i.IsSingle(':')
	c.backup()

	r.code, err = compileBaseSingle(nil, c)
	if err != nil {
		return nil, err
	}

	if altForm {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.Type != tokenizer.T_ENDFOREACH {
			return nil, i.Unexpected()
		}
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if !i.IsExpressionEnd() {
			return nil, i.Unexpected()
		}
	}

	return r, nil
}
