package compiler

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
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
		tokenizer.T_OPEN_TAG:            nil,
		tokenizer.T_OPEN_TAG_WITH_ECHO: &compileFuncCb{f: compileEchoTag},
		tokenizer.T_CLOSE_TAG:           nil,
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
		tokenizer.T_READONLY:     &compileFuncCb{f: compileClass, skip: true},
		tokenizer.T_INTERFACE:    &compileFuncCb{f: compileClass, skip: true},
		tokenizer.T_ENUM:         &compileFuncCb{f: compileEnum, skip: true},
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
		tokenizer.T_MATCH:        &compileFuncCb{f: compileExpr},
		tokenizer.T_NEW:          &compileFuncCb{f: compileNew},
		tokenizer.T_CLONE:        &compileFuncCb{f: compileClone},
		tokenizer.T_LIST:         &compileFuncCb{f: compileBaseDestructure},
		tokenizer.Rune('{'):      &compileFuncCb{f: compileBase, skip: true},
		tokenizer.Rune(':'):      &compileFuncCb{f: compileBaseUntilAltEnd, skip: true},
		tokenizer.Rune('('):      &compileFuncCb{f: compileExpr},
		tokenizer.Rune('['):      &compileFuncCb{f: compileExpr},
		tokenizer.Rune('@'):      &compileFuncCb{f: compileExpr},
		tokenizer.Rune('$'):      &compileFuncCb{f: compileExpr},
		tokenizer.T_NS_SEPARATOR: &compileFuncCb{f: compileExpr},
		tokenizer.T_NAMESPACE:    &compileFuncCb{f: compileNamespace, skip: true},
		tokenizer.T_USE:          &compileFuncCb{f: compileUse, skip: true},
		tokenizer.T_YIELD:        &compileFuncCb{f: compileYield},
		tokenizer.T_YIELD_FROM:   &compileFuncCb{f: compileYield},
		tokenizer.T_GOTO:         &compileFuncCb{f: compileGoto},
		tokenizer.T_CONSTANT_ENCAPSED_STRING: &compileFuncCb{f: compileExpr},
		tokenizer.Rune('"'):          &compileFuncCb{f: compileExpr},
		tokenizer.T_START_HEREDOC:    &compileFuncCb{f: compileExpr},
		tokenizer.T_LNUMBER:         &compileFuncCb{f: compileExpr},
		tokenizer.T_DNUMBER:         &compileFuncCb{f: compileExpr},
		tokenizer.T_VOID_CAST:       &compileFuncCb{f: compileExpr},
		tokenizer.T_ATTRIBUTE:       &compileFuncCb{f: compileAttributed, skip: true},
		tokenizer.T_DECLARE:         &compileFuncCb{f: compileDeclare, skip: true},
		tokenizer.T_HALT_COMPILER:   &compileFuncCb{f: compileHaltCompiler, skip: true},
		tokenizer.Rune(';'):         nil,
		// '}': return compileBase (hidden)
	}
}

// compileHaltCompiler handles __halt_compiler(); which stops compilation.
// It also sets the __COMPILER_HALT_OFFSET__ constant to the byte offset after the semicolon.
func compileHaltCompiler(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	filename := i.Filename

	// Consume remaining tokens until EOF
	for {
		tok, err := c.NextItem()
		if err != nil {
			return nil, err
		}
		if tok.Type == tokenizer.T_EOF {
			c.backup()
			break
		}
	}

	// Read the file to compute the byte offset after __halt_compiler();
	if filename != "" && filename != "-" {
		content, err := os.ReadFile(filename)
		if err == nil {
			// Find __halt_compiler in the file
			patterns := []string{"__halt_compiler();", "__HALT_COMPILER();"}
			for _, pat := range patterns {
				idx := bytes.Index(content, []byte(pat))
				if idx >= 0 {
					offset := idx + len(pat)
					c.Global().ConstantSet(phpv.ZString("__COMPILER_HALT_OFFSET__"), phpv.ZInt(offset))
					break
				}
			}
			// Also try case-insensitive search
			if _, ok := c.Global().ConstantGet(phpv.ZString("__COMPILER_HALT_OFFSET__")); !ok {
				lower := bytes.ToLower(content)
				idx := bytes.Index(lower, []byte("__halt_compiler"))
				if idx >= 0 {
					// Find the next (); after it
					rest := content[idx:]
					semiIdx := bytes.IndexByte(rest, ';')
					if semiIdx >= 0 {
						offset := idx + semiIdx + 1
						c.Global().ConstantSet(phpv.ZString("__COMPILER_HALT_OFFSET__"), phpv.ZInt(offset))
					}
				}
			}
		}
	}

	return nil, nil
}

