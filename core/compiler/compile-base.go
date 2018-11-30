package compiler

import (
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type compileFunc func(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error)

type compileFuncCb struct {
	f    compileFunc
	skip bool
}

var itemTypeHandler map[tokenizer.ItemType]*compileFuncCb

func init() {
	itemTypeHandler = map[tokenizer.ItemType]*compileFuncCb{
		tokenizer.T_OPEN_TAG:     nil,
		tokenizer.T_CLOSE_TAG:    nil,
		tokenizer.T_DOC_COMMENT:  nil, // TODO
		tokenizer.T_INLINE_HTML:  &compileFuncCb{f: compileInlineHtml, skip: true},
		tokenizer.T_FUNCTION:     &compileFuncCb{f: compileFunction, skip: true},
		tokenizer.T_WHILE:        &compileFuncCb{f: compileWhile, skip: true},
		tokenizer.T_DO:           &compileFuncCb{f: compileDoWhile},
		tokenizer.T_FOR:          &compileFuncCb{f: compileFor, skip: true},
		tokenizer.T_FOREACH:      &compileFuncCb{f: compileForeach, skip: true},
		tokenizer.T_SWITCH:       &compileFuncCb{f: compileSwitch, skip: true},
		tokenizer.T_IF:           &compileFuncCb{f: compileIf, skip: true},
		tokenizer.T_CLASS:        &compileFuncCb{f: compileClass, skip: true},
		tokenizer.T_INTERFACE:    &compileFuncCb{f: compileClass, skip: true},
		tokenizer.T_STATIC:       &compileFuncCb{f: compileStaticVar},
		tokenizer.T_RETURN:       &compileFuncCb{f: compileReturn},
		tokenizer.T_VARIABLE:     &compileFuncCb{f: compileExpr},
		tokenizer.T_ECHO:         &compileFuncCb{f: compileSpecialFuncCall},
		tokenizer.T_PRINT:        &compileFuncCb{f: compileSpecialFuncCall},
		tokenizer.T_EXIT:         &compileFuncCb{f: compileSpecialFuncCall},
		tokenizer.T_ISSET:        &compileFuncCb{f: compileSpecialFuncCall},
		tokenizer.T_UNSET:        &compileFuncCb{f: compileUnset},
		tokenizer.T_THROW:        &compileFuncCb{f: compileThrow},
		tokenizer.T_EMPTY:        &compileFuncCb{f: compileSpecialFuncCall},
		tokenizer.T_EVAL:         &compileFuncCb{f: compileSpecialFuncCall},
		tokenizer.T_INCLUDE:      &compileFuncCb{f: compileSpecialFuncCall},
		tokenizer.T_REQUIRE:      &compileFuncCb{f: compileSpecialFuncCall},
		tokenizer.T_INCLUDE_ONCE: &compileFuncCb{f: compileSpecialFuncCall},
		tokenizer.T_REQUIRE_ONCE: &compileFuncCb{f: compileSpecialFuncCall},
		tokenizer.T_GLOBAL:       &compileFuncCb{f: compileGlobal},
		tokenizer.T_STRING:       &compileFuncCb{f: compileExpr},
		tokenizer.T_CONTINUE:     &compileFuncCb{f: compileContinue},
		tokenizer.T_BREAK:        &compileFuncCb{f: compileBreak},
		tokenizer.T_NEW:          &compileFuncCb{f: compileNew},
		tokenizer.Rune('{'):      &compileFuncCb{f: compileBase, skip: true},
		tokenizer.Rune('('):      &compileFuncCb{f: compileExpr},
		tokenizer.Rune('@'):      &compileFuncCb{f: compileExpr},
		tokenizer.Rune('$'):      &compileFuncCb{f: compileExpr},
		tokenizer.Rune(';'):      nil,
		// '}': return compileBase (hidden)
	}
}

// compileIgnore will ignore a given token
func compileIgnore(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	return nil, nil
}

func compileBase(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	return compileBaseUntil(i, c, tokenizer.Rune('}'))
}

func compileBaseUntil(i *tokenizer.Item, c compileCtx, until tokenizer.ItemType) (phpv.Runnable, error) {
	var res phpv.Runnables

	for {
		i, err := c.NextItem()
		if err != nil {
			return res, err
		}
		if i.Type == until {
			return res, nil
		}
		switch i.Type {
		case tokenizer.T_ENDIF, tokenizer.T_ELSE, tokenizer.T_ELSEIF:
			// end of block, but need to backup one for caller to check
			c.backup()
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
}

func compileBaseSingle(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
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
	h, ok = itemTypeHandler[i.Type]
	if !ok {
		_, ok = operatorList[i.Type]
		if ok {
			h = &compileFuncCb{f: compileExpr}
		} else {
			return nil, i.Unexpected()
		}
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

	if !i.IsExpressionEnd() {
		// expecting a ';' after a var
		return nil, i.Unexpected()
	}
	return r, nil
}
