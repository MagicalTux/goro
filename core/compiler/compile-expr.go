package compiler

import (
	"fmt"
	"io"
	"math"
	"math/big"
	"path"
	"strconv"
	"strings"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

// parseBigLiteral parses a binary, hex, or octal literal that overflows uint64
// into a float64 value, matching PHP's behavior.
func parseBigLiteral(s string) float64 {
	var bi big.Int
	_, ok := bi.SetString(s, 0)
	if !ok {
		return math.Inf(1)
	}
	f, _ := new(big.Float).SetInt(&bi).Float64()
	return f
}

// an expression is:

// $a_variable
// "a string"
// "a string with a $var"
// $a + $b
// etc...

func compileExpr(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	res, err := compileOneExpr(i, c)
	if err != nil {
		return nil, err
	}

	for {
		sr, err := compilePostExpr(res, nil, c)
		if err != nil {
			return nil, err
		}
		if sr == nil {
			return res, nil
		}
		res = sr
	}
}

func compileOpExpr(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	res, err := compileOneExpr(i, c)
	if err != nil {
		return nil, err
	}

	for {
		pt := c.peekType()
		// Allow postfix ++ and -- to be consumed as part of the operand,
		// and also function calls, array access, object operators, and ::
		if isOperator(pt) && !isRightAssociative(pt) &&
			pt != tokenizer.T_INC && pt != tokenizer.T_DEC {
			return res, nil
		}
		sr, err := compilePostExpr(res, nil, c)
		if err != nil {
			return nil, err
		}
		if sr == nil {
			return res, nil
		}
		res = sr
	}
}

func compileOneExpr(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// fetch only one expression, without any operator or anything
	var err error

	if i == nil {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	l := i.Loc()

	switch i.Type {
	case tokenizer.T_STATIC:
		// "static" can appear as an expression when followed by :: (e.g. static::class, static::method())
		next, err := c.NextItem()
		if err != nil {
			return nil, err
		}
		c.backup()
		if next.Type == tokenizer.T_PAAMAYIM_NEKUDOTAYIM {
			return &runZVal{phpv.ZString("static"), l}, nil
		}
		// Not followed by :: — fall through to default handler (e.g. static $var, static function)
		h, ok := itemTypeHandler[i.Type]
		if ok && h != nil {
			return h.f(i, c)
		}
		return nil, i.Unexpected()
	case tokenizer.T_VARIABLE:
		return &runVariable{v: phpv.ZString(i.Data[1:]), l: l}, nil
	case tokenizer.Rune('$'):
		return compileRunVariableRef(nil, c, l)
	case tokenizer.T_LNUMBER:
		v, err := strconv.ParseInt(i.Data, 0, 64)
		if err == nil {
			return &runZVal{phpv.ZInt(v), l}, nil
		}
		// if ParseInt failed, try to parse as float (value too large?)
		// For binary/hex/octal with prefix, ParseFloat doesn't work, so use ParseUint then convert
		if len(i.Data) > 2 && i.Data[0] == '0' && (i.Data[1] == 'b' || i.Data[1] == 'B' ||
			i.Data[1] == 'x' || i.Data[1] == 'X' ||
			i.Data[1] == 'o' || i.Data[1] == 'O') {
			uv, uerr := strconv.ParseUint(i.Data, 0, 64)
			if uerr == nil {
				// PHP treats values > INT64_MAX as float, not as two's complement int.
				// Only values that fit in signed int64 remain int.
				if uv <= math.MaxInt64 {
					return &runZVal{phpv.ZInt(int64(uv)), l}, nil
				}
				return &runZVal{phpv.ZFloat(float64(uv)), l}, nil
			}
			// truly huge: parse manually for float approximation
			f := parseBigLiteral(i.Data)
			return &runZVal{phpv.ZFloat(phpv.ZFloat(f)), l}, nil
		}
		// Try octal overflow (0...)
		if len(i.Data) > 1 && i.Data[0] == '0' && i.Data[1] >= '0' && i.Data[1] <= '7' {
			uv, uerr := strconv.ParseUint(i.Data, 0, 64)
			if uerr == nil {
				return &runZVal{phpv.ZFloat(float64(uv)), l}, nil
			}
			f := parseBigLiteral(i.Data)
			return &runZVal{phpv.ZFloat(phpv.ZFloat(f)), l}, nil
		}
		fallthrough
	case tokenizer.T_DNUMBER:
		v, err := strconv.ParseFloat(i.Data, 64)
		if err != nil {
			errv := err.(*strconv.NumError)
			if errv.Err == strconv.ErrRange {
				// v is inf
				return &runZVal{phpv.ZFloat(v), l}, nil
			}
			return nil, err
		}
		return &runZVal{phpv.ZFloat(v), l}, nil
	case tokenizer.T_NAMESPACE:
		// namespace\Name — relative to the current namespace.
		next, err := c.NextItem()
		if err != nil {
			return nil, err
		}
		if next.Type != tokenizer.T_NS_SEPARATOR {
			return nil, next.Unexpected()
		}
		// Now read the name part(s) after the backslash
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if !i.IsSemiReserved() {
			return nil, i.Unexpected()
		}
		name := i.Data
		for {
			next, err := c.NextItem()
			if err != nil {
				return nil, err
			}
			if next.Type == tokenizer.T_NS_SEPARATOR {
				part, err := c.NextItem()
				if err != nil {
					return nil, err
				}
				if !part.IsSemiReserved() {
					return nil, part.Unexpected()
				}
				name += "\\" + part.Data
			} else {
				c.backup()
				break
			}
		}
		// Resolve relative to current namespace
		ns := c.getNamespace()
		if ns != "" {
			name = string(ns) + "\\" + name
		}
		// Check if followed by :: (class reference) or ( (function call)
		next, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		c.backup()
		if next.Type == tokenizer.T_PAAMAYIM_NEKUDOTAYIM {
			return &runZVal{phpv.ZString(name), l}, nil
		}
		// namespace\ prefix means explicitly qualified — no global fallback
		return &runConstant{c: name, l: l, noFallback: true}, nil
	case tokenizer.T_NS_SEPARATOR:
		// Fully-qualified name like \TypeError or \PHP_EOL
		// PHP 8.5: \clone($obj) is a function-call syntax for clone
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.Type == tokenizer.T_CLONE {
			// \clone(...) — PHP 8.5 clone as a function
			return compileClone(i, c)
		}
		if !i.IsSemiReserved() {
			return nil, i.Unexpected()
		}
		// Build the full name (consume any further \Parts)
		name := i.Data
		for {
			next, err := c.NextItem()
			if err != nil {
				return nil, err
			}
			if next.Type == tokenizer.T_NS_SEPARATOR {
				part, err := c.NextItem()
				if err != nil {
					return nil, err
				}
				if !part.IsSemiReserved() {
					return nil, part.Unexpected()
				}
				name += "\\" + part.Data
			} else {
				c.backup()
				break
			}
		}
		// Fully qualified — use as-is (already stripped leading \)
		// Check if followed by :: (static access) or ( (function call)
		next, err := c.NextItem()
		if err != nil {
			return nil, err
		}
		c.backup()
		if next.Type == tokenizer.T_PAAMAYIM_NEKUDOTAYIM {
			return &runZVal{phpv.ZString(name), l}, nil
		}
		// Fully qualified name — no global fallback
		return &runConstant{c: name, l: l, noFallback: true}, nil
	case tokenizer.T_ENUM:
		// `enum` used as identifier in expression context — treat as T_STRING
		i.Data = "enum"
		fallthrough
	case tokenizer.T_READONLY:
		if i.Type == tokenizer.T_READONLY {
			// `readonly` used as identifier in expression context — treat as T_STRING
			i.Data = "readonly"
		}
		fallthrough
	case tokenizer.T_FN:
		if i.Type == tokenizer.T_FN {
			// `fn` used as identifier in expression context (e.g. fn\test()) — treat as T_STRING
			// Only when followed by T_NS_SEPARATOR; otherwise fall through to default handler
			next, err := c.NextItem()
			if err != nil {
				return nil, err
			}
			c.backup()
			if next.Type != tokenizer.T_NS_SEPARATOR {
				// Not a namespace-qualified name — handle as arrow function
				h, ok := itemTypeHandler[i.Type]
				if ok && h != nil {
					return h.f(i, c)
				}
				return nil, i.Unexpected()
			}
			i.Data = "fn"
		}
		fallthrough
	case tokenizer.T_STRING:
		// Check for qualified names: T_STRING followed by T_NS_SEPARATOR
		name := i.Data
		for {
			peek, err := c.NextItem()
			if err != nil {
				return nil, err
			}
			if peek.Type == tokenizer.T_NS_SEPARATOR {
				part, err := c.NextItem()
				if err != nil {
					return nil, err
				}
				if !part.IsSemiReserved() {
					return nil, part.Unexpected()
				}
				name += "\\" + part.Data
			} else {
				c.backup()
				break
			}
		}
		// Peek ahead: if followed by ::, this is a class name
		// if followed by (, this is a function call
		// otherwise, it's a constant
		next, err := c.NextItem()
		if err != nil {
			return nil, err
		}
		c.backup()
		if next.Type == tokenizer.T_PAAMAYIM_NEKUDOTAYIM {
			resolved := c.resolveClassName(phpv.ZString(name))
			return &runZVal{resolved, l}, nil
		}
		if next.IsSingle('(') {
			// Will be resolved as function name in compilePostExpr
			resolved := c.resolveFunctionName(phpv.ZString(name))
			return &runConstant{c: string(resolved), l: l}, nil
		}
		// Constant: resolve through namespace
		resolved := c.resolveConstantName(name)
		return &runConstant{c: resolved, l: l}, nil

	case tokenizer.T_CONSTANT_ENCAPSED_STRING:
		return compileQuoteConstant(i, c)
	case tokenizer.T_START_HEREDOC:
		return compileQuoteHeredoc(i, c)
	case tokenizer.T_ARRAY:
		return compileArray(i, c)
	case tokenizer.T_MATCH:
		return compileMatch(i, c)
	case tokenizer.T_EXIT:
		return compileExitExpr(i, c)
	case tokenizer.T_THROW:
		return compileThrow(i, c)
	case tokenizer.T_INCLUDE, tokenizer.T_REQUIRE, tokenizer.T_INCLUDE_ONCE, tokenizer.T_REQUIRE_ONCE:
		return compileSpecialFuncCall(i, c)
	case tokenizer.T_FILE:
		return &runZVal{phpv.ZString(l.Filename), l}, nil
	case tokenizer.T_LINE:
		return &runZVal{phpv.ZInt(l.Line), l}, nil
	case tokenizer.T_DIR:
		return &runZVal{phpv.ZString(path.Dir(l.Filename)), l}, nil
	case tokenizer.T_CLASS_C:
		class := c.getClass()
		if class == nil {
			return &runZVal{phpv.ZString(""), l}, nil
		}
		// In a trait, __CLASS__ must resolve at runtime to the using class name
		if class.Type == phpv.ZClassTypeTrait {
			return &runClassConstant{l: l}, nil
		}
		return &runZVal{class.Name, l}, nil
	case tokenizer.T_FUNC_C:
		f := c.getFunc()
		if f == nil {
			return &runZVal{phpv.ZString(""), l}, nil
		}
		name := f.name
		if name == "" {
			// Anonymous closure: PHP 8 uses {closure:file:line}
			name = phpv.ZString(fmt.Sprintf("{closure:%s:%d}", l.Filename, l.Line))
		}
		return &runZVal{phpv.ZString(name), l}, nil
	case tokenizer.T_NS_C:
		return &runZVal{c.getNamespace(), l}, nil
	case tokenizer.T_METHOD_C:
		class := c.getClass()
		f := c.getFunc()
		if f == nil {
			return &runZVal{phpv.ZString(""), l}, nil
		}
		funcName := f.name
		if funcName == "" {
			// Anonymous closure: __METHOD__ returns just the closure name
			// without class prefix (even when defined inside a class method)
			funcName = phpv.ZString(fmt.Sprintf("{closure:%s:%d}", l.Filename, l.Line))
			return &runZVal{phpv.ZString(funcName), l}, nil
		}
		if class == nil {
			return &runZVal{phpv.ZString(funcName), l}, nil
		}
		// __METHOD__ should only return "Class::method" when we're inside a method
		// of the class. If we're at the class level (e.g., class constant), __METHOD__
		// returns "". Check if the function context is directly inside the class by
		// seeing if the current context chain has: class -> function (method case).
		// If it's function -> class (class inside function), __METHOD__ is "".
		if _, isClassCtx := c.(*zclassCompileCtx); isClassCtx {
			// We're directly inside a class context (not inside a method body)
			// This means __METHOD__ is being used in a class constant or property default
			return &runZVal{phpv.ZString(""), l}, nil
		}
		return &runZVal{phpv.ZString(fmt.Sprintf("%s::%s", class.Name, funcName)), l}, nil
	case tokenizer.T_TRAIT_C:
		class := c.getClass()
		if class != nil && class.Type == phpv.ZClassTypeTrait {
			return &runZVal{class.Name, l}, nil
		}
		return &runZVal{phpv.ZString(""), l}, nil
	case tokenizer.T_PROPERTY_C:
		// __PROPERTY__ returns the property name when inside a property hook, "" otherwise.
		// The hook function name format is "$propName::get" or "$propName::set".
		f := c.getFunc()
		if f != nil && len(f.name) > 0 && f.name[0] == '$' {
			parts := strings.SplitN(string(f.name), "::", 2)
			if len(parts) >= 2 {
				return &runZVal{phpv.ZString(parts[0][1:]), l}, nil // strip leading $
			}
		}
		return &runZVal{phpv.ZString(""), l}, nil
	case tokenizer.T_UNSET_CAST:
		phpErr := &phpv.PhpError{
			Err:  fmt.Errorf("The (unset) cast is no longer supported"),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  l,
		}
		c.Global().LogError(phpErr)
		return nil, phpv.ExitError(255)
	case tokenizer.T_VOID_CAST:
		// (void) cast: evaluate the expression and discard the result
		t_v, err := compileOpExpr(nil, c)
		if err != nil {
			return nil, err
		}
		return &runVoidCast{expr: t_v}, nil
	case tokenizer.T_BOOL_CAST, tokenizer.T_INT_CAST, tokenizer.T_ARRAY_CAST, tokenizer.T_DOUBLE_CAST, tokenizer.T_OBJECT_CAST, tokenizer.T_STRING_CAST:
		// Check for non-canonical casts and emit deprecation/error
		if i.Data != "" {
			switch i.Data {
			case "real":
				// (real) cast has been removed in PHP 8.0
				phpErr := &phpv.PhpError{
					Err:  fmt.Errorf("The (real) cast has been removed, use (float) instead"),
					Code: phpv.E_PARSE,
					Loc:  l,
				}
				c.Global().LogError(phpErr)
				return nil, phpv.ExitError(255)
			case "boolean":
				c.Deprecated("Non-canonical cast (boolean) is deprecated, use the (bool) cast instead", logopt.NoFuncName(true))
			case "integer":
				c.Deprecated("Non-canonical cast (integer) is deprecated, use the (int) cast instead", logopt.NoFuncName(true))
			case "double":
				c.Deprecated("Non-canonical cast (double) is deprecated, use the (float) cast instead", logopt.NoFuncName(true))
			case "binary":
				c.Deprecated("Non-canonical cast (binary) is deprecated, use the (string) cast instead", logopt.NoFuncName(true))
			}
		}
		// perform a cast operation on the following (note: v is null)
		// make this an operator for appropriate operator precedence
		t_v, err := compileOpExpr(nil, c)
		if err != nil {
			return nil, err
		}
		return spawnOperator(c, i.Type, nil, t_v, l)
	case tokenizer.T_INC, tokenizer.T_DEC:
		// this is an operator, let compilePostExpr() deal with it
		return compilePostExpr(nil, i, c)
	case tokenizer.Rune('"'):
		return compileQuoteEncapsed(i, c, '"')
	case tokenizer.Rune('`'):
		v, err := compileQuoteEncapsed(i, c, '`')
		if err != nil {
			return nil, err
		}
		// PHP 8.5: backtick operator is deprecated
		phpErr := &phpv.PhpError{
			Err:  fmt.Errorf("The backtick (`) operator is deprecated, use shell_exec() instead"),
			Code: phpv.E_DEPRECATED,
			Loc:  l,
		}
		c.Global().LogError(phpErr)
		return &runnableFunctionCall{name: "shell_exec", args: []phpv.Runnable{v}, l: l}, nil
	case tokenizer.Rune('!'), tokenizer.Rune('+'), tokenizer.Rune('-'), tokenizer.Rune('~'), tokenizer.Rune('@'):
		// this is an operator, let compilePostExpr() deal with it
		return compilePostExpr(nil, i, c)
	case tokenizer.Rune('['):
		return compileArray(i, c)
	case tokenizer.Rune('('):
		// sub-expr
		v, err := compileExpr(nil, c)
		if err != nil {
			return nil, err
		}
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if !i.IsSingle(')') {
			return nil, i.Unexpected()
		}
		// put the expr into a container to avoid
		return &runParentheses{v}, err
	case tokenizer.T_YIELD:
		return compileYieldExpr(i, c)
	case tokenizer.T_YIELD_FROM:
		return compileYieldExpr(i, c)
	case tokenizer.Rune('&'):
		// get ref of something
		// TODO make this operator?
		v, err := compileOpExpr(nil, c)
		if err != nil {
			return nil, err
		}

		// Cannot take a reference to a class constant
		if _, ok := v.(*runClassStaticObjRef); ok {
			phpErr := &phpv.PhpError{
				Err:  fmt.Errorf("syntax error, unexpected token \"::\""),
				Code: phpv.E_PARSE,
				Loc:  l,
			}
			c.Global().LogError(phpErr)
			return nil, phpv.ExitError(255)
		}

		// Cannot take reference of a nullsafe chain
		if containsNullSafe(v) {
			phpErr := &phpv.PhpError{
				Err:  fmt.Errorf("Cannot take reference of a nullsafe chain"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  l,
			}
			c.Global().LogError(phpErr)
			return nil, phpv.ExitError(255)
		}

		return &runRef{v, l}, nil
	case tokenizer.T_ATTRIBUTE:
		// #[Attr] in expression context: attributes on anonymous functions/closures
		attrs, err := parseAttributes(c)
		if err != nil {
			return nil, err
		}
		// Read what follows: should be function, fn, or static function/fn
		next, err := c.NextItem()
		if err != nil {
			return nil, err
		}
		// Handle additional attribute groups
		for next.Type == tokenizer.T_ATTRIBUTE {
			moreAttrs, err := parseAttributes(c)
			if err != nil {
				return nil, err
			}
			attrs = append(attrs, moreAttrs...)
			next, err = c.NextItem()
			if err != nil {
				return nil, err
			}
		}
		switch next.Type {
		case tokenizer.T_FUNCTION:
			r, err := compileFunction(next, c)
			if err != nil {
				return nil, err
			}
			if zc, ok := r.(*ZClosure); ok {
				zc.attributes = attrs
			}
			return r, nil
		case tokenizer.T_FN:
			r, err := compileArrowFunction(next, c)
			if err != nil {
				return nil, err
			}
			if zc, ok := r.(*ZClosure); ok {
				zc.attributes = attrs
			}
			return r, nil
		case tokenizer.T_STATIC:
			// static function or static fn
			next2, err := c.NextItem()
			if err != nil {
				return nil, err
			}
			switch next2.Type {
			case tokenizer.T_FUNCTION:
				r, err := compileFunction(next2, c)
				if err != nil {
					return nil, err
				}
				if zc, ok := r.(*ZClosure); ok {
					zc.attributes = attrs
					zc.isStatic = true
				}
				return r, nil
			case tokenizer.T_FN:
				r, err := compileArrowFunction(next2, c)
				if err != nil {
					return nil, err
				}
				if zc, ok := r.(*ZClosure); ok {
					zc.attributes = attrs
					zc.isStatic = true
				}
				return r, nil
			default:
				return nil, next2.Unexpected()
			}
		default:
			return nil, next.Unexpected()
		}
	default:
		h, ok := itemTypeHandler[i.Type]
		if ok && h != nil {
			return h.f(i, c)
		} else {
			return nil, i.Unexpected()
		}
	}
}

func compilePostExpr(v phpv.Runnable, i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	if i == nil {
		var err error
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	l := i.Loc()
	// can be any kind of glue (operators, etc)
	switch i.Type {
	case tokenizer.Rune('?'):
		return compileTernaryOp(v, c)
	case tokenizer.Rune('('):
		// this is a function call of whatever is before
		c.backup()
		args, err := compileFuncPassedArgs(c)
		if err != nil {
			return nil, err
		}
		// First-class callable syntax: func(...)
		if IsFirstClassCallable(args) {
			return &runFirstClassCallable{target: v, l: l}, nil
		}
		if constant, ok := v.(*runConstant); ok {
			// Name was already resolved through resolveFunctionName in compileOneExpr
			funcName := phpv.ZString(constant.c)
			// PHP 8: assert() auto-generates description from the AST of its argument
			// In a namespace, the function name is resolved to "Namespace\assert",
			// but assert is a language construct that should always be recognized.
			funcNameLower := strings.ToLower(string(funcName))
			isAssert := funcNameLower == "assert" || strings.HasSuffix(funcNameLower, "\\assert")
			if isAssert && len(args) == 1 {
				var buf strings.Builder
				buf.WriteString("assert(")
				if err := args[0].Dump(&buf); err == nil {
					buf.WriteString(")")
					desc := buf.String()
					args = append(args, &runZVal{phpv.ZString(desc), l})
				}
			}
			return &runnableFunctionCall{name: funcName, args: args, l: l}, nil
		}
		return &runnableFunctionCallRef{v, args, l}, nil
	case tokenizer.Rune('['):
		c.backup()
		return compileArrayAccess(v, c)
	case tokenizer.Rune('{'):
		// PHP8 removed curly-brace array access
		c.backup()
		return nil, nil
	case tokenizer.Rune(';'):
		c.backup()
		// just a value
		return nil, nil
	case tokenizer.T_INC, tokenizer.T_DEC:
		if v == nil {
			// what follows is also an expression
			t_v, err := compileOpExpr(nil, c)
			if err != nil {
				return nil, err
			}
			return spawnOperator(c, i.Type, nil, t_v, l)
		} else {
			return spawnOperator(c, i.Type, v, nil, l)
		}
	case tokenizer.T_OBJECT_OPERATOR:
		return compileObjectOperator(v, i, c, false)
	case tokenizer.T_NULLSAFE_OBJECT_OPERATOR:
		return compileObjectOperator(v, i, c, true)
	case tokenizer.T_PAAMAYIM_NEKUDOTAYIM:
		result, err := compilePaamayimNekudotayim(v, i, c)
		if err != nil {
			return nil, err
		}
		return wrapNullSafeChain(v, result), nil
	case tokenizer.T_INSTANCEOF:
		return compileInstanceOf(v, i, c)
	default:
		if isOperator(i.Type) {
			// what follows should be an expression
			t_v, err := compileOpExpr(nil, c)
			if err != nil {
				return nil, err
			}

			// PHP 8.5: Arrow functions on the RHS of |> must be parenthesized
			if i.Type == tokenizer.T_PIPE {
				if zc, ok := t_v.(*ZClosure); ok && zc.isArrow {
					return nil, &phpv.PhpError{
						Err:  fmt.Errorf("Arrow functions on the right hand side of |> must be parenthesized"),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  l,
					}
				}
			}

			return spawnOperator(c, i.Type, v, t_v, l)
		}
	}

	// unknown?
	c.backup()
	return nil, nil
}

// runClassConstant resolves __CLASS__ at runtime. This is needed when __CLASS__
// is used inside a trait, because the value depends on the class that uses the
// trait rather than the trait itself.
type runClassConstant struct {
	l *phpv.Loc
}

func (r *runClassConstant) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	class := ctx.Class()
	if class != nil {
		return class.GetName().ZVal(), nil
	}
	// Fall back to compiling class (used when evaluating property defaults
	// inherited from traits, where ctx.Class() may be nil but the compiling
	// class is set to the using class)
	if cc := ctx.Global().GetCompilingClass(); cc != nil {
		return cc.GetName().ZVal(), nil
	}
	return phpv.ZString("").ZVal(), nil
}

func (r *runClassConstant) Dump(w io.Writer) error {
	_, err := w.Write([]byte("__CLASS__"))
	return err
}

// runVoidCast evaluates an expression and discards the result.
// Implements the (void) cast in PHP 8.5.
type runVoidCast struct {
	expr phpv.Runnable
}

func (r *runVoidCast) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	_, err := r.expr.Run(ctx)
	if err != nil {
		return nil, err
	}
	return phpv.ZNULL.ZVal(), nil
}

func (r *runVoidCast) Dump(w io.Writer) error {
	w.Write([]byte("(void)"))
	return r.expr.Dump(w)
}
