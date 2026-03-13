package compiler

import (
	"fmt"
	"io"

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
		tokenizer.T_FN:           &compileFuncCb{f: compileArrowFunction, skip: true},
		tokenizer.T_WHILE:        &compileFuncCb{f: compileWhile, skip: true},
		tokenizer.T_DO:           &compileFuncCb{f: compileDoWhile},
		tokenizer.T_FOR:          &compileFuncCb{f: compileFor, skip: true},
		tokenizer.T_FOREACH:      &compileFuncCb{f: compileForeach, skip: true},
		tokenizer.T_SWITCH:       &compileFuncCb{f: compileSwitch, skip: true},
		tokenizer.T_IF:           &compileFuncCb{f: compileIf, skip: true},
		tokenizer.T_CLASS:        &compileFuncCb{f: compileClass, skip: true},
		tokenizer.T_TRAIT:        &compileFuncCb{f: compileClass, skip: true},
		tokenizer.T_ABSTRACT:     &compileFuncCb{f: compileClass, skip: true},
		tokenizer.T_FINAL:        &compileFuncCb{f: compileClass, skip: true},
		tokenizer.T_INTERFACE:    &compileFuncCb{f: compileClass, skip: true},
		tokenizer.T_TRY:          &compileFuncCb{f: compileTry, skip: true},
		tokenizer.T_CONST:        &compileFuncCb{f: compileTopLevelConst, skip: true},
		tokenizer.T_STATIC:       &compileFuncCb{f: compileStaticVar},
		tokenizer.T_RETURN:       &compileFuncCb{f: compileReturn},
		tokenizer.T_VARIABLE:     &compileFuncCb{f: compileExpr},
		tokenizer.T_ECHO:         &compileFuncCb{f: compileSpecialFuncCall},
		tokenizer.T_PRINT:        &compileFuncCb{f: compileSpecialFuncCall},
		tokenizer.T_EXIT:         &compileFuncCb{f: compileSpecialFuncCall},
		tokenizer.T_ISSET:        &compileFuncCb{f: compileIsset},
		tokenizer.T_UNSET:        &compileFuncCb{f: compileUnset},
		tokenizer.T_THROW:        &compileFuncCb{f: compileThrow},
		tokenizer.T_EMPTY:        &compileFuncCb{f: compileEmpty},
		tokenizer.T_EVAL:         &compileFuncCb{f: compileSpecialFuncCallOne},
		tokenizer.T_INCLUDE:      &compileFuncCb{f: compileSpecialFuncCall},
		tokenizer.T_REQUIRE:      &compileFuncCb{f: compileSpecialFuncCall},
		tokenizer.T_INCLUDE_ONCE: &compileFuncCb{f: compileSpecialFuncCall},
		tokenizer.T_REQUIRE_ONCE: &compileFuncCb{f: compileSpecialFuncCall},
		tokenizer.T_GLOBAL:       &compileFuncCb{f: compileGlobal},
		tokenizer.T_STRING:       &compileFuncCb{f: compileExpr},
		tokenizer.T_CONTINUE:     &compileFuncCb{f: compileContinue},
		tokenizer.T_BREAK:        &compileFuncCb{f: compileBreak},
		tokenizer.T_NEW:          &compileFuncCb{f: compileNew},
		tokenizer.T_CLONE:        &compileFuncCb{f: compileClone},
		tokenizer.T_LIST:         &compileFuncCb{f: compileBaseDestructure},
		tokenizer.Rune('{'):      &compileFuncCb{f: compileBase, skip: true},
		tokenizer.Rune(':'):      &compileFuncCb{f: compileBaseUntilAltEnd, skip: true},
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

// handle the blocks delimited with alternate syntax
// https://www.php.net/manual/en/control-structures.alternative-syntax.php
func compileBaseUntilAltEnd(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	var res phpv.Runnables

	for {
		i, err := c.NextItem()
		if err != nil {
			return res, err
		}
		switch i.Type {
		case tokenizer.T_ENDFOR, tokenizer.T_ENDFOREACH, tokenizer.T_ENDWHILE:
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

func compileBaseUntil(i *tokenizer.Item, c compileCtx, until tokenizer.ItemType) (phpv.Runnable, error) {
	var res phpv.Runnables
	var declaredStaticVars map[phpv.ZString]*phpv.Loc

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
			// Check for duplicate static variable declarations
			if sv, ok := t.(*runStaticVar); ok {
				if declaredStaticVars == nil {
					declaredStaticVars = make(map[phpv.ZString]*phpv.Loc)
				}
				for _, v := range sv.vars {
					if _, exists := declaredStaticVars[v.varName]; exists {
						return nil, &phpv.PhpError{
							Err:  fmt.Errorf("Duplicate declaration of static variable $%s", v.varName),
							Code: phpv.E_COMPILE_ERROR,
							Loc:  sv.l,
						}
					}
					declaredStaticVars[v.varName] = sv.l
				}
			}
			res = append(res, t)
		}
		if err != nil {
			return res, err
		}
	}
}

// compileTopLevelConst handles: const NAME = expr [, NAME2 = expr2] ;
func compileTopLevelConst(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	var res phpv.Runnables

	for {
		// Read constant name
		i, err := c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.Type != tokenizer.T_STRING {
			return nil, i.Unexpected()
		}
		name := i.Data

		// Read '='
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if !i.IsSingle('=') {
			return nil, i.Unexpected()
		}

		// Compile value expression
		val, err := compileExpr(nil, c)
		if err != nil {
			return nil, err
		}

		res = append(res, &runTopLevelConst{name: phpv.ZString(name), val: val, l: i.Loc()})

		// Check for ',' (more constants) or ';' (end)
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.IsSingle(';') || i.IsExpressionEnd() {
			break
		}
		if !i.IsSingle(',') {
			return nil, i.Unexpected()
		}
	}

	if len(res) == 1 {
		return res[0], nil
	}
	return res, nil
}

type runTopLevelConst struct {
	name phpv.ZString
	val  phpv.Runnable
	l    *phpv.Loc
}

func (r *runTopLevelConst) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	v, err := r.val.Run(ctx)
	if err != nil {
		return nil, err
	}
	ctx.Global().ConstantSet(r.name, v.Value())
	return nil, nil
}

func (r *runTopLevelConst) Dump(w io.Writer) error {
	return nil
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
