package compiler

import (
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type matchArm struct {
	conditions []phpv.Runnable // nil for default arm
	body       phpv.Runnable
}

type runMatch struct {
	cond phpv.Runnable
	arms []*matchArm
	def  *matchArm // default arm, nil if none
	l    *phpv.Loc
}

func (r *runMatch) Dump(w io.Writer) error {
	_, err := w.Write([]byte("match ("))
	if err != nil {
		return err
	}
	err = r.cond.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(") { ... }"))
	return err
}

func (r *runMatch) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	cond, err := r.cond.Run(ctx)
	if err != nil {
		return nil, err
	}

	for _, arm := range r.arms {
		for _, c := range arm.conditions {
			v, err := c.Run(ctx)
			if err != nil {
				return nil, err
			}
			// match uses strict comparison (===)
			res, err := operatorCompareStrict(ctx, tokenizer.T_IS_IDENTICAL, cond, v)
			if err != nil {
				return nil, err
			}
			if res.AsBool(ctx) {
				return arm.body.Run(ctx)
			}
		}
	}

	// Try default arm
	if r.def != nil {
		return r.def.body.Run(ctx)
	}

	// No match and no default → UnhandledMatchError
	return nil, phpobj.ThrowError(ctx, phpobj.UnhandledMatchError,
		fmt.Sprintf("Unhandled match case"))
}

func (r *runMatch) Loc() *phpv.Loc {
	return r.l
}

// compileMatch compiles a match expression:
//   match (expr) {
//       cond1, cond2 => body1,
//       cond3 => body2,
//       default => body3,
//   }
func compileMatch(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	m := &runMatch{l: i.Loc()}

	// Expect (
	err := c.ExpectSingle('(')
	if err != nil {
		return nil, err
	}

	m.cond, err = compileExpr(nil, c)
	if err != nil {
		return nil, err
	}

	err = c.ExpectSingle(')')
	if err != nil {
		return nil, err
	}

	// Expect {
	err = c.ExpectSingle('{')
	if err != nil {
		return nil, err
	}

	// Parse arms
	for {
		i, err := c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle('}') {
			break
		}

		arm := &matchArm{}

		if i.Type == tokenizer.T_DEFAULT {
			// default arm
			m.def = arm
		} else {
			// Parse condition list: expr1, expr2, ... => body
			c.backup()
			for {
				cond, err := compileExpr(nil, c)
				if err != nil {
					return nil, err
				}
				arm.conditions = append(arm.conditions, cond)

				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
				if i.Type == tokenizer.T_DOUBLE_ARROW {
					break
				}
				if !i.IsSingle(',') {
					return nil, i.Unexpected()
				}
				// Check if next is => (trailing comma before =>)
				next, err := c.NextItem()
				if err != nil {
					return nil, err
				}
				if next.Type == tokenizer.T_DOUBLE_ARROW {
					break
				}
				c.backup()
			}
			m.arms = append(m.arms, arm)
		}

		if m.def == arm {
			// For default arm, expect =>
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			if i.Type != tokenizer.T_DOUBLE_ARROW {
				return nil, i.Unexpected()
			}
		}

		// Parse body expression
		arm.body, err = compileExpr(nil, c)
		if err != nil {
			return nil, err
		}

		// Expect , or }
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.IsSingle('}') {
			break
		}
		if !i.IsSingle(',') {
			return nil, i.Unexpected()
		}
	}

	return m, nil
}
