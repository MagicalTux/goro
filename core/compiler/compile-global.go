package compiler

import (
	"io"

	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type globalVar struct {
	static  phpv.ZString  // for static variable names like $foo
	dynamic phpv.Runnable // for variable-variables like $$foo
}

type runGlobal struct {
	vars []globalVar
	l    *phpv.Loc
}

func (g *runGlobal) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	var err error
	var v *phpv.ZVal

	err = ctx.Tick(ctx, g.l)
	if err != nil {
		return nil, err
	}

	glob := ctx.Global()
	for _, gv := range g.vars {
		var k phpv.ZString
		if gv.dynamic != nil {
			z, err := gv.dynamic.Run(ctx)
			if err != nil {
				return nil, err
			}
			k = phpv.ZString(z.String())
		} else {
			k = gv.static
		}

		if ok, _ := glob.OffsetExists(ctx, k.ZVal()); !ok {
			// need to create it
			v = phpv.ZNull{}.ZVal()
			glob.OffsetSet(ctx, k.ZVal(), v)
		} else {
			v, err = glob.OffsetGet(ctx, k.ZVal())
			if err != nil {
				return nil, err
			}
		}

		err = ctx.OffsetSet(ctx, k.ZVal(), v)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (g *runGlobal) Dump(w io.Writer) error {
	_, err := w.Write([]byte("global "))
	first := true
	for _, gv := range g.vars {
		if !first {
			_, err = w.Write([]byte{','})
			if err != nil {
				return err
			}
		}
		first = false
		if gv.dynamic != nil {
			_, err = w.Write([]byte("$$"))
			if err != nil {
				return err
			}
			err = gv.dynamic.Dump(w)
		} else {
			_, err = w.Write([]byte{'$'})
			if err != nil {
				return err
			}
			_, err = w.Write([]byte(gv.static))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func compileGlobal(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// global $var, $var, $var, ...
	var err error

	// TODO check we are in a function/etc?

	g := &runGlobal{l: i.Loc()}

	// parse passed arguments
	for {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.Type == tokenizer.T_VARIABLE {
			g.vars = append(g.vars, globalVar{static: phpv.ZString(i.Data[1:])})
		} else if i.IsSingle('$') {
			// variable-variable: global $$k
			expr, err := compileOneExpr(nil, c)
			if err != nil {
				return nil, err
			}
			g.vars = append(g.vars, globalVar{dynamic: expr})
		} else {
			return nil, i.Unexpected()
		}

		i, err = c.NextItem()

		if i.IsSingle(',') {
			continue
		}

		if i.IsSingle(';') {
			c.backup()
			return g, nil
		}

		return nil, i.Unexpected()
	}
}
