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
	name   phpv.ZString
	nsName phpv.ZString // fallback: namespace-qualified or global name
	args   []phpv.Runnable
	l      *phpv.Loc
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

		// Check for __invoke method on objects (user-defined classes with __invoke)
		if obj, ok := v.Value().(phpv.ZObject); ok {
			if invokeMethod, hasInvoke := obj.GetClass().GetMethod("__invoke"); hasInvoke {
				return ctx.Call(ctx, invokeMethod.Method, r.args, obj)
			}
		}

		if f, ok = v.Value().(phpv.Callable); !ok {
			v, err = v.As(ctx, phpv.ZtString)
			if err != nil {
				return nil, err
			}
			// grab function — handle "Class::method" syntax
			funcName := v.Value().(phpv.ZString)
			if idx := strings.Index(string(funcName), "::"); idx > 0 {
				className := phpv.ZString(funcName[:idx])
				methodName := phpv.ZString(funcName[idx+2:])
				class, classErr := ctx.Global().GetClass(ctx, className, true)
				if classErr == nil {
					if method, methodOk := class.GetMethod(methodName); methodOk {
						f = method.Method
					} else {
						return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to undefined method %s::%s()", className, methodName))
					}
				} else {
					return nil, classErr
				}
			} else {
				f, err = ctx.Global().GetFunction(ctx, funcName)
			}
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
		// regular function definition - prepend namespace
		funcName := phpv.ZString(i.Data)
		ns := c.getNamespace()
		if ns != "" {
			funcName = ns + "\\" + funcName
		}
		f, err := compileFunctionWithName(funcName, c, l, rref)
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

	// Special function calls (echo, print, exit, etc.) are language constructs
	// and should not be namespace-resolved.

	if i.IsSingle(';') {
		c.backup()
		return &runnableFunctionCall{name: fn_name, l: l}, nil
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
			return &runnableFunctionCall{name: fn_name, args: args, l: l}, nil
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

	return &runnableFunctionCall{name: fn_name, args: []phpv.Runnable{arg}, l: l}, nil
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
		zc.returnType, err = parseReturnType(c)
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

		// If we're in a class context and get ';' instead of '{', give a better error
		if i.IsSingle(';') && name != "" {
			if cls := c.getClass(); cls != nil {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("Non-abstract method %s::%s() must contain body", cls.Name, name),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  l,
				}
			}
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
		zc.returnType, err = parseReturnType(c)
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
		// For nested arrow functions, their auto-captured variables need
		// to be available in our scope too. Walk the use list.
		for _, u := range v.use {
			if !seen[u.VarName] {
				seen[u.VarName] = true
				*result = append(*result, u.VarName)
			}
		}
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

		// Handle #[...] attributes on parameters (PHP 8.0+)
		for i.Type == tokenizer.T_ATTRIBUTE {
			paramAttrs, err := parseAttributes(c)
			if err != nil {
				return nil, err
			}
			arg.Attributes = append(arg.Attributes, paramAttrs...)
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
		}

		// Handle constructor promotion visibility modifiers (PHP 8.0+)
		// Can include readonly modifier before or after visibility
		// Also handles PHP 8.4 asymmetric visibility: public private(set)
		for i.Type == tokenizer.T_PUBLIC || i.Type == tokenizer.T_PROTECTED || i.Type == tokenizer.T_PRIVATE || i.Type == tokenizer.T_READONLY {
			switch i.Type {
			case tokenizer.T_PUBLIC, tokenizer.T_PROTECTED, tokenizer.T_PRIVATE:
				var thisAccess phpv.ZObjectAttr
				switch i.Type {
				case tokenizer.T_PUBLIC:
					thisAccess = phpv.ZAttrPublic
				case tokenizer.T_PROTECTED:
					thisAccess = phpv.ZAttrProtected
				case tokenizer.T_PRIVATE:
					thisAccess = phpv.ZAttrPrivate
				}
				// Check for asymmetric visibility: modifier(set)
				setAccess, isAsymmetric, err := tryParseAsymmetricSet(thisAccess, c)
				if err != nil {
					return nil, err
				}
				if isAsymmetric {
					if arg.Promotion&phpv.ZAttrAccess == 0 {
						arg.Promotion |= phpv.ZAttrPublic // implicit public read
					}
					arg.SetPromotion = setAccess
				} else {
					arg.Promotion |= thisAccess
				}
			case tokenizer.T_READONLY:
				arg.Promotion |= phpv.ZAttrReadonly
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
		hintFullyQualified := false
		if i.Type == tokenizer.T_NS_SEPARATOR {
			hintFullyQualified = true
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

			// Resolve type hint through namespace (for class type hints)
			resolvedHint := hint
			if hintFullyQualified {
				resolvedHint = string(c.resolveClassName("\\" + phpv.ZString(hint)))
			} else {
				resolvedHint = string(c.resolveClassName(phpv.ZString(hint)))
			}
			arg.Hint = phpv.ParseTypeHint(phpv.ZString(resolvedHint))
			if isNullable {
				arg.Hint.Nullable = true
			}

			// Check for union type: Type1|Type2|...
			if i.IsSingle('|') {
				arg.Hint, i, err = parseUnionTypeHint(arg.Hint, c)
				if err != nil {
					return nil, err
				}
			}
		}

		if i.IsSingle('&') {
			// Disambiguate: &$var (reference) vs Type&Type2 (intersection type)
			if arg.Hint != nil {
				// We have a type hint already. Peek to see if this is an intersection type.
				peek, peekErr := c.NextItem()
				if peekErr != nil {
					return nil, peekErr
				}
				if peek.Type == tokenizer.T_STRING || peek.Type == tokenizer.T_ARRAY || peek.Type == tokenizer.T_CALLABLE {
					// Intersection type: A&B — treat like union for now (either type accepted)
					arg.Hint, i, err = parseIntersectionTypeHint(arg.Hint, peek, c)
					if err != nil {
						return nil, err
					}
				} else {
					// It's a reference marker: &$var
					c.backup()
					arg.Ref = true
					i, err = c.NextItem()
					if err != nil {
						return
					}
				}
			} else {
				arg.Ref = true
				i, err = c.NextItem()
				if err != nil {
					return
				}
			}
		}

		// Handle variadic parameter: ...
		if i.Type == tokenizer.T_ELLIPSIS {
			arg.Variadic = true
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
			arg.DefaultValue = &phpv.CompileDelayed{V: r}
			arg.Required = false

			// Check for implicitly nullable parameter (type hint + NULL default)
			isNull := false
			if arg.Hint != nil {
				if zv, ok := r.(*runZVal); ok {
					_, isNull = zv.v.(phpv.ZNull)
				} else if rc, ok := r.(*runConstant); ok {
					isNull = strings.EqualFold(string(rc.c), "null")
				}
				if isNull && !arg.Hint.Nullable {
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

// parseUnionTypeHint takes the first type hint already parsed and a compileCtx
// positioned after the '|', and parses remaining union members.
// Returns the combined union TypeHint and the next token to process.
func parseUnionTypeHint(first *phpv.TypeHint, c compileCtx) (*phpv.TypeHint, *tokenizer.Item, error) {
	union := &phpv.TypeHint{Union: []*phpv.TypeHint{first}}
	for {
		i, err := c.NextItem()
		if err != nil {
			return nil, nil, err
		}
		// Handle leading namespace separator
		if i.Type == tokenizer.T_NS_SEPARATOR {
			i, err = c.NextItem()
			if err != nil {
				return nil, nil, err
			}
		}
		if i.Type != tokenizer.T_STRING && i.Type != tokenizer.T_ARRAY && i.Type != tokenizer.T_CALLABLE {
			return nil, nil, i.Unexpected()
		}
		hint := i.Data
		// Consume namespace parts
		for {
			i, err = c.NextItem()
			if err != nil {
				return nil, nil, err
			}
			if i.Type != tokenizer.T_NS_SEPARATOR {
				break
			}
			i, err = c.NextItem()
			if err != nil {
				return nil, nil, err
			}
			if i.Type != tokenizer.T_STRING {
				return nil, nil, i.Unexpected()
			}
			hint = hint + "\\" + i.Data
		}
		parsedHint := phpv.ParseTypeHint(phpv.ZString(hint))
		if i.IsSingle('&') {
			// Intersection type: A&B (PHP 8.1)
			if len(union.Intersection) == 0 {
				// Move existing union entries to intersection
				union.Intersection = append(union.Union, parsedHint)
				union.Union = nil
			} else {
				union.Intersection = append(union.Intersection, parsedHint)
			}
			continue
		}
		if len(union.Intersection) > 0 {
			// Finishing an intersection within a union (DNF: (A&B)|C)
			union.Intersection = append(union.Intersection, parsedHint)
			// Wrap intersection as a single union member
			intersect := &phpv.TypeHint{Intersection: union.Intersection}
			union.Union = append(union.Union, intersect)
			union.Intersection = nil
		} else {
			union.Union = append(union.Union, parsedHint)
		}
		if !i.IsSingle('|') {
			return union, i, nil
		}
	}
}

// parseIntersectionTypeHint parses an intersection type (A&B&C).
// `first` is the already-parsed first type, `second` is the T_STRING token
// right after the &. Returns combined hint and next token.
func parseIntersectionTypeHint(first *phpv.TypeHint, secondToken *tokenizer.Item, c compileCtx) (*phpv.TypeHint, *tokenizer.Item, error) {
	intersection := &phpv.TypeHint{Union: []*phpv.TypeHint{first}}

	// Parse second type (already have its token)
	hint := secondToken.Data
	var i *tokenizer.Item
	var err error
	for {
		i, err = c.NextItem()
		if err != nil {
			return nil, nil, err
		}
		if i.Type != tokenizer.T_NS_SEPARATOR {
			break
		}
		i, err = c.NextItem()
		if err != nil {
			return nil, nil, err
		}
		if i.Type != tokenizer.T_STRING {
			return nil, nil, i.Unexpected()
		}
		hint = hint + "\\" + i.Data
	}
	intersection.Union = append(intersection.Union, phpv.ParseTypeHint(phpv.ZString(hint)))

	// Check for more & types
	for i.IsSingle('&') {
		i, err = c.NextItem()
		if err != nil {
			return nil, nil, err
		}
		if i.Type != tokenizer.T_STRING && i.Type != tokenizer.T_ARRAY && i.Type != tokenizer.T_CALLABLE {
			return nil, nil, i.Unexpected()
		}
		hint = i.Data
		for {
			i, err = c.NextItem()
			if err != nil {
				return nil, nil, err
			}
			if i.Type != tokenizer.T_NS_SEPARATOR {
				break
			}
			i, err = c.NextItem()
			if err != nil {
				return nil, nil, err
			}
			if i.Type != tokenizer.T_STRING {
				return nil, nil, i.Unexpected()
			}
			hint = hint + "\\" + i.Data
		}
		intersection.Union = append(intersection.Union, phpv.ParseTypeHint(phpv.ZString(hint)))
	}

	return intersection, i, nil
}

// parseReturnType parses a return type declaration after ':' in a function signature.
// Handles: simple types (int, string, void, mixed), nullable (?Type),
// namespaced types (\Foo\Bar), and union/intersection types (Type1|Type2, Type1&Type2).
// Returns the parsed TypeHint, or nil if no return type is declared.
func parseReturnType(c compileCtx) (*phpv.TypeHint, error) {
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	// Handle nullable prefix: ?Type
	isNullable := false
	if i.IsSingle('?') {
		isNullable = true
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	// Handle leading namespace separator: \Foo
	hintFullyQualified := false
	if i.Type == tokenizer.T_NS_SEPARATOR {
		hintFullyQualified = true
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	// Expect a type name token
	switch i.Type {
	case tokenizer.T_STRING, tokenizer.T_ARRAY, tokenizer.T_CALLABLE:
		// valid type name - ok
	default:
		return nil, i.Unexpected()
	}

	hint := i.Data

	// Handle namespace parts
	for {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.Type == tokenizer.T_NS_SEPARATOR {
			// namespace separator, consume next T_STRING
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			if i.Type != tokenizer.T_STRING {
				return nil, i.Unexpected()
			}
			hint = hint + "\\" + i.Data
			continue
		}

		break
	}

	// Resolve the type hint through namespace
	resolvedHint := hint
	if hintFullyQualified {
		resolvedHint = string(c.resolveClassName("\\" + phpv.ZString(hint)))
	} else {
		resolvedHint = string(c.resolveClassName(phpv.ZString(hint)))
	}
	th := phpv.ParseTypeHint(phpv.ZString(resolvedHint))
	if isNullable {
		th.Nullable = true
	}

	// Handle union types (Type1|Type2) and intersection types (Type1&Type2)
	if i.IsSingle('|') {
		th, i, err = parseUnionTypeHint(th, c)
		if err != nil {
			return nil, err
		}
		c.backup()
		return th, nil
	}

	if i.IsSingle('&') {
		// Intersection type in return position: Type1&Type2
		peek, peekErr := c.NextItem()
		if peekErr != nil {
			return nil, peekErr
		}
		if peek.Type == tokenizer.T_STRING || peek.Type == tokenizer.T_ARRAY || peek.Type == tokenizer.T_CALLABLE {
			th, i, err = parseIntersectionTypeHint(th, peek, c)
			if err != nil {
				return nil, err
			}
			c.backup()
			return th, nil
		}
		return nil, peek.Unexpected()
	}

	// Not a type continuation - put it back and we're done
	c.backup()
	return th, nil
}

// NamedArg wraps a Runnable with a parameter name for PHP 8.0 named arguments.
type NamedArg struct {
	Name phpv.ZString
	Arg  phpv.Runnable
}

func (n *NamedArg) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	return n.Arg.Run(ctx)
}

func (n *NamedArg) ArgName() phpv.ZString {
	return n.Name
}

func (n *NamedArg) Inner() phpv.Runnable {
	return n.Arg
}

func (n *NamedArg) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s: ", n.Name)
	if err != nil {
		return err
	}
	return n.Arg.Dump(w)
}

// FirstClassCallableMarker is returned by compileFuncPassedArgs when it detects
// the first-class callable syntax func(...). Callers should check for this and
// create a closure wrapping the function instead of a regular call.
type FirstClassCallableMarker struct{}

func (f *FirstClassCallableMarker) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	return nil, fmt.Errorf("internal: FirstClassCallableMarker should not be run directly")
}

func (f *FirstClassCallableMarker) Dump(w io.Writer) error {
	_, err := w.Write([]byte("..."))
	return err
}

// IsFirstClassCallable checks if args represent a first-class callable syntax.
func IsFirstClassCallable(args phpv.Runnables) bool {
	return len(args) == 1 && isFirstClassCallableMarker(args[0])
}

func isFirstClassCallableMarker(r phpv.Runnable) bool {
	_, ok := r.(*FirstClassCallableMarker)
	return ok
}

// runFirstClassCallable implements the first-class callable syntax: strlen(...)
// It resolves the function at runtime and wraps it in a closure.
type runFirstClassCallable struct {
	target phpv.Runnable
	l      *phpv.Loc
}

func (r *runFirstClassCallable) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	// Resolve the function name
	var funcName phpv.ZString
	if constant, ok := r.target.(*runConstant); ok {
		funcName = phpv.ZString(constant.c)
	} else {
		val, err := r.target.Run(ctx)
		if err != nil {
			return nil, err
		}
		funcName = phpv.ZString(val.String())
	}

	// Look up the function
	f, err := ctx.Global().GetFunction(ctx, funcName)
	if err != nil {
		return nil, err
	}

	// Create a Closure wrapping this function
	closure := phpv.Bind(f, nil)
	return phpv.NewZVal(closure), nil
}

func (r *runFirstClassCallable) Dump(w io.Writer) error {
	err := r.target.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte("(...)"))
	return err
}

// SpreadArg wraps a Runnable for the argument unpacking syntax: func(...$arr)
type SpreadArg struct {
	Arg phpv.Runnable
}

func (s *SpreadArg) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	return s.Arg.Run(ctx)
}

func (s *SpreadArg) Inner() phpv.Runnable {
	return s.Arg
}

func (s *SpreadArg) Dump(w io.Writer) error {
	_, err := w.Write([]byte("..."))
	if err != nil {
		return err
	}
	return s.Arg.Dump(w)
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

	// Check for first-class callable syntax: func(...)
	if i.Type == tokenizer.T_ELLIPSIS {
		next, nextErr := c.NextItem()
		if nextErr != nil {
			return nil, nextErr
		}
		if next.IsSingle(')') {
			// Return special sentinel: FirstClassCallable marker
			return phpv.Runnables{&FirstClassCallableMarker{}}, nil
		}
		// Not first-class callable, put back the token after ...
		c.backup()
	}

	// parse passed arguments
	hadSpread := false
	for {
		var a phpv.Runnable

		// Check for spread operator: ...$expr
		if i.Type == tokenizer.T_ELLIPSIS {
			hadSpread = true
			spreadExpr, spreadErr := compileExpr(nil, c)
			if spreadErr != nil {
				return nil, spreadErr
			}
			a = &SpreadArg{Arg: spreadExpr}
		} else if hadSpread && !(i.IsLabel() && c.peekType() == tokenizer.Rune(':')) {
			// Positional argument after spread is a compile error
			phpErr := &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use positional argument after argument unpacking"),
				Code: phpv.E_ERROR,
				Loc:  i.Loc(),
			}
			c.Global().LogError(phpErr)
			return nil, phpv.ExitError(255)
		} else if i.IsLabel() && c.peekType() == tokenizer.Rune(':') {
			// Check for named argument: identifier followed by ':'
			// Use peekType() to check the next token without consuming it.
			// Note: ':' won't appear after an identifier in normal expressions
			// (T_DOUBLE_COLON '::' is a separate token), so this is safe.
			// PHP 8.0 allows keywords as named argument names (e.g., array:, match:)
			argName := phpv.ZString(i.Data)
			c.NextItem() // consume the ':'
			nextI, nextErr := c.NextItem()
			if nextErr != nil {
				return nil, nextErr
			}
			a, err = compileExpr(nextI, c)
			if err != nil {
				return nil, err
			}
			a = &NamedArg{Name: argName, Arg: a}
		} else {
			a, err = compileExpr(i, c)
			if err != nil {
				return nil, err
			}
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
