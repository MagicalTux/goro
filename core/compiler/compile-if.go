package compiler

import (
	"io"

	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runnableIf struct {
	cond phpv.Runnable
	yes  phpv.Runnable
	no   phpv.Runnable
	l    *phpv.Loc

	ternary bool
}

func (r *runnableIf) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	t, err := r.cond.Run(ctx)
	if err != nil {
		return nil, err
	}
	t, err = t.As(ctx, phpv.ZtBool)
	if err != nil {
		return nil, err
	}

	if t.Value().(phpv.ZBool) {
		return r.yes.Run(ctx)
	} else if r.no != nil {
		return r.no.Run(ctx)
	} else {
		return nil, nil
	}
}

func (r *runnableIf) dumpTernary(w io.Writer) error {
	_, err := w.Write([]byte("["))
	if err != nil {
		return err
	}
	err = r.cond.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(" ? "))
	if err != nil {
		return err
	}
	err = r.yes.Dump(w)
	if err != nil {
		return err
	}
	if r.no != nil {
		_, err = w.Write([]byte(" : "))
		if err != nil {
			return err
		}
		err = r.no.Dump(w)
		if err != nil {
			return err
		}
	}
	_, err = w.Write([]byte{']'})
	return err
}
func (r *runnableIf) Dump(w io.Writer) error {
	if r.ternary {
		return r.dumpTernary(w)
	}
	_, err := w.Write([]byte("if ("))
	if err != nil {
		return err
	}
	err = r.cond.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(") {"))
	if err != nil {
		return err
	}
	err = r.yes.Dump(w)
	if err != nil {
		return err
	}
	if r.no != nil {
		_, err = w.Write([]byte("} else {"))
		if err != nil {
			return err
		}
		err = r.no.Dump(w)
		if err != nil {
			return err
		}
	}
	_, err = w.Write([]byte{';'})
	return err
}

func compileIf(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// T_IF (expression) ...? else ...?

	// parse if expression
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}
	if !i.IsSingle('(') {
		return nil, i.Unexpected()
	}

	r := &runnableIf{l: i.Loc()}
	r.cond, err = compileExpr(nil, c)
	if err != nil {
		return nil, err
	}

	// check for )
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	if !i.IsSingle(')') {
		return nil, i.Unexpected()
	}

	// check for next if ':'
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	if i.IsSingle(':') {
		// parse expression until endif
		// See: http://php.net/manual/en/control-structures.alternative-syntax.php
		r.yes, err = compileBase(nil, c)
		if err != nil {
			return nil, err
		}

		i, err = c.NextItem()
		if err != nil {
			return r, err
		}

		switch i.Type {
		case tokenizer.T_ELSEIF:
			r.no, err = compileIf(nil, c)
			if err != nil {
				return nil, err
			}
		case tokenizer.T_ELSE:
			i, err = c.NextItem()
			if err != nil {
				return r, err
			}
			if !i.IsSingle(':') {
				return nil, i.Unexpected()
			}
			r.no, err = compileBase(nil, c)

			// then we should be getting a endif
			i, err = c.NextItem()
			if err != nil {
				return r, err
			}
			if i.Type != tokenizer.T_ENDIF {
				return nil, i.Unexpected()
			}
			fallthrough
		case tokenizer.T_ENDIF:
			// end of if
			i, err = c.NextItem()
			if err != nil {
				return r, err
			}
			if !i.IsSingle(';') {
				return nil, i.Unexpected()
			}
		default:
			return nil, i.Unexpected()
		}
	} else {
		c.backup()

		// parse expression normally
		r.yes, err = compileBaseSingle(nil, c)
		if err != nil {
			return nil, err
		}

		i, err = c.NextItem()
		if err != nil {
			return r, err
		}

		// check for else or elseif
		switch i.Type {
		case tokenizer.T_ELSEIF:
			r.no, err = compileIf(nil, c)
			if err != nil {
				return nil, err
			}
		case tokenizer.T_ELSE:
			// parse else
			r.no, err = compileBaseSingle(nil, c)
			if err != nil {
				return nil, err
			}
		default:
			c.backup()
		}
	}

	return r, nil
}