// compileGoto handles the `goto label;` statement.
// PHP 8.5 fully reserves exit/die as keywords, so they cannot be used as goto labels.
func compileGoto(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	l := i.Loc()

	// Read the label name - must be a T_STRING identifier
	label, err := c.NextItem()
	if err != nil {
		return nil, err
	}
	if label.Type != tokenizer.T_STRING {
		return nil, label.UnexpectedExpecting("identifier")
	}

	// Note: semicolon is consumed by compileBaseSingle after this returns
	return &runGoto{label: phpv.ZString(label.Data), l: l}, nil
}

// runGoto represents a goto statement. Currently produces a runtime error
// since goto is not fully supported.
type runGoto struct {
	label phpv.ZString
	l     *phpv.Loc
}

func (r *runGoto) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	return nil, fmt.Errorf("'goto' operator is not supported")
}

func (r *runGoto) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "goto %s", r.label)
	return err
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
		// PHP 8.5: exit/die are fully reserved — cannot be used as constant names
		if i.Type == tokenizer.T_EXIT {
			return nil, i.UnexpectedExpecting("identifier")
		}
		if i.Type != tokenizer.T_STRING && !i.IsSemiReserved() {
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

		// Validate closures in constant expressions
		if zc, ok := val.(*ZClosure); ok {
			if !zc.isStatic {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("Closures in constant expressions must be static"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  zc.start,
				}
			}
			if len(zc.use) > 0 {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("Cannot use(...) variables in constant expression"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  zc.start,
				}
			}
		}

		// Check for static:: in top-level constant (compile-time error)
		if loc := checkStaticClassInConstExpr(val); loc != nil {
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("\"static::\" is not allowed in compile-time constants"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  loc,
			}
		}

		// Check for (expression)::class in constant expressions
		if loc := checkExpressionClassInConstExpr(val); loc != nil {
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("(expression)::class cannot be used in constant expressions"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  loc,
			}
		}

		// Check for dynamic class names ($var::CONST) in constant expressions
		if containsDynamicClassName(val) {
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Dynamic class names are not allowed in compile-time class constant references"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
		}

		// Check for other runtime ops in constant expressions
		if containsRuntimeOps(val) {
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Constant expression contains invalid operations"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
		}

		// Prepend current namespace to constant name
		constName := phpv.ZString(name)
		ns := c.getNamespace()
		if ns != "" {
			constName = ns + "\\" + constName
		}
		res = append(res, &runTopLevelConst{name: constName, val: val, l: i.Loc()})

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
	name  phpv.ZString
	val   phpv.Runnable
	l     *phpv.Loc
	attrs []*phpv.ZAttribute // PHP 8.0 attributes on this constant
}

