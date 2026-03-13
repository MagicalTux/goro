package compiler

import (
	"fmt"
	"io"
	"strings"

	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runnableFunctionCall struct {
	name phpv.ZString
	args []phpv.Runnable
	l    *phpv.Loc
}

func (*runnableFunctionCall) IsFuncCallExpression()    {}
func (*runnableFunctionCallRef) IsFuncCallExpression() {}

type runnableFunctionCallRef struct {
	name phpv.Runnable
	args []phpv.Runnable
	l    *phpv.Loc
}

func (r *runnableFunctionCall) Dump(w io.Writer) error {
	_, err := w.Write([]byte(r.name))
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{'('})
	if err != nil {
		return err
	}
	// args
	first := true
	for _, a := range r.args {
		if !first {
			_, err = w.Write([]byte{','})
			if err != nil {
				return err
			}
		}
		first = false
		err = a.Dump(w)
		if err != nil {
			return err
		}
	}
	_, err = w.Write([]byte{')'})
	return err
}

func (r *runnableFunctionCallRef) Dump(w io.Writer) error {
	err := r.name.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{'('})
	if err != nil {
		return err
	}
	// args
	first := true
	for _, a := range r.args {
		if !first {
			_, err = w.Write([]byte{','})
			if err != nil {
				return err
			}
		}
		first = false
		err = a.Dump(w)
		if err != nil {
			return err
		}
	}
	_, err = w.Write([]byte{')'})
	return err
}

func (r *runnableFunctionCall) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	err = ctx.Tick(ctx, r.l)
	if err != nil {
		return nil, err
	}
	// grab function
	f, err := ctx.Global().GetFunction(ctx, r.name)
	if err != nil {
		return nil, err
	}

	return ctx.Call(ctx, f, r.args, nil)
}

func (r *runnableFunctionCallRef) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	var f phpv.Callable

	err = ctx.Tick(ctx, r.l)
	if err != nil {
		return nil, err
	}

	if classRef, ok := r.name.(*runClassStaticObjRef); ok {
		className, err := classRef.className.Run(ctx)
		if err != nil {
			return nil, err
		}

		class, err := ctx.Global().GetClass(ctx, className.AsString(ctx), false)
		if err != nil {
			return nil, err
		}
		method, ok := class.GetMethod(classRef.objName)
		if !ok {
			return nil, ctx.Errorf("Call to undefined method %s::%s()", classRef.className, classRef.objName)
		}
		f = method.Method
	} else if classRef, ok := r.name.(*runClassStaticVarRef); ok {
		className, err := classRef.className.Run(ctx)
		if err != nil {
			return nil, err
		}

		class, err := ctx.Global().GetClass(ctx, className.AsString(ctx), false)
		if err != nil {
			return nil, err
		}

		varnameVal, _ := ctx.OffsetGet(ctx, classRef.varName)
		if varnameVal.GetType() != phpv.ZtString {
			return nil, ctx.Errorf("Function name must be a string")
		}
		varname := varnameVal.AsString(ctx)
		method, ok := class.GetMethod(varname)
		if !ok {
			return nil, ctx.Errorf("Call to undefined method %s::%s()", className.String(), varname)
		}
		f = method.Method
	} else if f, ok = r.name.(phpv.Callable); !ok {
		v, err := r.name.Run(ctx)
		if err != nil {
			return nil, err
		}

		if f, ok := v.Value().(*phpobj.ZObject); ok && f.Class.Handlers() != nil && f.Class.Handlers().HandleInvoke != nil {
			return f.Class.Handlers().HandleInvoke(ctx, f, r.args)
		}

		if f, ok = v.Value().(phpv.Callable); !ok {
			v, err = v.As(ctx, phpv.ZtString)
			if err != nil {
				return nil, err
			}
			// grab function
			f, err = ctx.Global().GetFunction(ctx, v.Value().(phpv.ZString))
			if err != nil {
				return nil, err
			}
		}
	}

	return ctx.Call(ctx, f, r.args, nil)
}

