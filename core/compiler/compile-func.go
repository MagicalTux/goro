package compiler

import (
	"fmt"
	"io"
	"strings"

	"github.com/MagicalTux/goro/core/logopt"
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
	name := string(r.name)
	// PHP AST printing prefixes built-in language constructs with \ for global namespace
	// PHP normalizes "die" to "exit" in AST dumps
	if name == "exit" || name == "die" {
		name = "\\exit"
	}
	_, err := w.Write([]byte(name))
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
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to undefined method %s::%s()", classRef.className, classRef.objName))
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

		// Emit "Undefined variable" warning if $foo is not defined
		if _, exists, _ := ctx.OffsetCheck(ctx, classRef.varName); !exists {
			if err := ctx.Warn("Undefined variable $%s", classRef.varName, logopt.NoFuncName(true)); err != nil {
				return nil, err
			}
		}
		varnameVal, _ := ctx.OffsetGet(ctx, classRef.varName)
		if varnameVal.GetType() != phpv.ZtString {
			return nil, ctx.Errorf("Function name must be a string")
		}
		varname := varnameVal.AsString(ctx)
		method, ok := class.GetMethod(varname)
		if !ok {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to undefined method %s::%s()", className.String(), varname))
		}
		f = method.Method
	} else if f, ok = r.name.(phpv.Callable); !ok {
		v, err := r.name.Run(ctx)
		if err != nil {
			return nil, err
		}

		if obj, ok := v.Value().(*phpobj.ZObject); ok && obj.Class.Handlers() != nil && obj.Class.Handlers().HandleInvoke != nil {
			// For direct closure call $f($args), extract the callable from
			// the closure object and call it directly via ctx.Call so that
			// type error messages include "called in" (matching PHP behavior).
			// Method dispatch ($f->__invoke($args)) uses HandleInvoke with
			// CallZValNoCalledIn which suppresses "called in".
			opaque := obj.GetOpaque(Closure)
			switch z := opaque.(type) {
			case *ZClosure:
				if z.this != nil {
					return ctx.Call(ctx, z, r.args, z.this)
				}
				return ctx.Call(ctx, z, r.args, nil)
			case *generatorClosure:
				if z.this != nil {
					return ctx.Call(ctx, z, r.args, z.this)
				}
				return ctx.Call(ctx, z, r.args, nil)
			case *wrappedClosure:
				if z.this != nil {
					return ctx.Call(ctx, z, r.args, z.this)
				}
				return ctx.Call(ctx, z, r.args, nil)
			default:
				return obj.Class.Handlers().HandleInvoke(ctx, obj, r.args)
			}
		}

		// Check for __invoke method on objects (user-defined classes with __invoke)
		if obj, ok := v.Value().(phpv.ZObject); ok {
			if invokeMethod, hasInvoke := obj.GetClass().GetMethod("__invoke"); hasInvoke {
				// Emit NoDiscard warning before __invoke body runs
				if ndErr := EmitNoDiscardForMagicCall(ctx, invokeMethod.Method, obj.GetClass().GetName(), "__invoke"); ndErr != nil {
					return nil, ndErr
				}
				return ctx.Call(ctx, invokeMethod.Method, r.args, obj)
			}
		}

		if f, ok = v.Value().(phpv.Callable); !ok {
			switch v.GetType() {
			case phpv.ZtString:
				funcName := v.Value().(phpv.ZString)
				// PHP 8: certain functions cannot be called dynamically
				switch string(funcName.ToLower()) {
				case "extract", "compact", "get_defined_vars", "func_get_args",
					"func_get_arg", "func_num_args":
					return nil, phpobj.ThrowError(ctx, phpobj.Error,
						fmt.Sprintf("Cannot call %s() dynamically", funcName))
				}
				if idx := strings.Index(string(funcName), "::"); idx > 0 {
					className := phpv.ZString(funcName[:idx])
					methodName := phpv.ZString(funcName[idx+2:])
					class, classErr := ctx.Global().GetClass(ctx, className, true)
					if classErr == nil {
						if method, methodOk := class.GetMethod(methodName.ToLower()); methodOk {
							f = method.Method
						} else {
							return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to undefined method %s::%s()", className, methodName))
						}
					} else {
						return nil, classErr
					}
				} else {
					var fnErr error
					f, fnErr = ctx.Global().GetFunction(ctx, funcName)
					if fnErr != nil {
						errName := funcName
						if len(errName) > 0 && errName[0] == '\\' {
							errName = errName[1:]
						}
						return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to undefined function %s()", errName))
					}
				}
			case phpv.ZtArray:
				// Array callable: [$obj, "method"] or ["Class", "method"]
				arr := v.Array()
				// Check that indices 0 and 1 exist
				has0, _ := arr.OffsetExists(ctx, phpv.ZInt(0).ZVal())
				has1, _ := arr.OffsetExists(ctx, phpv.ZInt(1).ZVal())
				if !has0 || !has1 {
					if countable, ok := arr.(phpv.ZCountable); !ok || countable.Count(ctx) != 2 {
						return nil, phpobj.ThrowError(ctx, phpobj.Error, "Array callback must have exactly two elements")
					}
					return nil, phpobj.ThrowError(ctx, phpobj.Error, "Array callback has to contain indices 0 and 1")
				}
				first, err1 := arr.OffsetGet(ctx, phpv.ZInt(0))
				second, err2 := arr.OffsetGet(ctx, phpv.ZInt(1))
				if err1 != nil || err2 != nil || first == nil || second == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, "Array callback must have exactly two elements")
				}
				methodName := second.AsString(ctx)
				if first.GetType() == phpv.ZtObject {
					obj := first.AsObject(ctx)
					class := obj.GetClass()
					if method, ok := class.GetMethod(methodName.ToLower()); ok {
						f = phpv.Bind(method.Method, obj)
					} else if methodName.ToLower() == "__invoke" && class.Handlers() != nil && class.Handlers().HandleInvoke != nil {
						// Handle __invoke via HandleInvoke (e.g., Closure::__invoke)
						// Suppress "called in" suffix on type errors (PHP behavior)
						ctx.Global().SetNextCallSuppressCalledIn(true)
						res, err := class.Handlers().HandleInvoke(ctx, obj, r.args)
						ctx.Global().SetNextCallSuppressCalledIn(false)
						return res, err
					} else if callMethod, hasCall := class.GetMethod("__call"); hasCall {
						// Fall back to __call magic method
						var zArgs []*phpv.ZVal
						for _, arg := range r.args {
							val, err := arg.Run(ctx)
							if err != nil {
								return nil, err
							}
							zArgs = append(zArgs, val)
						}
						a := phpv.NewZArray()
						for _, sub := range zArgs {
							a.OffsetSet(ctx, nil, sub.Dup())
						}
						callArgs := []*phpv.ZVal{methodName.ZVal(), a.ZVal()}
						return ctx.CallZVal(ctx, phpv.Bind(callMethod.Method, obj), callArgs, obj)
					} else {
						return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to undefined method %s::%s()", class.GetName(), methodName))
					}
				} else if first.GetType() == phpv.ZtString {
					className := first.AsString(ctx)
					class, classErr := ctx.Global().GetClass(ctx, className, true)
					if classErr != nil {
						return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Class \"%s\" not found", className))
					}
					if method, ok := class.GetMethod(methodName.ToLower()); ok {
						f = phpv.BindClass(method.Method, class, true)
					} else {
						return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to undefined method %s::%s()", className, methodName))
					}
				} else {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "Array callback must have exactly two elements")
				}
			default:
				typeWord := "Value"
				if v.GetType() == phpv.ZtObject {
					typeWord = "Object"
				}
				return nil, phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("%s of type %s is not callable", typeWord, phpv.ZValTypeName(v)))
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

	// PHP 8.5: exit/die are fully reserved and cannot be used as function names.
	if i.Type == tokenizer.T_EXIT {
		return nil, &phpv.PhpError{
			Err:  fmt.Errorf("syntax error, unexpected token \"exit\", expecting \"(\""),
			Code: phpv.E_PARSE,
			Loc:  i.Loc(),
		}
	}

	// Semi-reserved keywords (including 'enum') can be used as function names.
	if i.IsSemiReserved() && i.Type != tokenizer.T_STRING {
		// Treat semi-reserved keyword as a string for function naming
		i.Type = tokenizer.T_STRING
	}

	switch i.Type {
	case tokenizer.T_STRING:
		// regular function definition - prepend namespace
		funcName := phpv.ZString(i.Data)
		ns := c.getNamespace()
		if ns != "" {
			funcName = ns + "\\" + funcName
		}
		// PHP 8.5: defining a custom assert() function is forbidden
		baseName := funcName
		if idx := strings.LastIndex(string(funcName), "\\"); idx >= 0 {
			baseName = funcName[idx+1:]
		}
		if strings.ToLower(string(baseName)) == "assert" {
			phpErr := &phpv.PhpError{
				Err:  fmt.Errorf("Defining a custom assert() function is not allowed, as the function has special semantics"),
				Loc:  l,
				Code: phpv.E_COMPILE_ERROR,
			}
			c.Global().LogError(phpErr)
			return nil, phpv.ExitError(255)
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

	// PHP 8.5: exit/die are fully reserved keywords and cannot be used as labels.
	// If followed by ':', produce a parse error (e.g. `exit:` is not a valid label).
	if i.IsSingle(':') && (fn_name == "exit" || fn_name == "die") {
		return nil, i.Unexpected()
	}

	if i.IsSingle(';') {
		c.backup()
		return &runnableFunctionCall{name: fn_name, l: l}, nil
	}

	// Check for first-class callable syntax: exit(...) / die(...)
	// Only T_EXIT supports this; echo/print/include are language constructs
	// without callable semantics.
	if i.IsSingle('(') && (fn_name == "exit" || fn_name == "die") {
		next, nextErr := c.NextItem()
		if nextErr != nil {
			return nil, nextErr
		}
		if next.Type == tokenizer.T_ELLIPSIS {
			close, closeErr := c.NextItem()
			if closeErr != nil {
				return nil, closeErr
			}
			if close.IsSingle(')') {
				// PHP normalizes "die" to "exit" for first-class callables
				callableName := string(fn_name)
				if callableName == "die" {
					callableName = "exit"
				}
				return &runFirstClassCallable{
					target: &runConstant{c: callableName},
					l:      l,
				}, nil
			}
			// Saw '(' '...' but not ')' — this is a syntax error since
			// backup() only supports one level and we can't restore both
			// the '...' and the following token.
			return nil, close.Unexpected()
		}
		if next.IsSingle(')') {
			// exit() / die() with no arguments
			return &runnableFunctionCall{name: fn_name, l: l}, nil
		}
		// Not '...'; backup the token after '(' so the expression parser
		// handles the full parenthesized expression (e.g., exit(42)).
		c.backup()
		// i is still '(' — fall through to expression parsing
	}

	var args []phpv.Runnable

	// For include/require constructs, parse a single expression and return
	// without consuming the terminator - they can be used in expression context
	// where the terminator is ) not ;
	isInclude := fn_name == "include" || fn_name == "require" || fn_name == "include_once" || fn_name == "require_once" || fn_name == "print"
	if isInclude {
		a, err := compileExpr(i, c)
		if err != nil {
			return nil, err
		}
		return &runnableFunctionCall{name: fn_name, args: []phpv.Runnable{a}, l: l}, nil
	}

	// parse passed arguments (for echo which takes multiple comma-separated args)
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

// compileExitExpr handles exit/die in expression context (PHP 8.5).
func compileExitExpr(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	fn_name := phpv.ZString(i.Data)
	l := i.Loc()

	next, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if next.IsSingle('(') {
		after, err := c.NextItem()
		if err != nil {
			return nil, err
		}
		if after.IsSingle(')') {
			return &runnableFunctionCall{name: fn_name, l: l}, nil
		}
		if after.Type == tokenizer.T_ELLIPSIS {
			close, err := c.NextItem()
			if err != nil {
				return nil, err
			}
			if close.IsSingle(')') {
				callableName := string(fn_name)
				if callableName == "die" {
					callableName = "exit"
				}
				return &runFirstClassCallable{
					target: &runConstant{c: callableName},
					l:      l,
				}, nil
			}
			return nil, close.Unexpected()
		}
		c.backup()
		arg, err := compileExpr(nil, c)
		if err != nil {
			return nil, err
		}
		close, err := c.NextItem()
		if err != nil {
			return nil, err
		}
		if !close.IsSingle(')') {
			return nil, close.Unexpected()
		}
		return &runnableFunctionCall{name: fn_name, args: []phpv.Runnable{arg}, l: l}, nil
	}

	c.backup()
	return &runnableFunctionCall{name: fn_name, l: l}, nil
}

func compileFunctionWithName(name phpv.ZString, c compileCtx, l *phpv.Loc, rref bool, optionalBody ...bool) (phpv.ZClosure, error) {
	var err error

	zc := &ZClosure{
		name:  name,
		start: l,
		rref:  rref,
	}

	// For anonymous closures, capture the enclosing function/method name
	// for PHP 8.4+ closure naming: {closure:enclosingFunc():line}
	if name == "" {
		if enclosing := c.getFunc(); enclosing != nil && enclosing.Name() != "" {
			encName := enclosing.Name()
			if strings.HasPrefix(encName, "{closure:") {
				// Nested closure: use the full closure name as scope (no ()  suffix)
				zc.enclosingFunc = encName
			} else {
				// Named function/method: prepend class name and add ()
				if cls := c.getClass(); cls != nil {
					encName = string(cls.GetName()) + "::" + encName
				}
				zc.enclosingFunc = encName + "()"
			}
		}
	}

	c = &zclosureCompileCtx{c, zc}

	args, err := compileFunctionArgs(c)
	if err != nil {
		return nil, err
	}
	zc.args = args

	// Reject promoted properties outside class methods
	if c.getClass() == nil {
		for _, arg := range args {
			if arg.Promotion != 0 {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("Cannot declare promoted property outside a constructor"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  l,
				}
			}
		}
	}

	// Emit deprecation/error for implicitly nullable parameters
	for _, arg := range args {
		if arg.ImplicitlyNullable {
			// For promoted properties, implicit nullable is a fatal error
			// (typed properties cannot be implicitly nullable)
			if arg.Promotion != 0 {
				// Show the original non-nullable type in the error message
				typeName := arg.Hint.String()
				if strings.HasPrefix(typeName, "?") {
					typeName = typeName[1:]
				}
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("Cannot use null as default value for parameter $%s of type %s", arg.VarName, typeName),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  l,
				}
			}
			funcName := string(name)
			if funcName == "" {
				// Anonymous closure: use {closure:file:line} format
				if l != nil && l.Filename != "" {
					funcName = fmt.Sprintf("{closure:%s:%d}", l.Filename, l.Line)
				} else {
					funcName = "{closure}"
				}
			} else if cls := c.Global().GetCompilingClass(); cls != nil {
				funcName = string(cls.GetName()) + "::" + funcName
			}
			c.Deprecated("%s(): Implicitly marking parameter $%s as nullable is deprecated, the explicit nullable type must be used instead", funcName, arg.VarName, logopt.Data{Loc: l})
		}
	}

	// PHP 8.0: Emit deprecation for optional parameters before required parameters
	lastRequiredIdx := -1
	for idx := len(args) - 1; idx >= 0; idx-- {
		if args[idx].Required && !args[idx].Variadic {
			lastRequiredIdx = idx
			break
		}
	}
	if lastRequiredIdx > 0 {
		for idx := 0; idx < lastRequiredIdx; idx++ {
			arg := args[idx]
			if arg.Required || arg.Variadic {
				continue
			}
			if arg.ImplicitlyNullable {
				continue
			}
			funcName := string(name)
			if funcName == "" {
				if l != nil && l.Filename != "" {
					funcName = fmt.Sprintf("{closure:%s:%d}", l.Filename, l.Line)
				} else {
					funcName = "{closure}"
				}
			} else if cls := c.Global().GetCompilingClass(); cls != nil {
				funcName = string(cls.GetName()) + "::" + funcName
			}
			c.Deprecated("%s(): Optional parameter $%s declared before required parameter $%s is implicitly treated as a required parameter", funcName, arg.VarName, args[lastRequiredIdx].VarName, logopt.Data{Loc: l})
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
		// Validate use variables don't conflict with parameter names
		for _, u := range zc.use {
			for _, a := range zc.args {
				if u.VarName == a.VarName {
					return nil, &phpv.PhpError{
						Err:  fmt.Errorf("Cannot use lexical variable $%s as a parameter name", u.VarName),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  l,
					}
				}
			}
		}

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	// Handle return type after use() clause: function() use ($x): Type { ... }
	if i.IsSingle(':') && zc.returnType == nil {
		zc.returnType, err = parseReturnType(c)
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
			// Abstract method — check for disallowed constructor promotion
			for _, arg := range zc.args {
				if arg.Promotion != 0 {
					return nil, &phpv.PhpError{
						Err:  fmt.Errorf("Cannot declare promoted property in an abstract constructor"),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  l,
					}
				}
			}
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

	// Validate break/continue usage: they must be inside a loop or switch
	if breakErr := validateBreakContinue(zc.code, 0); breakErr != nil {
		return nil, breakErr
	}

	// Check if the closure body references $this (for unbinding warnings)
	if name == "" {
		for _, vn := range collectVariableNames(zc.code) {
			if vn == "this" {
				zc.usesThis = true
				break
			}
		}
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
		start:   l,
		rref:    rref,
		isArrow: true,
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
	case *runVariableRef:
		collectVarsWalk(v.v, seen, result)
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
	case *runnableIf:
		collectVarsWalk(v.cond, seen, result)
		collectVarsWalk(v.yes, seen, result)
		collectVarsWalk(v.no, seen, result)
	case *runnableIsset:
		for _, arg := range v.args {
			collectVarsWalk(arg, seen, result)
		}
	case *runnableEmpty:
		collectVarsWalk(v.arg, seen, result)
	case *runReturn:
		collectVarsWalk(v.v, seen, result)
	case *runnableTry:
		collectVarsWalk(v.try, seen, result)
		for _, c := range v.catches {
			collectVarsWalk(c.body, seen, result)
		}
		collectVarsWalk(v.finally, seen, result)
	case *runnableFor:
		for _, s := range v.start {
			collectVarsWalk(s, seen, result)
		}
		for _, c := range v.cond {
			collectVarsWalk(c, seen, result)
		}
		for _, e := range v.each {
			collectVarsWalk(e, seen, result)
		}
		collectVarsWalk(v.code, seen, result)
	case *runnableWhile:
		collectVarsWalk(v.cond, seen, result)
		collectVarsWalk(v.code, seen, result)
	case *runNoDiscardStatement:
		collectVarsWalk(v.inner, seen, result)
	case *runDestroyTemporary:
		collectVarsWalk(v.inner, seen, result)
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
		if i.Type == tokenizer.T_STATIC {
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use the static modifier on a parameter"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
		}
		for i.Type == tokenizer.T_PUBLIC || i.Type == tokenizer.T_PROTECTED || i.Type == tokenizer.T_PRIVATE || i.Type == tokenizer.T_READONLY || i.Type == tokenizer.T_STATIC || i.Type == tokenizer.T_FINAL {
			if i.Type == tokenizer.T_STATIC {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("Cannot use the static modifier on a parameter"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  i.Loc(),
				}
			}
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
			case tokenizer.T_FINAL:
				// PHP 8.5: final modifier on promoted properties
				// Mark as promoted if not already (final implies promotion)
				if arg.Promotion == 0 {
					arg.Promotion |= phpv.ZAttrPublic // implicit public if no visibility given
				}
				arg.Promotion |= phpv.ZAttrFinal
			}
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
		}

		// Handle DNF type when '(' was consumed by tryParseAsymmetricSet
		if parenConsumedByAsymmetric {
			c.backup()
			intersect, next, pErr := parseParenIntersection(c)
			if pErr != nil {
				return nil, pErr
			}
			if next.IsSingle('|') {
				arg.Hint, i, err = parseUnionTypeHint(intersect, c)
				if err != nil {
					return nil, err
				}
			} else {
				arg.Hint = intersect
				i = next
			}
		}

		// Handle nullable type hint prefix: ?Type
		isNullable := false
		if arg.Hint == nil && i.IsSingle('?') {
			isNullable = true
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
		}

		// Handle DNF type: (A&B)|C in parameter position
		if arg.Hint == nil && i.IsSingle('(') {
			intersect, next, pErr := parseParenIntersection(c)
			if pErr != nil {
				return nil, pErr
			}
			if next.IsSingle('|') {
				arg.Hint, i, err = parseUnionTypeHint(intersect, c)
				if err != nil {
					return nil, err
				}
			} else {
				arg.Hint = intersect
				i = next
			}
		}

		// Handle leading namespace separator: \ClassName
		hintFullyQualified := false
		hintNamespaceRelative := false
		if arg.Hint == nil && i.Type == tokenizer.T_NS_SEPARATOR {
			hintFullyQualified = true
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
		} else if arg.Hint == nil && i.Type == tokenizer.T_NAMESPACE {
			// namespace\ClassName - namespace-relative type hint
			peek, peekErr := c.NextItem()
			if peekErr != nil {
				return nil, peekErr
			}
			if peek.Type == tokenizer.T_NS_SEPARATOR {
				hintNamespaceRelative = true
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
			} else {
				c.backup()
			}
		}

		if arg.Hint == nil && (i.Type == tokenizer.T_STRING || i.Type == tokenizer.T_ARRAY || i.Type == tokenizer.T_CALLABLE) {
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
			} else if hintNamespaceRelative {
				// namespace\X resolves to CurrentNamespace\X (ns-relative name)
				root := getRootCtx(c)
				if root != nil && root.namespace != "" {
					resolvedHint = string(root.namespace) + "\\" + hint
				} else {
					resolvedHint = hint
				}
			} else {
				resolvedHint = string(c.resolveClassName(phpv.ZString(hint)))
			}

			// PHP 8.1: Warn for confusable type names (integer, double, boolean, resource)
			if !hintFullyQualified && !hintNamespaceRelative && !strings.Contains(hint, "\\") {
				hintLower := strings.ToLower(hint)
				ns := string(c.getNamespace())
				naturalResolved := hint
				if ns != "" {
					naturalResolved = ns + "\\" + hint
				}
				if strings.EqualFold(resolvedHint, naturalResolved) {
					var confusableOf string
					switch hintLower {
					case "integer":
						confusableOf = "int"
					case "double":
						confusableOf = "float"
					case "boolean":
						confusableOf = "bool"
					}
					if confusableOf != "" {
						if ns != "" {
							c.Warn("\"%s\" will be interpreted as a class name. Did you mean \"%s\"? Write \"\\%s\\%s\" or import the class with \"use\" to suppress this warning", hint, confusableOf, ns, hint)
						} else {
							c.Warn("\"%s\" will be interpreted as a class name. Did you mean \"%s\"? Write \"\\%s\" to suppress this warning", hint, confusableOf, hint)
						}
					} else if hintLower == "resource" {
						if ns != "" {
							c.Warn("\"%s\" is not a supported builtin type and will be interpreted as a class name. Write \"\\%s\\%s\" or import the class with \"use\" to suppress this warning", hint, ns, hint)
						} else {
							c.Warn("\"%s\" is not a supported builtin type and will be interpreted as a class name. Write \"\\%s\" to suppress this warning", hint, hint)
						}
					}
				}
			}

			// Check for reserved type names in namespace-qualified positions
			// e.g., bar\int is invalid because "int" is reserved
			if strings.Contains(hint, "\\") {
				if err := checkReservedTypeInNamespace(hint, i.Loc()); err != nil {
					return nil, err
				}
			}

			arg.Hint = phpv.ParseTypeHint(phpv.ZString(resolvedHint))

			// void and never cannot be used as parameter types
			if arg.Hint.Type() == phpv.ZtVoid {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("void cannot be used as a parameter type"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  i.Loc(),
				}
			}
			if arg.Hint.Type() == phpv.ZtNever {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("never cannot be used as a parameter type"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  i.Loc(),
				}
			}

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

		// Validate type hint
		if arg.Hint != nil {
			if err := validateTypeHint(arg.Hint, i.Loc(), classNamesFromCtx(c)...); err != nil {
				return nil, err
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
		arg.Loc = i.Loc()

		if arg.VarName == "this" {
			phpErr := &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use $this as parameter"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
			c.Global().LogError(phpErr)
			return nil, phpv.ExitError(255)
		}

		// Check for duplicate parameter names
		for _, existing := range res {
			if existing.VarName == arg.VarName {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("Redefinition of parameter $%s", arg.VarName),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  i.Loc(),
				}
			}
		}

		// Validate internal attributes on parameters at compile time
		// For promoted properties, also allow attributes targeting properties
		if len(arg.Attributes) > 0 {
			target := phpobj.AttributeTARGET_PARAMETER
			if arg.Promotion != 0 {
				target |= phpobj.AttributeTARGET_PROPERTY
			}
			if msg := phpobj.ValidateInternalAttributeList(c, arg.Attributes, target); msg != "" {
				phpErr := &phpv.PhpError{
					Err:  fmt.Errorf("%s", msg),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  i.Loc(),
				}
				c.Global().LogError(phpErr)
				return nil, phpv.ExitError(255)
			}
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

			// Check for static::class in method default parameter (compile-time error)
			if loc := checkStaticClassInConstExpr(r); loc != nil {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("static::class cannot be used for compile-time class name resolution"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  loc,
				}
			}

			// Resolve self::class and parent::class at compile time in default parameters
			if cn, ok := r.(*runClassNameOf); ok {
				if zv, ok2 := cn.className.(*runZVal); ok2 {
					if s, ok3 := zv.v.(phpv.ZString); ok3 {
						switch s.ToLower() {
						case "self":
							if cls := c.getClass(); cls != nil {
								r = &runZVal{cls.GetName(), cn.l}
							}
						case "parent":
							if cls := c.getClass(); cls != nil {
								if cls.ExtendsStr != "" {
									// Resolve through namespace
									r = &runZVal{cls.ExtendsStr, cn.l}
								} else {
									return nil, &phpv.PhpError{
										Err:  fmt.Errorf("Cannot use \"parent\" when current class scope has no parent"),
										Code: phpv.E_COMPILE_ERROR,
										Loc:  cn.l,
									}
								}
							}
						}
					}
				}
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
				if isNull && !arg.Hint.Nullable && arg.Hint.Type() != phpv.ZtMixed {
					arg.ImplicitlyNullable = true
					arg.Hint.Nullable = true
				}
			}

			// Check: default value must be type-compatible with the type hint
			if arg.Hint != nil && !isNull {
				valTypeName := ""
				if zv, ok := r.(*runZVal); ok {
					switch zv.v.(type) {
					case phpv.ZInt:
						valTypeName = "int"
					case phpv.ZFloat:
						valTypeName = "float"
					case phpv.ZString:
						valTypeName = "string"
					case phpv.ZBool:
						valTypeName = "bool"
					}
				} else if _, ok := r.(runConcat); ok {
					// Double-quoted strings compile to runConcat; the result is always a string
					valTypeName = "string"
				} else if rc, ok := r.(*runConstant); ok {
					// Handle true/false/null constants
					switch strings.ToLower(shortName(rc.c)) {
					case "true", "false":
						valTypeName = "bool"
					case "null":
						// null is handled by the nullable check above
					}
				}
				if valTypeName != "" {
					hintType := arg.Hint.Type()
					hintName := arg.Hint.String()
					incompatible := false
					// Skip union/intersection types - too complex for compile-time
					if len(arg.Hint.Union) == 0 && len(arg.Hint.Intersection) == 0 {
						switch hintType {
						case phpv.ZtObject:
							// class-typed parameter cannot have a scalar default
							if arg.Hint.ClassName() != "" {
								incompatible = true
								// Use String() for iterable (displays as "Traversable|array"),
								// otherwise use raw class name
								if arg.Hint.ClassName() != "iterable" {
									hintName = string(arg.Hint.ClassName())
								}
							}
						case phpv.ZtArray:
							incompatible = true
							hintName = "array"
						case phpv.ZtInt:
							// int only accepts int literal defaults
							incompatible = valTypeName != "int"
						case phpv.ZtFloat:
							// float accepts int and float literal defaults
							incompatible = valTypeName != "float" && valTypeName != "int"
						case phpv.ZtString:
							// string only accepts string literal defaults
							incompatible = valTypeName != "string"
						case phpv.ZtBool:
							// bool only accepts bool literal defaults
							incompatible = valTypeName != "bool"
						}
					}
					if incompatible {
						phpErr := &phpv.PhpError{
							Err:  fmt.Errorf("Cannot use %s as default value for parameter $%s of type %s", valTypeName, arg.VarName, hintName),
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

		// PHP 8.4: Property hooks in constructor promoted properties
		// A { after a parameter implies promotion even without visibility keyword
		if i.IsSingle('{') && (arg.Promotion != 0 || c.getClass() != nil) {
			// If no explicit visibility, imply public promotion
			if arg.Promotion == 0 {
				arg.Promotion = phpv.ZAttrPublic
			}
			// Parse property hooks for promoted parameter
			hookProp := &phpv.ZClassProp{
				VarName:   arg.VarName,
				Modifiers: arg.Promotion,
				TypeHint:  arg.Hint,
			}
			// We need a class context for compilePropertyHooks
			cls := c.getClass()
			if cls == nil {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("Cannot declare property hooks outside of a class context"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  i.Loc(),
				}
			}
			err = compilePropertyHooks(hookProp, cls, c)
			if err != nil {
				return nil, err
			}
			arg.PromotionHooks = hookProp
			// Read the next token after the closing }
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

		varName := phpv.ZString(i.Data[1:]) // skip $

		// Check for auto-global variables (cannot be used in use())
		if varName == "this" {
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use $this as lexical variable"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
		}
		switch varName {
		case "GLOBALS", "_SERVER", "_GET", "_POST", "_COOKIE", "_FILES", "_REQUEST", "_SESSION", "_ENV":
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use auto-global as lexical variable"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
		}

		// Check for duplicate use variables
		for _, existing := range res {
			if existing.VarName == varName {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("Cannot use variable $%s twice", varName),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  i.Loc(),
				}
			}
		}

		res = append(res, &phpv.FuncUse{VarName: varName, Ref: isRef})

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

// classNamesFromCtx extracts the current class name and parent class name
// from a compile context, for use with validateTypeHint.
func classNamesFromCtx(c compileCtx) []phpv.ZString {
	cls := c.getClass()
	if cls == nil {
		return nil
	}
	names := []phpv.ZString{cls.Name}
	if cls.Extends != nil {
		names = append(names, cls.Extends.Name)
	} else if cls.ExtendsStr != "" {
		// Parent class might not be resolved yet, but we have the name string
		names = append(names, cls.ExtendsStr)
	} else {
		names = append(names, "")
	}
	return names
}

// validateTypeHint checks a parsed TypeHint for PHP compile-time validity rules.
// It returns a compile error if the type is invalid (e.g., ?mixed, mixed|X, ?void).
// className is the current class name (for resolving self/parent); may be empty.
// parentClassName is the parent class name; may be empty.
func validateTypeHint(th *phpv.TypeHint, loc *phpv.Loc, className ...phpv.ZString) error {
	if th == nil {
		return nil
	}

	// mixed cannot be nullable: "Type mixed cannot be marked as nullable since mixed already includes null"
	if th.Nullable && th.Type() == phpv.ZtMixed {
		return &phpv.PhpError{
			Err:  fmt.Errorf("Type mixed cannot be marked as nullable since mixed already includes null"),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  loc,
		}
	}

	// void cannot be nullable: "Void can only be used as a standalone type"
	if th.Nullable && th.Type() == phpv.ZtVoid {
		return &phpv.PhpError{
			Err:  fmt.Errorf("Void can only be used as a standalone type"),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  loc,
		}
	}

	// null cannot be marked as nullable
	if th.Nullable && th.Type() == phpv.ZtNull {
		return &phpv.PhpError{
			Err:  fmt.Errorf("null cannot be marked as nullable"),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  loc,
		}
	}

	// Union type validations
	if len(th.Union) > 0 {
		for _, u := range th.Union {
			// mixed cannot be in union: "Type mixed can only be used as a standalone type"
			if u.Type() == phpv.ZtMixed {
				return &phpv.PhpError{
					Err:  fmt.Errorf("Type mixed can only be used as a standalone type"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  loc,
				}
			}
			// void cannot be in union: "Void can only be used as a standalone type"
			if u.Type() == phpv.ZtVoid {
				return &phpv.PhpError{
					Err:  fmt.Errorf("Void can only be used as a standalone type"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  loc,
				}
			}

			// Validate intersection type members: scalar types cannot be in intersections
			if len(u.Intersection) > 0 {
				for _, part := range u.Intersection {
					if err := validateIntersectionMember(part, loc); err != nil {
						return err
					}
					// self/parent cannot be part of intersection types if not resolvable
					if part.Type() == phpv.ZtObject {
						cn := strings.ToLower(string(part.ClassName()))
						if cn == "self" || cn == "parent" {
							canResolve := false
							if len(className) > 0 && className[0] != "" {
								if cn == "self" {
									canResolve = true
								}
								if cn == "parent" && len(className) > 1 && className[1] != "" {
									canResolve = true
								}
							}
							if !canResolve {
								return &phpv.PhpError{
									Err:  fmt.Errorf("Type %s cannot be part of an intersection type", part.ClassName()),
									Code: phpv.E_COMPILE_ERROR,
									Loc:  loc,
								}
							}
						}
					}
				}
			}
		}

		// never cannot be in a union
		for _, u := range th.Union {
			if u.Type() == phpv.ZtNever {
				return &phpv.PhpError{
					Err:  fmt.Errorf("never can only be used as a standalone type"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  loc,
				}
			}
		}

		// Check for duplicate types and bool/true/false conflicts.
		// Resolve self/parent to the actual class name for duplicate detection.
		var curClassName, curParentName phpv.ZString
		if len(className) > 0 {
			curClassName = className[0]
		}
		if len(className) > 1 {
			curParentName = className[1]
		}

		resolveKey := func(u *phpv.TypeHint) (key string, displayName string) {
			if u.Type() == phpv.ZtObject {
				lowerName := strings.ToLower(string(u.ClassName()))
				if lowerName == "self" {
					if curClassName != "" {
						return strings.ToLower(string(curClassName)), string(curClassName)
					}
					// Even without class context, normalize "SELF" -> "self" for duplicate detection
					return "self", "self"
				}
				if lowerName == "parent" {
					if curParentName != "" {
						return strings.ToLower(string(curParentName)), string(curParentName)
					}
					return "parent", "parent"
				}
			}
			return strings.ToLower(u.String()), u.String()
		}

		seen := make(map[string]bool)
		hasFalse := false
		hasTrue := false
		hasBool := false
		for _, u := range th.Union {
			key, displayName := resolveKey(u)
			if u.Type() == phpv.ZtBool {
				if u.ClassName() == "false" {
					hasFalse = true
					key = "false"
				} else if u.ClassName() == "true" {
					hasTrue = true
					key = "true"
				} else {
					hasBool = true
					key = "bool"
				}
			}
			// Expand iterable into Traversable and array for duplicate detection.
			// PHP expands iterable at compile time, so iterable|iterable detects
			// "Duplicate type array is redundant" since array is checked first.
			if u.Type() == phpv.ZtObject && u.ClassName() == "iterable" {
				if seen["array"] {
					return &phpv.PhpError{
						Err:  fmt.Errorf("Duplicate type array is redundant"),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  loc,
					}
				}
				if seen["traversable"] {
					return &phpv.PhpError{
						Err:  fmt.Errorf("Duplicate type Traversable is redundant"),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  loc,
					}
				}
				seen["traversable"] = true
				seen["array"] = true
				continue
			}
			if seen[key] {
				return &phpv.PhpError{
					Err:  fmt.Errorf("Duplicate type %s is redundant", displayName),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  loc,
				}
			}
			seen[key] = true
		}
		if hasBool && hasFalse {
			return &phpv.PhpError{
				Err:  fmt.Errorf("Duplicate type false is redundant"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  loc,
			}
		}
		if hasBool && hasTrue {
			return &phpv.PhpError{
				Err:  fmt.Errorf("Duplicate type true is redundant"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  loc,
			}
		}
		if hasTrue && hasFalse {
			return &phpv.PhpError{
				Err:  fmt.Errorf("Type contains both true and false, bool must be used instead"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  loc,
			}
		}

		// Check for semantic redundancies
		hasIterable := false
		hasArray := false
		hasObject := false // object type (matches any object)
		hasNull := false
		hasTraversable := false
		hasStatic := false
		var classNames []string // non-special class names in the union

		for _, u := range th.Union {
			lowerName := strings.ToLower(string(u.ClassName()))
			switch {
			case u.Type() == phpv.ZtObject && u.ClassName() == "iterable":
				hasIterable = true
			case u.Type() == phpv.ZtArray:
				hasArray = true
			case u.Type() == phpv.ZtObject && u.ClassName() == "":
				hasObject = true
			case u.Type() == phpv.ZtNull:
				hasNull = true
			case u.Type() == phpv.ZtObject && lowerName == "traversable":
				hasTraversable = true
			case u.Type() == phpv.ZtObject && u.ClassName() == "static":
				hasStatic = true
			case u.Type() == phpv.ZtObject && u.ClassName() != "" &&
				u.ClassName() != "static" &&
				u.ClassName() != "iterable" && u.ClassName() != "callable":
				lnm := strings.ToLower(string(u.ClassName()))
				if lnm == "self" && curClassName != "" {
					classNames = append(classNames, string(curClassName))
				} else if lnm == "parent" && curParentName != "" {
					classNames = append(classNames, string(curParentName))
				} else if lnm != "self" && lnm != "parent" {
					classNames = append(classNames, string(u.ClassName()))
				}
			}
		}

		// iterable already includes array
		if hasIterable && hasArray {
			return &phpv.PhpError{
				Err:  fmt.Errorf("Duplicate type array is redundant"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  loc,
			}
		}
		// iterable already includes Traversable
		if hasIterable && hasTraversable {
			return &phpv.PhpError{
				Err:  fmt.Errorf("Duplicate type Traversable is redundant"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  loc,
			}
		}
		// object includes all class types (including intersection types and static)
		if hasObject {
			hasClassType := len(classNames) > 0 || hasStatic
			// Also check for intersection types (DNF)
			if !hasClassType {
				for _, u := range th.Union {
					if len(u.Intersection) > 0 {
						hasClassType = true
						break
					}
				}
			}
			if hasClassType {
				return &phpv.PhpError{
					Err:  fmt.Errorf("Type %s contains both object and a class type, which is redundant", th.String()),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  loc,
				}
			}
		}
		// nullable + null in union
		if th.Nullable && hasNull {
			return &phpv.PhpError{
				Err:  fmt.Errorf("null cannot be marked as nullable"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  loc,
			}
		}
	}

	// Intersection type validations (standalone, not within a union)
	if len(th.Intersection) > 0 {
		for _, part := range th.Intersection {
			if err := validateIntersectionMember(part, loc); err != nil {
				return err
			}
			// self/parent cannot be part of intersection types if not resolvable
			if part.Type() == phpv.ZtObject {
				cn := strings.ToLower(string(part.ClassName()))
				if cn == "self" || cn == "parent" {
					// Check if we have a class context and it's not a trait
					canResolve := false
					if len(className) > 0 && className[0] != "" {
						// self is resolvable
						if cn == "self" {
							canResolve = true
						}
						// parent is resolvable only if there's a parent
						if cn == "parent" && len(className) > 1 && className[1] != "" {
							canResolve = true
						}
					}
					if !canResolve {
						return &phpv.PhpError{
							Err:  fmt.Errorf("Type %s cannot be part of an intersection type", part.ClassName()),
							Code: phpv.E_COMPILE_ERROR,
							Loc:  loc,
						}
					}
				}
			}
		}
		// Check for duplicate types in intersection
		seen := make(map[string]bool)
		for _, part := range th.Intersection {
			key := strings.ToUpper(part.String())
			if seen[key] {
				return &phpv.PhpError{
					Err:  fmt.Errorf("Duplicate type %s is redundant", part.String()),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  loc,
				}
			}
			seen[key] = true
		}
	}

	return nil
}

// validateIntersectionMember checks a single member of an intersection type.
// Scalar types, array, callable, void, never, mixed, null, bool, true, false
// cannot be part of an intersection type.
func validateIntersectionMember(part *phpv.TypeHint, loc *phpv.Loc) error {
	t := part.Type()
	name := part.ClassName()

	var errType string
	switch t {
	case phpv.ZtInt:
		errType = "int"
	case phpv.ZtFloat:
		errType = "float"
	case phpv.ZtString:
		errType = "string"
	case phpv.ZtBool:
		if name == "false" {
			errType = "false"
		} else if name == "true" {
			errType = "true"
		} else {
			errType = "bool"
		}
	case phpv.ZtArray:
		errType = "array"
	case phpv.ZtNull:
		errType = "null"
	case phpv.ZtVoid:
		errType = "void"
	case phpv.ZtNever:
		errType = "never"
	case phpv.ZtMixed:
		errType = "mixed"
	case phpv.ZtObject:
		// Check for special pseudo-types
		switch name {
		case "callable":
			errType = "callable"
		case "iterable":
			errType = "Traversable|array"
		case "static":
			errType = "static"
		case "":
			errType = "object"
		}
	}

	if errType != "" {
		return &phpv.PhpError{
			Err:  fmt.Errorf("Type %s cannot be part of an intersection type", errType),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  loc,
		}
	}
	return nil
}

// checkReservedTypeInNamespace checks if a namespace-qualified type name ends with a
// reserved type name (e.g., bar\int, foo\string). PHP does not allow such usage.
func checkReservedTypeInNamespace(hint string, loc *phpv.Loc) error {
	parts := strings.Split(hint, "\\")
	if len(parts) < 2 {
		return nil
	}
	lastPart := strings.ToLower(parts[len(parts)-1])
	switch lastPart {
	case "int", "float", "string", "bool", "array", "object", "mixed",
		"void", "never", "null", "true", "false", "callable", "iterable",
		"self", "parent", "static":
		return &phpv.PhpError{
			Err:  fmt.Errorf("Cannot use \"%s\" as a type name as it is reserved", hint),
			Code: phpv.E_COMPILE_ERROR,
			Loc:  loc,
		}
	}
	return nil
}

// parseParenIntersection parses a parenthesized intersection group: (A&B&C)
// The opening '(' has already been consumed. Returns the intersection TypeHint
// and the next token after ')'.
func parseParenIntersection(c compileCtx) (*phpv.TypeHint, *tokenizer.Item, error) {
	intersection := &phpv.TypeHint{}
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
		if i.Type != tokenizer.T_STRING && i.Type != tokenizer.T_ARRAY && i.Type != tokenizer.T_CALLABLE && i.Type != tokenizer.T_STATIC {
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
		intersection.Intersection = append(intersection.Intersection, phpv.ParseTypeHint(phpv.ZString(hint)))
		if i.IsSingle(')') {
			// End of group, get next token
			next, err := c.NextItem()
			if err != nil {
				return nil, nil, err
			}
			return intersection, next, nil
		}
		if !i.IsSingle('&') {
			return nil, nil, i.Unexpected()
		}
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
		// Handle DNF: (A&B) group within union
		if i.IsSingle('(') {
			intersect, next, pErr := parseParenIntersection(c)
			if pErr != nil {
				return nil, nil, pErr
			}
			union.Union = append(union.Union, intersect)
			if !next.IsSingle('|') {
				return union, next, nil
			}
			continue
		}
		// Handle leading namespace separator
		if i.Type == tokenizer.T_NS_SEPARATOR {
			i, err = c.NextItem()
			if err != nil {
				return nil, nil, err
			}
		}
		if i.Type != tokenizer.T_STRING && i.Type != tokenizer.T_ARRAY && i.Type != tokenizer.T_CALLABLE && i.Type != tokenizer.T_STATIC {
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
			// If the result is a Union with a single Intersection member,
			// unwrap it to a plain Intersection type. This happens when
			// parsing standalone intersection types like X&Y&Z (not DNF).
			if len(union.Union) == 1 && len(union.Union[0].Intersection) > 0 {
				return union.Union[0], i, nil
			}
			return union, i, nil
		}
	}
}

// parseIntersectionTypeHint parses an intersection type (A&B&C).
// `first` is the already-parsed first type, `second` is the T_STRING token
// right after the &. Returns combined hint and next token.
func parseIntersectionTypeHint(first *phpv.TypeHint, secondToken *tokenizer.Item, c compileCtx) (*phpv.TypeHint, *tokenizer.Item, error) {
	intersection := &phpv.TypeHint{Intersection: []*phpv.TypeHint{first}}

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
	intersection.Intersection = append(intersection.Intersection, phpv.ParseTypeHint(phpv.ZString(hint)))

	// Check for more & types
	for i.IsSingle('&') {
		i, err = c.NextItem()
		if err != nil {
			return nil, nil, err
		}
		if i.Type != tokenizer.T_STRING && i.Type != tokenizer.T_ARRAY && i.Type != tokenizer.T_CALLABLE && i.Type != tokenizer.T_STATIC {
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
		intersection.Intersection = append(intersection.Intersection, phpv.ParseTypeHint(phpv.ZString(hint)))
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

	// Handle DNF type: (A&B)|C in return position
	if i.IsSingle('(') {
		intersect, next, pErr := parseParenIntersection(c)
		if pErr != nil {
			return nil, pErr
		}
		if next.IsSingle('|') {
			th, next2, pErr2 := parseUnionTypeHint(intersect, c)
			if pErr2 != nil {
				return nil, pErr2
			}
			c.backup()
			if err := validateTypeHint(th, next2.Loc(), classNamesFromCtx(c)...); err != nil {
				return nil, err
			}
			return th, nil
		}
		c.backup()
		if err := validateTypeHint(intersect, next.Loc(), classNamesFromCtx(c)...); err != nil {
			return nil, err
		}
		return intersect, nil
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
	case tokenizer.T_STRING, tokenizer.T_ARRAY, tokenizer.T_CALLABLE, tokenizer.T_STATIC:
		// valid type name - ok (T_STATIC is allowed as a return type)
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

	// Check for reserved type names in namespace-qualified positions
	if strings.Contains(hint, "\\") {
		if err := checkReservedTypeInNamespace(hint, i.Loc()); err != nil {
			return nil, err
		}
	}

	// Resolve the type hint through namespace
	resolvedHint := hint
	if hintFullyQualified {
		resolvedHint = string(c.resolveClassName("\\" + phpv.ZString(hint)))
	} else {
		resolvedHint = string(c.resolveClassName(phpv.ZString(hint)))
	}
	th := phpv.ParseTypeHint(phpv.ZString(resolvedHint))

	// Check for self/parent/static outside of class scope
	// Note: anonymous closures are allowed to use self/parent/static since they may
	// be bound to a class later via Closure::bind() or Closure::bindTo().
	// Named functions outside a class must NOT use these.
	if th.Type() == phpv.ZtObject {
		cn := th.ClassName()
		if cn == "self" || cn == "parent" || cn == "static" {
			class := c.getClass()
			if class == nil {
				// Check if we're inside an anonymous closure (which can be bound later)
				isAnonymousClosure := false
				if f := c.getFunc(); f != nil && f.name == "" {
					isAnonymousClosure = true
				}
				if !isAnonymousClosure {
					return nil, &phpv.PhpError{
						Err:  fmt.Errorf("Cannot use \"%s\" when no class scope is active", cn),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  i.Loc(),
					}
				}
			}
			// "parent" inside an interface or a class with no parent
			if class != nil && cn == "parent" && class.Type == phpv.ZClassTypeInterface {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("Cannot use \"parent\" when current class scope has no parent"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  i.Loc(),
				}
			}
		}
	}

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
		if err := validateTypeHint(th, i.Loc(), classNamesFromCtx(c)...); err != nil {
			return nil, err
		}
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
			if err := validateTypeHint(th, i.Loc(), classNamesFromCtx(c)...); err != nil {
				return nil, err
			}
			return th, nil
		}
		return nil, peek.Unexpected()
	}

	// Not a type continuation - put it back and we're done
	c.backup()
	if err := validateTypeHint(th, i.Loc(), classNamesFromCtx(c)...); err != nil {
		return nil, err
	}
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

	// Use closureFromCallable to create a proper Closure object
	return closureFromCallable(ctx, funcName.ZVal())
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
	hadNamed := false
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
		} else if (hadSpread || hadNamed) && !(i.IsLabel() && c.peekType() == tokenizer.Rune(':')) {
			// Positional argument after named/spread is a compile error
			msg := "Cannot use positional argument after named argument"
			if hadSpread {
				msg = "Cannot use positional argument after argument unpacking"
			}
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("%s", msg),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
		} else if i.IsLabel() && c.peekType() == tokenizer.Rune(':') {
			// Check for named argument: identifier followed by ':'
			hadNamed = true
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
