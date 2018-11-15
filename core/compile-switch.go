package core

import "github.com/MagicalTux/gophp/core/tokenizer"

// TODO find ways to optimize switch

type runSwitchBlock struct {
	cond Runnable // condition for run (nil = default)
	code Runnables
	l    *Loc
}

type runSwitch struct {
	blocks []*runSwitchBlock
	def    *runSwitchBlock
	cond   Runnable
	l      *Loc
}

func (r *runSwitch) Loc() *Loc {
	return r.l
}

func (r *runSwitch) Run(ctx Context) (*ZVal, error) {
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

				v, err := operatorCompare(ctx, "==", cond, z)
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
			if e.t == PhpBreak || e.t == PhpContinue {
				break
			}
			return nil, err
		}
	}

	return nil, nil
}

func compileSwitch(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	sw := &runSwitch{l: MakeLoc(i.Loc())}

	// we expect a {
	err := c.ExpectSingle('(')
	if err != nil {
		return nil, err
	}

	sw.cond, err = compileExpr(nil, c)
	err = c.ExpectSingle(')')
	if err != nil {
		return nil, err
	}
	err = c.ExpectSingle('{')
	if err != nil {
		return nil, err
	}

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	for {
		if i.IsSingle('}') {
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
			if i.IsSingle('}') {
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