func compileFunction(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// typically T_FUNCTION is followed by:
	// - a name and parameters → this is a regular function
	// - directly parameters → this is a lambda function
	l := i.Loc()

	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	rref := false
	if i.IsSingle('&') {
		// this is a ref return function
		rref = true

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	switch i.Type {
	case tokenizer.T_STRING:
		// regular function definition
		f, err := compileFunctionWithName(phpv.ZString(i.Data), c, l, rref)
		if err != nil {
			return nil, err
		}
		return f, nil
	case tokenizer.Rune('('):
		// function with no name is lambda
		c.backup()
		f, err := compileFunctionWithName("", c, l, rref)
		if err != nil {
			return nil, err
		}
		return f, nil
	}

	return nil, i.Unexpected()
}

func compileSpecialFuncCall(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// special function call that comes with optional (), so as a keyword. Example: echo, die, etc
	fn_name := phpv.ZString(i.Data)
	l := i.Loc()

	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.IsSingle(';') {
		c.backup()
		return &runnableFunctionCall{fn_name, nil, l}, nil
	}

	var args []phpv.Runnable

	// parse passed arguments
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
			// read and parse next argument
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			continue
		}
		if i.IsExpressionEnd() {
			c.backup()
			return &runnableFunctionCall{fn_name, args, l}, nil
		}

		return nil, i.Unexpected()
	}
}

func compileSpecialFuncCallOne(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// empty() and eval only takes one expression argument,
	// anything more or less is a syntax error.
	// Parenthesis is required.
	fn_name := phpv.ZString(i.Data)
	l := i.Loc()

	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if !i.IsSingle('(') {
		return nil, i.Unexpected()
	}

	arg, err := compileExpr(nil, c)
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

	return &runnableFunctionCall{fn_name, []phpv.Runnable{arg}, l}, nil
}

