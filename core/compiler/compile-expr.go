package compiler

import (
	"fmt"
	"math"
	"math/big"
	"path"
	"strconv"

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
		if isOperator(c.peekType()) && !isRightAssociative(c.peekType()) {
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
		if i.Type != tokenizer.T_STRING {
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
				if part.Type != tokenizer.T_STRING {
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
		return &runConstant{name, l}, nil
	case tokenizer.T_NS_SEPARATOR:
		// Fully-qualified name like \TypeError or \PHP_EOL
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.Type != tokenizer.T_STRING {
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
				if part.Type != tokenizer.T_STRING {
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
		return &runConstant{name, l}, nil
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
				if part.Type != tokenizer.T_STRING {
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
			return &runConstant{string(resolved), l}, nil
		}
		// Constant: resolve through namespace
		resolved := c.resolveConstantName(name)
		return &runConstant{resolved, l}, nil

	case tokenizer.T_CONSTANT_ENCAPSED_STRING:
		return compileQuoteConstant(i, c)
	case tokenizer.T_START_HEREDOC:
		return compileQuoteHeredoc(i, c)
	case tokenizer.T_ARRAY:
		return compileArray(i, c)
	case tokenizer.T_MATCH:
		return compileMatch(i, c)
	case tokenizer.T_THROW:
		return compileThrow(i, c)
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
		return &runZVal{class.Name, l}, nil
	case tokenizer.T_FUNC_C:
		f := c.getFunc()
		if f == nil {
			return &runZVal{phpv.ZString(""), l}, nil
		}
		return &runZVal{phpv.ZString(f.name), l}, nil
	case tokenizer.T_NS_C:
		return &runZVal{c.getNamespace(), l}, nil
	case tokenizer.T_METHOD_C:
		class := c.getClass()
		f := c.getFunc()
		if class == nil || f == nil {
			return &runZVal{phpv.ZString(""), l}, nil
		}

		return &runZVal{phpv.ZString(fmt.Sprintf("%s::%s", class.Name, f.name)), l}, nil
	case tokenizer.T_BOOL_CAST, tokenizer.T_INT_CAST, tokenizer.T_ARRAY_CAST, tokenizer.T_DOUBLE_CAST, tokenizer.T_OBJECT_CAST, tokenizer.T_STRING_CAST:
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

		return &runRef{v, l}, nil
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
			return &runnableFunctionCall{name: phpv.ZString(constant.c), args: args, l: l}, nil
		}
		return &runnableFunctionCallRef{v, args, l}, nil
	case tokenizer.Rune('['), tokenizer.Rune('{'):
		c.backup()
		return compileArrayAccess(v, c)
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
		return compilePaamayimNekudotayim(v, i, c)
	case tokenizer.T_INSTANCEOF:
		return compileInstanceOf(v, i, c)
	default:
		if isOperator(i.Type) {
			// what follows should be an expression
			t_v, err := compileOpExpr(nil, c)
			if err != nil {
				return nil, err
			}

			return spawnOperator(c, i.Type, v, t_v, l)
		}
	}

	// unknown?
	c.backup()
	return nil, nil
}