func (r *runTopLevelConst) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	// Validate internal attributes on the constant before defining it
	if len(r.attrs) > 0 {
		if msg := phpobj.ValidateInternalAttributeList(ctx, r.attrs, phpobj.AttributeTARGET_CONSTANT); msg != "" {
			phpErr := &phpv.PhpError{
				Err:  fmt.Errorf("%s", msg),
				Code: phpv.E_ERROR,
				Loc:  r.l,
			}
			ctx.Global().LogError(phpErr)
			return nil, phpv.ExitError(255)
		}
	}

	v, err := r.val.Run(ctx)
	if err != nil {
		return nil, err
	}
	ok := ctx.Global().ConstantSet(r.name, v.Value())
	if !ok {
		// Constant already defined - emit a warning (will become an error in PHP 9)
		if err := ctx.Warn("Constant %s already defined, this will be an error in PHP 9", r.name, logopt.Data{NoFuncName: true, Loc: r.l}); err != nil {
			return nil, err
		}
		// Do NOT update attributes - the original constant's attributes are preserved
	} else {
		// Store attributes for reflection access (only on first definition)
		if len(r.attrs) > 0 {
			ctx.Global().ConstantSetAttributes(r.name, r.attrs)
		}
	}
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

	// Check for label definition: T_STRING followed by ':'
	// Labels are used as goto targets. We parse and discard them (they're no-ops).
	if i.Type == tokenizer.T_STRING && c.peekType() == tokenizer.Rune(':') {
		// Consume the ':'
		c.NextItem()
		// Label is a no-op — just compile the next statement
		return compileBaseSingle(nil, c)
	}

	// Special case: T_READONLY followed by '(' is a function call, not a class modifier.
	if i.Type == tokenizer.T_READONLY && c.peekType() == tokenizer.Rune('(') {
		h = &compileFuncCb{f: compileExpr}
		ok = true
	}

	// is it a single char item?
	if !ok {
		h, ok = itemTypeHandler[i.Type]
	}
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
		// If the next token is an operator, continue as expression
		// This handles cases like `new d + new d;`
		if isOperator(i.Type) {
			c.backup()
			for {
				sr, err := compilePostExpr(r, nil, c)
				if err != nil {
					return nil, err
				}
				if sr == nil {
					break
				}
				r = sr
			}
			// Now expect ';'
			i, err = c.NextItem()
			if err != nil {
				return r, err
			}
			if !i.IsExpressionEnd() {
				return nil, i.Unexpected()
			}
		} else {
			// expecting a ';' after a var
			return nil, i.Unexpected()
		}
	}

	// Wrap standalone `new Foo()` expressions so temporary objects get
	// destroyed at statement end (matching PHP's refcount behavior).
	if _, isNew := r.(*runNewObject); isNew {
		r = &runDestroyTemporary{inner: r}
	}

	if _, isFuncCall := r.(phpv.FuncCallExpression); isFuncCall {
		r = &runNoDiscardStatement{inner: r}
	}

	return r, nil
}

// runDestroyTemporary wraps a statement expression (typically `new Foo()`)
// and calls __destruct on the resulting object at statement end. This
// simulates PHP's refcount-based destruction of temporary objects.
type runDestroyTemporary struct {
	inner phpv.Runnable
}

func (r *runDestroyTemporary) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	result, err := r.inner.Run(ctx)
	if err != nil {
		return result, err
	}
	// If the result is an object with a destructor, destroy it immediately.
	if result != nil && result.GetType() == phpv.ZtObject {
		if obj, ok := result.Value().(phpv.ZObject); ok {
			if _, hasDestructor := obj.GetClass().GetMethod("__destruct"); hasDestructor {
				if destructable, ok2 := obj.(interface {
					CallImplicitDestructor(phpv.Context) error
				}); ok2 {
					if derr := destructable.CallImplicitDestructor(ctx); derr != nil {
						return nil, derr
					}
				}
			}
		}
	}
	return result, nil
}

func (r *runDestroyTemporary) Dump(w io.Writer) error {
	return r.inner.Dump(w)
}

// compileEchoTag handles <?= expr ?> which is equivalent to echo expr;
func compileEchoTag(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	l := i.Loc()

	// Parse expressions as arguments to echo, same as compileSpecialFuncCall
	var args []phpv.Runnable

	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	for {
		var a phpv.Runnable
		a, err = compileExpr(i, c)
		if err != nil {
			return nil, err
		}

		args = append(args, a)

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(',') {
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			continue
		}
		if i.IsExpressionEnd() {
			c.backup()
			return &runnableFunctionCall{name: "echo", args: args, l: l}, nil
		}

		return nil, i.Unexpected()
	}
}