func compileFunctionWithName(name phpv.ZString, c compileCtx, l *phpv.Loc, rref bool, optionalBody ...bool) (phpv.ZClosure, error) {
	var err error

	zc := &ZClosure{
		name:  name,
		start: l,
		rref:  rref,
	}

	c = &zclosureCompileCtx{c, zc}

	args, err := compileFunctionArgs(c)
	if err != nil {
		return nil, err
	}
	zc.args = args

	// Emit deprecation for implicitly nullable parameters
	for _, arg := range args {
		if arg.ImplicitlyNullable {
			funcName := string(name)
			if cls := c.Global().GetCompilingClass(); cls != nil {
				funcName = string(cls.GetName()) + "::" + funcName
			}
			c.Deprecated("%s(): Implicitly marking parameter $%s as nullable is deprecated, the explicit nullable type must be used instead", funcName, arg.VarName)
		}
	}

	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	// Handle return type declaration: function foo(): Type { ... }
	if i.IsSingle(':') {
		// Skip the return type - we parse but don't enforce it yet
		err = skipReturnType(c)
		if err != nil {
			return nil, err
		}
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	if i.Type == tokenizer.T_USE && name == "" {
		// anonymous function variables
		zc.use, err = compileFunctionUse(c)
		if err != nil {
			return nil, err
		}

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	if !i.IsSingle('{') {
		if len(optionalBody) > 0 && optionalBody[0] && i.IsSingle(';') {
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			c.backup()
			zc.end = i.Loc()
			zc.code = phpv.RunNull{}
			return zc, nil
		}

		return nil, i.Unexpected()
	}

	zc.code, err = compileBase(nil, c)
	if err != nil {
		return nil, err
	}

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	c.backup()
	zc.end = i.Loc()

	return zc, nil
}

// compileArrowFunction compiles: fn(args) => expr
// Arrow functions auto-capture variables from the enclosing scope by value.
func compileArrowFunction(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	l := i.Loc()

	rref := false
	// Check for fn&(...) => expr (return by reference)
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}
	if i.IsSingle('&') {
		rref = true
	} else {
		c.backup()
	}

	zc := &ZClosure{
		start: l,
		rref:  rref,
	}

	c = &zclosureCompileCtx{c, zc}

	// Parse arguments
	args, err := compileFunctionArgs(c)
	if err != nil {
		return nil, err
	}
	zc.args = args

	// Optional return type: fn(): Type => expr
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	if i.IsSingle(':') {
		err = skipReturnType(c)
		if err != nil {
			return nil, err
		}
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	// Expect =>
	if i.Type != tokenizer.T_DOUBLE_ARROW {
		return nil, i.Unexpected()
	}

	// Parse body expression
	body, err := compileExpr(nil, c)
	if err != nil {
		return nil, err
	}

	// Wrap body in implicit return
	zc.code = &runArrowReturn{body}

	// Collect all variable names referenced in the body to auto-capture.
	// Exclude variables that are function parameters.
	paramNames := make(map[phpv.ZString]bool)
	for _, a := range args {
		paramNames[a.VarName] = true
	}
	varNames := collectVariableNames(body)
	for _, name := range varNames {
		if paramNames[name] || name == "this" {
			continue
		}
		zc.use = append(zc.use, &phpv.FuncUse{VarName: name})
	}

	return zc, nil
}

// runArrowReturn wraps an expression to implicitly return its value
type runArrowReturn struct {
	expr phpv.Runnable
}

func (r *runArrowReturn) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	v, err := r.expr.Run(ctx)
	if err != nil {
		return nil, err
	}
	return nil, &phperr.PhpReturn{V: v}
}

func (r *runArrowReturn) Dump(w io.Writer) error {
	return r.expr.Dump(w)
}

// collectVariableNames walks a Runnable tree and collects all variable names
// referenced by runVariable nodes.
func collectVariableNames(r phpv.Runnable) []phpv.ZString {
	seen := make(map[phpv.ZString]bool)
	var result []phpv.ZString
	collectVarsWalk(r, seen, &result)
	return result
}

func collectVarsWalk(r phpv.Runnable, seen map[phpv.ZString]bool, result *[]phpv.ZString) {
	if r == nil {
		return
	}
	switch v := r.(type) {
	case *runVariable:
		if !seen[v.v] {
			seen[v.v] = true
			*result = append(*result, v.v)
		}
	case *runOperator:
		collectVarsWalk(v.a, seen, result)
		collectVarsWalk(v.b, seen, result)
	case *runnableFunctionCall:
		for _, arg := range v.args {
			collectVarsWalk(arg, seen, result)
		}
	case *runnableFunctionCallRef:
		for _, arg := range v.args {
			collectVarsWalk(arg, seen, result)
		}
	case *runArrayAccess:
		collectVarsWalk(v.value, seen, result)
		collectVarsWalk(v.offset, seen, result)
	case *runObjectVar:
		collectVarsWalk(v.ref, seen, result)
	case *runObjectFunc:
		collectVarsWalk(v.ref, seen, result)
		for _, arg := range v.args {
			collectVarsWalk(arg, seen, result)
		}
	case phpv.Runnables:
		for _, sub := range v {
			collectVarsWalk(sub, seen, result)
		}
	case *runZVal:
		// literal, no variables
	case *ZClosure:
		// nested closure, don't walk into it
	default:
		// For other types, use reflection or just skip
	}
}

func compileFunctionArgs(c compileCtx) (res []*phpv.FuncArg, err error) {
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if !i.IsSingle('(') {
		return nil, i.Unexpected()
	}

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.IsSingle(')') {
		return
	}

	// parse arguments
	for {
		arg := &phpv.FuncArg{}
		arg.Required = true // typically

		// Handle constructor promotion visibility modifiers (PHP 8.0+)
		if i.Type == tokenizer.T_PUBLIC || i.Type == tokenizer.T_PROTECTED || i.Type == tokenizer.T_PRIVATE {
			switch i.Type {
			case tokenizer.T_PUBLIC:
				arg.Promotion = phpv.ZAttrPublic
			case tokenizer.T_PROTECTED:
				arg.Promotion = phpv.ZAttrProtected
			case tokenizer.T_PRIVATE:
				arg.Promotion = phpv.ZAttrPrivate
			}
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
		}

		// Handle nullable type hint prefix: ?Type
		isNullable := false
		if i.IsSingle('?') {
			isNullable = true
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
		}

		// Handle leading namespace separator: \ClassName
		if i.Type == tokenizer.T_NS_SEPARATOR {
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
		}

		if i.Type == tokenizer.T_STRING || i.Type == tokenizer.T_ARRAY || i.Type == tokenizer.T_CALLABLE {
			// this is a function parameter type hint
			hint := i.Data

			for {
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}

				if i.Type != tokenizer.T_NS_SEPARATOR {
					break
				}

				// going to be a ns there!
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
				if i.Type != tokenizer.T_STRING {
					// ending with a ns_separator?
					return nil, i.Unexpected()
				}
				hint = hint + "\\" + i.Data
			}

			arg.Hint = phpv.ParseTypeHint(phpv.ZString(hint))
			if isNullable {
				arg.Hint.Nullable = true
			}
		}

		if i.IsSingle('&') {
			arg.Ref = true
			i, err = c.NextItem()
			if err != nil {
				return
			}
		}

		// Handle variadic parameter: ...
		if i.Type == tokenizer.T_ELLIPSIS {
			// Skip the ... - we treat variadic like a regular param for now
			i, err = c.NextItem()
			if err != nil {
				return
			}
		}

		// in a function declaration, we must have a T_VARIABLE now
		if i.Type != tokenizer.T_VARIABLE {
			return nil, i.Unexpected()
		}

		arg.VarName = phpv.ZString(i.Data[1:]) // skip $

		if arg.VarName == "this" {
			phpErr := &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use $this as parameter"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
			c.Global().LogError(phpErr)
			return nil, phpv.ExitError(255)
		}

		res = append(res, arg)

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle('=') {
			// we have a default value
			r, err := compileExpr(nil, c)
			if err != nil {
				return nil, err
			}
			arg.DefaultValue = &phpv.CompileDelayed{r}
			arg.Required = false

			// Check for implicitly nullable parameter (type hint + NULL default)
			isNull := false
			if arg.Hint != nil {
				if zv, ok := r.(*runZVal); ok {
					_, isNull = zv.v.(phpv.ZNull)
				} else if rc, ok := r.(*runConstant); ok {
					isNull = strings.EqualFold(string(rc.c), "null")
				}
				if isNull {
					arg.ImplicitlyNullable = true
					arg.Hint.Nullable = true
				}
			}

			// Check: class-typed parameters cannot have scalar defaults (except null)
			if arg.Hint != nil && arg.Hint.Type() == phpv.ZtObject && arg.Hint.ClassName() != "" && !isNull {
				if zv, ok := r.(*runZVal); ok {
					typeName := ""
					switch zv.v.(type) {
					case phpv.ZInt:
						typeName = "int"
					case phpv.ZFloat:
						typeName = "float"
					case phpv.ZString:
						typeName = "string"
					case phpv.ZBool:
						typeName = "bool"
					}
					if typeName != "" {
						phpErr := &phpv.PhpError{
							Err:  fmt.Errorf("Cannot use %s as default value for parameter $%s of type %s", typeName, arg.VarName, arg.Hint.ClassName()),
							Code: phpv.E_COMPILE_ERROR,
							Loc:  i.Loc(),
						}
						c.Global().LogError(phpErr)
						return nil, phpv.ExitError(255)
					}
				}
			}

			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
		}

		if i.IsSingle(',') {
			// read and parse next argument
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			// Allow trailing comma (PHP 7.3+)
			if i.IsSingle(')') {
				return
			}
			continue
		}

		if i.IsSingle(')') {
			return // end of arguments
		}

		return nil, i.Unexpected()
	}
}

