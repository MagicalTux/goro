package core

import (
	"io"

	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runGlobal struct {
	vars []phpv.ZString
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
	for _, k := range g.vars {
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
	for _, v := range g.vars {
		if !first {
			_, err = w.Write([]byte{','})
			if err != nil {
				return err
			}
		}
		first = false
		_, err = w.Write([]byte{'$'})
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(v))
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

	g := &runGlobal{l: phpv.MakeLoc(i.Loc())}

	// parse passed arguments
	for {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.Type != tokenizer.T_VARIABLE {
			return nil, i.Unexpected()
		}

		g.vars = append(g.vars, phpv.ZString(i.Data[1:]))

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
