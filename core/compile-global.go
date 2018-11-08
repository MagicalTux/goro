package core

import "git.atonline.com/tristantech/gophp/core/tokenizer"

type runGlobal []ZString

func (g runGlobal) Run(ctx Context) (*ZVal, error) {
	glob := ctx.GetGlobal()
	for _, k := range g {
		v, err := glob.GetVariable(k)
		if err != nil {
			return nil, err
		}
		err = ctx.SetVariable(k, v)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func compileGlobal(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	// global $var, $var, $var, ...
	var err error

	// TODO check we are in a function/etc?

	var g runGlobal

	// parse passed arguments
	for {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.Type != tokenizer.T_VARIABLE {
			return nil, i.Unexpected()
		}

		g = append(g, ZString(i.Data[1:]))

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