func compileFunctionUse(c compileCtx) (res []*phpv.FuncUse, err error) {
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if !i.IsSingle('(') {
		return nil, i.Unexpected()
	}

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.IsSingle(')') {
		return
	}

	// parse arguments
	for {
		// Allow & prefix for reference capture
		isRef := false
		if i.IsSingle('&') {
			isRef = true
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
		}

		// in a function declaration, we must have a T_VARIABLE now
		if i.Type != tokenizer.T_VARIABLE {
			return nil, i.Unexpected()
		}

		res = append(res, &phpv.FuncUse{VarName: phpv.ZString(i.Data[1:]), Ref: isRef}) // skip $

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(',') {
			// read and parse next argument
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			// Allow trailing comma (PHP 7.3+)
			if i.IsSingle(')') {
				return
			}
			continue
		}

		if i.IsSingle(')') {
			return // end of arguments
		}

		return nil, i.Unexpected()
	}
}

// skipReturnType skips a return type declaration after ':' in a function signature.
// Handles: simple types (int, string, void, mixed), nullable (?Type),
// namespaced types (\Foo\Bar), and union/intersection types (Type1|Type2, Type1&Type2).
func skipReturnType(c compileCtx) error {
	i, err := c.NextItem()
	if err != nil {
		return err
	}

	// Handle nullable prefix: ?Type
	if i.IsSingle('?') {
		i, err = c.NextItem()
		if err != nil {
			return err
		}
	}

	// Handle leading namespace separator: \Foo
	if i.Type == tokenizer.T_NS_SEPARATOR {
		i, err = c.NextItem()
		if err != nil {
			return err
		}
	}

	// Expect a type name token
	switch i.Type {
	case tokenizer.T_STRING, tokenizer.T_ARRAY, tokenizer.T_CALLABLE:
		// valid type name - ok
	default:
		return i.Unexpected()
	}

	// Handle namespace parts and union/intersection types
	for {
		i, err = c.NextItem()
		if err != nil {
			return err
		}

		if i.Type == tokenizer.T_NS_SEPARATOR {
			// namespace separator, skip next T_STRING
			i, err = c.NextItem()
			if err != nil {
				return err
			}
			if i.Type != tokenizer.T_STRING {
				return i.Unexpected()
			}
			continue
		}

		// Handle union types (Type1|Type2) and intersection types (Type1&Type2)
		if i.IsSingle('|') || i.IsSingle('&') {
			i, err = c.NextItem()
			if err != nil {
				return err
			}
			// Handle leading namespace separator in union member
			if i.Type == tokenizer.T_NS_SEPARATOR {
				i, err = c.NextItem()
				if err != nil {
					return err
				}
			}
			switch i.Type {
			case tokenizer.T_STRING, tokenizer.T_ARRAY, tokenizer.T_CALLABLE:
				// valid - continue to check for more union/namespace parts
				continue
			default:
				return i.Unexpected()
			}
		}

		// Not a type continuation - put it back and we're done
		c.backup()
		return nil
	}
}

func compileFuncPassedArgs(c compileCtx) (res phpv.Runnables, err error) {
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if !i.IsSingle('(') {
		return nil, i.Unexpected()
	}

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.IsSingle(')') {
		return
	}

	// parse passed arguments
	for {
		var a phpv.Runnable
		a, err = compileExpr(i, c)
		if err != nil {
			return nil, err
		}

		res = append(res, a)

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(',') {
			// read and parse next argument
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			// Allow trailing comma (PHP 7.3+)
			if i.IsSingle(')') {
				return
			}
			continue
		}

		if i.IsSingle(')') {
			return // end of arguments
		}

		return nil, i.Unexpected()
	}
}
