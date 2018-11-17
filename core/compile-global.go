package core

import (
	"io"

	"github.com/MagicalTux/gophp/core/tokenizer"
)

type runGlobal struct {
	vars []ZString
	l    *Loc
}

func (g *runGlobal) Run(ctx Context) (*ZVal, error) {
	glob := ctx.GetGlobal()
	for _, k := range g.vars {
		v, err := glob.OffsetGet(ctx, k.ZVal())
		if err != nil {
			return nil, err
		}
		err = ctx.OffsetSet(ctx, k.ZVal(), v)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (g *runGlobal) Loc() *Loc {
	return g.l
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

func compileGlobal(i *tokenizer.Item, c compileCtx) (Runnable, error) {
	// global $var, $var, $var, ...
	var err error

	// TODO check we are in a function/etc?

	g := &runGlobal{l: MakeLoc(i.Loc())}

	// parse passed arguments
	for {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.Type != tokenizer.T_VARIABLE {
			return nil, i.Unexpected()
		}

		g.vars = append(g.vars, ZString(i.Data[1:]))

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
