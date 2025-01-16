package compiler

import (
	"io"

	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

// TODO find ways to optimize switch

type runSwitchBlock struct {
	cond phpv.Runnable // condition for run (nil = default)
	code phpv.Runnables
	l    *phpv.Loc
}

type runSwitch struct {
	blocks []*runSwitchBlock
	def    *runSwitchBlock
	cond   phpv.Runnable
	l      *phpv.Loc
}

func (r runSwitch) Dump(w io.Writer) error {
	_, err := w.Write([]byte("switch ("))
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

	for _, c := range r.blocks {
		if c.cond == nil {
			_, err = w.Write([]byte("default:"))
			if err != nil {
				return err
			}
		} else {
			_, err = w.Write([]byte("case "))
			if err != nil {
				return err
			}
			err = c.cond.Dump(w)
			if err != nil {
				return err
			}
			_, err = w.Write([]byte{':'})
			if err != nil {
				return err
			}
		}
		err = c.code.Dump(w)
		if err != nil {
			return err
		}
	}
	_, err = w.Write([]byte{'}'})
	return err
}

func (r *runSwitch) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	cond, err := r.cond.Run(ctx)
	if err != nil {
		return nil, err
	}

	run := false

	for _, bl := range r.blocks {
		if !run {
			// check cond (if nil, this is a default option)
			if bl.cond != nil {
				z, err := bl.cond.Run(ctx)
				if err != nil {
					return nil, err
				}

				v, err := operatorCompare(ctx, tokenizer.T_IS_EQUAL, cond, z)
				if err != nil {
					return nil, err
				}
				if !v.AsBool(ctx) {
					continue
				}
			}
			run = true
		}

		_, err = bl.code.Run(ctx)
		if err != nil {
			e := r.l.Error(err)
			err = e
			switch br := e.Err.(type) {
			case *phperr.PhpBreak:
				if br.Intv > 1 {
					br.Intv -= 1
					return nil, br
				}
			case *phperr.PhpContinue:
				if br.Intv > 1 {
					br.Intv -= 1
					return nil, br
				}
			default:
				return nil, err
			}
			break
		}
	}

	return nil, nil
}

func compileSwitch(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	sw := &runSwitch{l: i.Loc()}

	// we expect a {
	err := c.ExpectSingle('(')
	if err != nil {
		return nil, err
	}

	sw.cond, err = compileExpr(nil, c)
	if err != nil {
		return nil, err
	}
	err = c.ExpectSingle(')')
	if err != nil {
		return nil, err
	}

	altForm := false

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	switch i.Type {
	case tokenizer.Rune('{'):
	case tokenizer.Rune(':'):
		altForm = true
	default:
		c.backup()
		return nil, i.Unexpected()
	}

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	for {

		if (altForm && i.Type == tokenizer.T_ENDSWITCH) || (!altForm && i.IsSingle('}')) {
			break
		}

		bl := &runSwitchBlock{}
		sw.blocks = append(sw.blocks, bl)

		switch i.Type {
		case tokenizer.T_CASE:
			bl.cond, err = compileExpr(nil, c)
			if err != nil {
				return nil, err
			}
		case tokenizer.T_DEFAULT:
		default:
			return sw, i.Unexpected()
		}

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if !i.IsSingle(':') && !i.IsSingle(';') {
			return nil, i.Unexpected()
		}

		// parse case code
		for {
			i, err = c.NextItem()
			if err != nil {
				return sw, err
			}

			if (altForm && i.Type == tokenizer.T_ENDSWITCH) || (!altForm && i.IsSingle('}')) {
				break
			}

			if i.Type == tokenizer.T_CASE || i.Type == tokenizer.T_DEFAULT {
				break
			}

			t, err := compileBaseSingle(i, c)
			if t != nil {
				bl.code = append(bl.code, t)
			}
			if err != nil {
				return sw, err
			}
		}
	}

	return sw, nil
}
