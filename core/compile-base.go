package core

import "git.atonline.com/tristantech/gophp/core/tokenizer"

type compileFunc func(i *tokenizer.Item, c *compileCtx) (Runnable, error)

type compileFuncCb struct {
	f    compileFunc
	skip bool
}

var itemTypeHandler map[tokenizer.ItemType]*compileFuncCb
var itemSingleHandler map[rune]*compileFuncCb

func init() {
	itemTypeHandler = map[tokenizer.ItemType]*compileFuncCb{
		tokenizer.T_OPEN_TAG:    nil,
		tokenizer.T_CLOSE_TAG:   nil,
		tokenizer.T_INLINE_HTML: &compileFuncCb{f: compileInlineHtml, skip: true},
		tokenizer.T_FUNCTION:    &compileFuncCb{f: compileFunction, skip: true},
		tokenizer.T_WHILE:       &compileFuncCb{f: compileWhile, skip: true},
		tokenizer.T_FOREACH:     &compileFuncCb{f: compileForeach, skip: true},
		tokenizer.T_IF:          &compileFuncCb{f: compileIf, skip: true},
		tokenizer.T_RETURN:      &compileFuncCb{f: compileReturn},
		tokenizer.T_VARIABLE:    &compileFuncCb{f: compileExpr},
		tokenizer.T_ECHO:        &compileFuncCb{f: compileSpecialFuncCall},
		tokenizer.T_EXIT:        &compileFuncCb{f: compileSpecialFuncCall},
		tokenizer.T_ISSET:       &compileFuncCb{f: compileSpecialFuncCall},
		tokenizer.T_EMPTY:       &compileFuncCb{f: compileSpecialFuncCall},
		tokenizer.T_EVAL:        &compileFuncCb{f: compileSpecialFuncCall},
		tokenizer.T_GLOBAL:      &compileFuncCb{f: compileGlobal},
		tokenizer.T_STRING:      &compileFuncCb{f: compileExpr},
	}

	itemSingleHandler = map[rune]*compileFuncCb{
		'{': &compileFuncCb{f: compileBase, skip: true},
		'(': &compileFuncCb{f: compileExpr, skip: true},
		'@': &compileFuncCb{f: compileExpr},
		';': nil,
		//'}': return compileBase (hidden)
	}
}

// compileIgnore will ignore a given token
func compileIgnore(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	return nil, nil
}

func compileBase(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	var res Runnables

	for {
		i, err := c.NextItem()
		if err != nil {
			return res, err
		}
		if i.IsSingle('}') {
			return res, nil
		}

		t, err := compileBaseSingle(i, c)
		if t != nil {
			res = append(res, t)
		}
		if err != nil {
			return res, err
		}
	}
	return res, nil
}

func compileBaseSingle(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	//log.Printf("compileBase: %s:%d %s %q", i.Filename, i.Line, i.Type, i.Data)
	var h *compileFuncCb
	var ok bool

	if i == nil {
		var err error
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	// is it a single char item?
	if i.Type == tokenizer.ItemSingleChar {
		ch := []rune(i.Data)[0]
		h, ok = itemSingleHandler[ch]
	} else {
		// is it a token?
		h, ok = itemTypeHandler[i.Type]
	}
	if !ok {
		return nil, i.Unexpected()
	}
	if h == nil {
		// ignore this tag
		return nil, nil
	}

	r, err := h.f(i, c)
	if err != nil {
		return nil, err
	}

	if h.skip {
		return r, nil
	}

	// check for ';'
	i, err = c.NextItem()
	if err != nil {
		return r, err
	}

	if !i.IsSingle(';') {
		// expecting a ';' after a var
		return nil, i.Unexpected()
	}
	return r, nil
}

func compileReturn(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	i, err := c.NextItem()
	c.backup()
	if err != nil {
		return nil, err
	}

	l := MakeLoc(i.Loc())

	if i.IsSingle(';') {
		return &runReturn{}, i.Unexpected()
	}

	v, err := compileExpr(nil, c)
	if err != nil {
		return nil, err
	}

	return &runReturn{v, l}, nil
}

type runReturn struct {
	v Runnable
	l *Loc
}

func (r *runReturn) Run(ctx Context) (*ZVal, error) {
	return r.v.Run(ctx) // TODO
}

func (r *runReturn) Loc() *Loc {
	return r.l
}
