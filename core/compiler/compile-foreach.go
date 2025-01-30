package compiler

import (
	"errors"
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runnableForeach struct {
	src  phpv.Runnable
	code phpv.Runnable
	k, v phpv.Runnable
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

	for {
		err = ctx.Tick(ctx, r.l)
		if err != nil {
			return nil, err
		}

		if !it.Valid(ctx) {
			break
		}

		if r.k != nil {
			k, err := it.Key(ctx)
			if err != nil {
				return nil, err
			}
			if w, ok := r.k.(phpv.Writable); !ok {
				return nil, errors.New("foreach key must be writable")
			} else {
				w.WriteValue(ctx, k.Dup())
			}
		}

		v, err := it.Current(ctx)
		if err != nil {
			return nil, err
		}

		if w, ok := r.v.(phpv.Writable); !ok {
			return nil, errors.New("foreach value must be writable")
		} else {
			w.WriteValue(ctx, v.Dup())
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
	if r.k == nil {
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

func compileForeachExpr(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	var err error
	if i == nil {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	var res phpv.Runnable

	// in addition to the list() and $varname,
	// foreach key/val take any LHS expression, such as:
	// - $x
	// - $x['a'][0]
	// - $obj->x
	// - $obj->x->y
	// - &$x
	// - foo()[$x]
	// The following are not parse errors, but still throws an error:
	// - foo()
	// - ""

	switch i.Type {
	case tokenizer.T_LIST:
		res, err = compileDestructure(nil, c)
		if err != nil {
			return nil, err
		}
	case tokenizer.T_VARIABLE:
		// store in r.k or r.v ?
		res = &runVariable{phpv.ZString(i.Data[1:]), i.Loc()}
		i2, err := c.NextItem()
		if err != nil {
			return nil, err
		}
		if !i2.IsSingle('[') {
			c.backup()
		} else {
			res, err = compilePostExpr(res, i2, c)
			if err != nil {
				return nil, err
			}
		}

	default:
		return nil, i.Unexpected()
	}

	return res, nil
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

	r.v, err = compileForeachExpr(nil, c)
	if err != nil {
		return nil, err
	}

	// check for ) or =>
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.Type == tokenizer.T_DOUBLE_ARROW {
		if _, ok := r.v.(*runDestructure); ok {
			// foreach($arr as list(...) => $x) is invalid
			return nil, i.Unexpected()
		}

		// check for T_VARIABLE or T_LIST again
		r.k = r.v

		r.v, err = compileForeachExpr(nil, c)
		if err != nil {
			return nil, err
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
