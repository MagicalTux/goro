package compiler

import (
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runNewObject struct {
	obj    phpv.ZString
	cl     phpv.Runnable // for anonymous
	newArg phpv.Runnables
	l      *phpv.Loc
}

func (*runNewObject) IsFuncCallExpression() {}

func (r *runNewObject) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "new %s(", r.obj)
	if err != nil {
		return err
	}

	// newargs
	err = r.newArg.DumpWith(w, []byte{','})
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{')'})
	return err
}

func (r *runNewObject) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	ctx.Tick(ctx, r.l)

	var className phpv.ZString
	if r.cl != nil {
		// Dynamic class name from variable/expression
		v, err := r.cl.Run(ctx)
		if err != nil {
			return nil, err
		}
		className = v.AsString(ctx)
	} else {
		className = r.obj
	}

	class, err := ctx.Global().GetClass(ctx, className, true)
	if err != nil {
		return nil, err
	}

	// Determine constructor parameter info for undefined variable warnings
	var funcArgs []*phpv.FuncArg
	if constructor := getConstructor(class); constructor != nil {
		if fga, ok := constructor.(phpv.FuncGetArgs); ok {
			funcArgs = fga.GetArgs()
		}
	}

	// Expand spread arguments (...$arr) into individual args
	expandedArgs, expandErr := expandNewSpreadArgs(ctx, r.newArg)
	if expandErr != nil {
		return nil, expandErr
	}

	var args []*phpv.ZVal
	var byRefCleanups []*phpv.ZVal
	for i, a := range expandedArgs {
		// Emit "Undefined variable" warning for by-value params
		isRefParam := funcArgs != nil && i < len(funcArgs) && funcArgs[i].Ref
		if !isRefParam {
			if uc, ok := a.(phpv.UndefinedChecker); ok {
				if uc.IsUnDefined(ctx) {
					ctx.Warn("Undefined variable $%s",
						uc.VarName(), logopt.NoFuncName(true))
				}
			}
		}

		// For by-ref params, enable write context for auto-vivification
		if isRefParam {
			if wcs, ok := a.(phpv.WriteContextSetter); ok {
				wcs.SetWriteContext(true)
			}
		}

		arg, err := a.Run(ctx)
		if err != nil {
			if isRefParam {
				if wcs, ok := a.(phpv.WriteContextSetter); ok {
					wcs.SetWriteContext(false)
				}
			}
			return nil, err
		}

		if isRefParam {
			if cw, isCW := a.(phpv.CompoundWritable); isCW && !arg.IsRef() {
				// Ensure the element exists (auto-vivification for $undef[0])
				cw.WriteValue(ctx, arg.Dup())
				// Re-read to get the actual hash table entry
				arg, _ = a.Run(ctx)
				// Make the hash table entry into a reference in-place
				arg.MakeRef()
				byRefCleanups = append(byRefCleanups, arg)
			}
			if wcs, ok := a.(phpv.WriteContextSetter); ok {
				wcs.SetWriteContext(false)
			}
		}

		args = append(args, arg)
	}

	z, err := phpobj.NewZObject(ctx, class, args...)

	// Unwrap by-ref hash table entries after constructor returns — but
	// only if no other location still references the same inner ZVal.
	for _, ref := range byRefCleanups {
		ref.UnRefIfAlone()
	}

	if err != nil {
		return nil, err
	}

	return z.ZVal(), nil
}

// getConstructor returns the constructor Callable for a class, or nil.
func getConstructor(class phpv.ZClass) phpv.Callable {
	if h := class.Handlers(); h != nil && h.Constructor != nil {
		return h.Constructor.Method
	}
	if m, ok := class.GetMethod("__construct"); ok {
		return m.Method
	}
	return nil
}

// runNewAnonymousClass handles `new class [(args)] { ... }` anonymous class syntax.
type runNewAnonymousClass struct {
	class           *phpobj.ZClass
	constructorArgs []phpv.Runnable
	l               *phpv.Loc
	compiled        bool
}

func (r *runNewAnonymousClass) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "new class {...}")
	return err
}

func (r *runNewAnonymousClass) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	ctx.Tick(ctx, r.l)

	// Register and compile the anonymous class only once (it may be in a loop)
	if !r.compiled {
		err := ctx.Global().RegisterClass(r.class.Name, r.class)
		if err != nil {
			return nil, err
		}
		err = r.class.Compile(ctx)
		if err != nil {
			ctx.Global().UnregisterClass(r.class.Name)
			return nil, err
		}
		r.compiled = true
	}

	// Evaluate constructor arguments
	var args []*phpv.ZVal
	for _, a := range r.constructorArgs {
		v, err := a.Run(ctx)
		if err != nil {
			return nil, err
		}
		args = append(args, v)
	}

	// Create instance
	obj, err := phpobj.NewZObject(ctx, r.class, args...)
	if err != nil {
		return nil, err
	}
	return obj.ZVal(), nil
}

func compileNew(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// next should be either:
	// T_CLASS (anonymous class)
	// T_VARIABLE (dynamic class name)
	// string (name of a class)
	var err error

	n := &runNewObject{l: i.Loc()}

	// Peek at next token to check for variable class name
	next, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	// Handle attributes before anonymous class: new #[Attr] class { }
	var anonAttrs []*phpv.ZAttribute
	if next.Type == tokenizer.T_ATTRIBUTE {
		anonAttrs, err = parseAttributes(c)
		if err != nil {
			return nil, err
		}
		next, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		// Handle multiple attribute groups
		for next.Type == tokenizer.T_ATTRIBUTE {
			moreAttrs, err := parseAttributes(c)
			if err != nil {
				return nil, err
			}
			anonAttrs = append(anonAttrs, moreAttrs...)
			next, err = c.NextItem()
			if err != nil {
				return nil, err
			}
		}
	}

	if next.Type == tokenizer.T_CLASS {
		// Anonymous class: new class [(args)] [extends X] [implements Y, Z] { ... }
		// Parse optional constructor args first (before the class body)
		peek, err := c.NextItem()
		if err != nil {
			return nil, err
		}

		var constructorArgs []phpv.Runnable
		if peek.IsSingle('(') {
			c.backup()
			constructorArgs, err = compileFuncPassedArgs(c)
			if err != nil {
				return nil, err
			}
		} else {
			c.backup()
		}

		// Compile the class using the normal class compiler but with T_CLASS token
		// We back up and let compileClass handle extends/implements/body
		classRunnable, err := compileClass(next, c)
		if err != nil {
			return nil, err
		}

		var class *phpobj.ZClass
		if zc, ok := classRunnable.(*phpobj.ZClass); ok {
			class = zc
		} else if w, ok := classRunnable.(interface{ GetClass() *phpobj.ZClass }); ok {
			class = w.GetClass()
		} else {
			return nil, fmt.Errorf("invalid anonymous class definition")
		}
		// Apply attributes from new #[Attr] class { }
		if len(anonAttrs) > 0 {
			class.Attributes = append(anonAttrs, class.Attributes...)
		}
		// Generate unique anonymous class name.
		// PHP 8.4+: the prefix is "ParentClass@anonymous" or "FirstInterface@anonymous"
		// or "class@anonymous" if neither extends nor implements.
		prefix := "class"
		if class.ExtendsStr != "" {
			prefix = string(class.ExtendsStr)
		} else if len(class.ImplementsStr) > 0 {
			prefix = string(class.ImplementsStr[0])
		}
		class.Name = phpv.ZString(fmt.Sprintf("%s@anonymous\x00%s:%d$0", prefix, n.l.Filename, n.l.Line))

		return &runNewAnonymousClass{
			class:           class,
			constructorArgs: constructorArgs,
			l:               n.l,
		}, nil
	} else if next.Type == tokenizer.T_VARIABLE {
		v := phpv.Runnable(&runVariable{v: phpv.ZString(next.Data[1:]), l: next.Loc()})
		// Check for property access or array subscript like $this->name or $a[0][1]
		for {
			peek, err := c.NextItem()
			if err != nil {
				return nil, err
			}
			if peek.Type == tokenizer.T_OBJECT_OPERATOR {
				prop, err := c.NextItem()
				if err != nil {
					return nil, err
				}
				v = &runObjectVar{ref: v, varName: phpv.ZString(prop.Data), l: peek.Loc()}
			} else if peek.IsSingle('[') {
				c.backup()
				v, err = compileArrayAccess(v, c)
				if err != nil {
					return nil, err
				}
			} else {
				c.backup()
				break
			}
		}
		n.cl = v
	} else if next.Type == tokenizer.T_STATIC {
		// new static — late static binding (resolved at runtime)
		n.obj = phpv.ZString("static")
	} else {
		c.backup()
		n.obj, err = compileClassName(c)
		if err != nil {
			return nil, err
		}
	}

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	c.backup()

	if !i.IsSingle('(') {
		// no arguments to new
		return n, nil
	}

	// read constructor args
	n.newArg, err = compileFuncPassedArgs(c)

	return n, err
}

type runObjectFunc struct {
	ref      phpv.Runnable
	op       phpv.ZString
	args     phpv.Runnables
	l        *phpv.Loc
	static   bool
	nullsafe bool
}

func (*runObjectFunc) IsFuncCallExpression() {}

type runObjectVar struct {
	ref          phpv.Runnable
	varName      phpv.ZString
	l            *phpv.Loc
	writeContext bool // set when reading as part of a write chain (suppress undefined property warnings)
	nullsafe     bool

	// PrepareWrite caching
	prepared   bool
	cachedProp *phpv.ZVal
}

func (r *runObjectFunc) Dump(w io.Writer) error {
	err := r.ref.Dump(w)
	if err != nil {
		return err
	}
	op := "->"
	if r.nullsafe {
		op = "?->"
	}
	_, err = fmt.Fprintf(w, "%s%s(", op, r.op)
	if err != nil {
		return err
	}

	err = r.args.DumpWith(w, []byte{','})
	if err != nil {
		return err
	}

	_, err = w.Write([]byte{')'})
	return err
}

func (r *runObjectVar) Dump(w io.Writer) error {
	err := r.ref.Dump(w)
	if err != nil {
		return err
	}
	op := "->"
	if r.nullsafe {
		op = "?->"
	}
	_, err = fmt.Fprintf(w, "%s%s", op, r.varName)
	return err
}

func (r *runObjectFunc) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	ctx.Tick(ctx, r.l)

	// For :: calls with a variable receiver, emit "Undefined variable" warning
	// before evaluating. The variable's parent is runObjectFunc which suppresses
	// the warning in runVariable.Run, so we must check it here.
	if r.static {
		if uc, ok := r.ref.(phpv.UndefinedChecker); ok {
			if uc.IsUnDefined(ctx) {
				ctx.Warn("Undefined variable $%s", uc.VarName(), logopt.NoFuncName(true))
			}
		}
	}

	// fetch object
	obj, err := r.ref.Run(ctx)
	if err != nil {
		return nil, err
	}

	if r.nullsafe && obj.GetType() == phpv.ZtNull {
		return phpv.ZNULL.ZVal(), nil
	}

	// For :: calls, validate class name type before evaluating method name.
	if r.static && obj.GetType() != phpv.ZtObject && obj.GetType() != phpv.ZtString {
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			"Class name must be a valid object or a string")
	}

	op := r.op
	if op[0] == '$' {
		// variable method name
		var opz *phpv.ZVal
		opz, err = ctx.OffsetGet(ctx, op[1:].ZVal())
		if err != nil {
			return nil, err
		}
		// Method name must be a string
		if opz.GetType() != phpv.ZtString {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Method name must be a string")
		}
		op = opz.Value().(phpv.ZString)
	}

	var objI phpv.ZObject
	var class phpv.ZClass
	switch obj.GetType() {
	case phpv.ZtObject:
		objI = obj.Value().(*phpobj.ZObject).Unwrap()
		class = objI.GetClass()
	case phpv.ZtString:
		// object receiver is a string, so :: syntax was used
		className := obj.AsString(ctx)

		// :: can take the following as the receiver:
		// - parent::method()
		// - self::method()
		// - ClassName::method() # where ClassName is any class name
		//
		// parent::method() and self::method() are not static calls.
		// self::method() and $this->method() aren't the same.
		// self:: will first search for the method starting with the class
		// where self:: was referred to, and search upwards in the class heirarchy.
		// $this-> will always start searching from the end of the inheritance chain.
		//
		// For example, given A extends B extends C,
		// in B, self::foo() will first search for the method in B, then A.
		// Whereas, in B, $this->foo() will first search for the method in C, B, then A.
		//
		//
		// ClassName::method() may or may not be static call, depending on where
		// it's called from.
		// If it's called from outside the class context, then it's a static call.
		// If ClassName is NOT part of the current inheritance chain, then it's also a static call.
		// Otherwise, it's a non-static call.
		//
		// For example, given A extends B extends C,
		// in B, A::foo() and C::foo() is a non-static call.
		// Whereas, in or outside of B, D::foo() is a static call.

		switch className {
		case "self":
			if ctx.This() != nil {
				objI = ctx.This()
				class = objI.GetClass()
			} else {
				// In static context, resolve self from the closure's class
				class, err = ctx.Global().GetClass(ctx, "self", false)
				if err != nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, `Cannot access "self" when no class scope is active`)
				}
			}

		case "parent":
			if ctx.This() != nil {
				if ctx.This().GetClass().GetParent() == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, `Cannot access "parent" when current class scope has no parent`)
				}
				objI = ctx.This().GetParent()
				class = objI.GetClass()
			} else {
				// In static context, resolve parent from the closure's class
				selfClass, selfErr := ctx.Global().GetClass(ctx, "self", false)
				if selfErr != nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, `Cannot access "parent" when no class scope is active`)
				}
				parentClass := selfClass.GetParent()
				if parentClass == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, `Cannot access "parent" when current class scope has no parent`)
				}
				class = parentClass
			}

		case "static":
			// Late static binding: resolve to the runtime class
			if ctx.This() != nil {
				objI = ctx.This()
				class = objI.GetClass()
			} else {
				class, err = ctx.Global().GetClass(ctx, "static", false)
				if err != nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, `Cannot access "static" when no class scope is active`)
				}
			}

		default:
			nonStatic := false
			if ctx.This() != nil {
				kin := ctx.This().GetKin(string(className))
				if kin != nil {
					objI = kin
					class = objI.GetClass()
					nonStatic = true
				}
			}

			if !nonStatic {
				class, err = ctx.Global().GetClass(ctx, className, true)
				if err != nil {
					return nil, err
				}
			}
		}
	default:
		// PHP 8: calling method on non-object throws Error
		typeName := phpValueTypeName(obj)
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Call to a member function %s() on %s", r.op, typeName))
	}

	method, ok := class.GetMethod(op)

	// Note: #[\Deprecated] check for user methods is handled by ZClosure.Call()
	// which fires when the method body is actually invoked. We do NOT check
	// method.Attributes here to avoid double-firing the deprecation warning.

	// PHP resolves private methods from the caller's class scope, not the runtime class.
	// Private methods are not virtual — when calling $this->method() from within a class
	// that defines a private method with that name, use the caller's private method
	// regardless of what the runtime class hierarchy provides.
	if objI != nil {
		callerClass := ctx.Class()
		if callerClass != nil {
			if callerMethod, callerOk := callerClass.GetMethod(op); callerOk && callerMethod.Modifiers.Has(phpv.ZAttrPrivate) && callerMethod.Class != nil && callerMethod.Class.GetName() == callerClass.GetName() {
				method = callerMethod
				ok = true
			}
		}
	}

	if !ok {
		// Check for __invoke method on objects with HandleInvoke
		if objI != nil && op.ToLower() == "__invoke" && class.Handlers() != nil && class.Handlers().HandleInvoke != nil {
			return class.Handlers().HandleInvoke(ctx, objI, r.args)
		}

		// Check for __call magic method on instance calls.
		// When there's an instance context (objI != nil), __call takes priority
		// over __callStatic, even with :: syntax (self::, static::, ClassName::).
		if objI != nil {
			callClass := class
			callObj := objI
			if r.static && ctx.This() != nil {
				callClass = ctx.This().GetClass()
				callObj = ctx.This()
			}
			if callMethod, hasCall := callClass.GetMethod("__call"); hasCall {
				// Evaluate arguments
				var zArgs []*phpv.ZVal
				for _, arg := range r.args {
					val, err := arg.Run(ctx)
					if err != nil {
						return nil, err
					}
					zArgs = append(zArgs, val)
				}
				// Build args array (each arg must be a copy, not a reference)
				a := phpv.NewZArray()
				for _, sub := range zArgs {
					a.OffsetSet(ctx, nil, sub.Dup())
				}
				callArgs := []*phpv.ZVal{op.ZVal(), a.ZVal()}
				// Wrap in BoundedCallable so stack trace shows class and -> type
				return ctx.CallZVal(ctx, phpv.Bind(callMethod.Method, callObj), callArgs, callObj)
			}
		}

		// For :: syntax without an instance (pure static call), check __callStatic
		if r.static {
			callClass := class
			if ctx.This() != nil {
				callClass = ctx.This().GetClass()
			}
			if callStaticMethod, hasCallStatic := callClass.GetMethod("__callstatic"); hasCallStatic {
				a := phpv.NewZArray()
				callArgs := []*phpv.ZVal{op.ZVal(), a.ZVal()}
				for _, sub := range r.args {
					val, err := sub.Run(ctx)
					if err != nil {
						return nil, err
					}
					a.OffsetSet(ctx, nil, val)
				}
				// Wrap in MethodCallable so stack trace shows class and :: type
				return ctx.CallZVal(ctx, phpv.BindClass(callStaticMethod.Method, callClass, true), callArgs, objI)
			}
		}
		return nil, ctx.Errorf("Call to undefined method %s::%s()", class.GetName(), op)
	}

	// Check if method is abstract - cannot be called directly
	if method.Modifiers.Has(phpv.ZAttrAbstract) || (method.Empty && method.Class != nil && method.Class.GetType() != phpv.ZClassTypeInterface) {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot call abstract method %s::%s()", method.Class.GetName(), method.Name))
	}

	// Check method visibility. If the method is not visible but __call exists on the
	// class, PHP calls __call instead of throwing the visibility error.
	methodNotVisible := false
	var visErrMsg string
	if method.Modifiers.Has(phpv.ZAttrPrivate) {
		callerClass := ctx.Class()
		methodClass := method.Class
		if callerClass == nil || methodClass == nil || callerClass.GetName() != methodClass.GetName() {
			methodClassName := class.GetName()
			if methodClass != nil {
				methodClassName = methodClass.GetName()
			}
			scope := "global scope"
			if callerClass != nil {
				scope = "scope " + string(callerClass.GetName())
			}
			if method.Name == "__construct" {
				if callerClass == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to private %s::__construct() from global scope", methodClassName))
				}
				return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot call private %s::__construct()", methodClassName))
			}
			methodNotVisible = true
			visErrMsg = fmt.Sprintf("Call to private method %s::%s() from %s", methodClassName, method.Name, scope)
		}
	} else if method.Modifiers.Has(phpv.ZAttrProtected) {
		callerClass := ctx.Class()
		if callerClass == nil {
			if method.Name == "__construct" {
				return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to protected %s::__construct() from global scope", class.GetName()))
			}
			methodNotVisible = true
			visErrMsg = fmt.Sprintf("Call to protected method %s::%s() from global scope", class.GetName(), method.Name)
		} else if !callerClass.InstanceOf(method.Class) && !method.Class.InstanceOf(callerClass) {
			if method.Name == "__construct" {
				return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to protected %s::__construct() from scope %s", class.GetName(), callerClass.GetName()))
			}
			methodNotVisible = true
			visErrMsg = fmt.Sprintf("Call to protected method %s::%s() from scope %s", class.GetName(), method.Name, callerClass.GetName())
		}
	}
	if methodNotVisible {
		// Before returning the visibility error, check if __call is available
		if objI != nil {
			callClass := class
			callObj := objI
			if r.static && ctx.This() != nil {
				callClass = ctx.This().GetClass()
				callObj = ctx.This()
			}
			if callMethod, hasCall := callClass.GetMethod("__call"); hasCall {
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
				callArgs := []*phpv.ZVal{op.ZVal(), a.ZVal()}
				return ctx.CallZVal(ctx, callMethod.Method, callArgs, callObj)
			}
		}
		return nil, phpobj.ThrowError(ctx, phpobj.Error, visErrMsg)
	}

	// Save the runtime class before narrowing for late static binding
	runtimeClass := class
	if objI != nil {
		objI = objI.GetKin(string(method.Class.GetName()))
		class = method.Class
	}

	if objI == nil && r.static {
		// :: is used outside of class context
		if !method.Modifiers.IsStatic() {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Non-static method %s::%s() cannot be called statically", class.GetName(), method.Name))
		}

		// Use method.Class (defining class) for self:: resolution,
		// and class (called class) for static:: resolution (LSB)
		bindClass := class
		if method.Class != nil {
			bindClass = method.Class
		}
		m := phpv.BindClassLSB(method.Method, bindClass, class, true)
		return ctx.Call(ctx, m, r.args, nil)
	}

	if r.static {
		// :: syntax but with an object (e.g., parent::method(), self::method())
		// Not truly static: $this is forwarded, so use Static=false for the binding
		m := phpv.BindClass(method.Method, class, false)
		return ctx.Call(ctx, m, r.args, objI)
	}

	// Static methods don't get $this even when called via instance ($obj->staticMethod())
	// Use BindClassLSB to preserve the runtime class for late static binding (get_called_class)
	if method.Modifiers.IsStatic() {
		m := phpv.BindClassLSB(method.Method, class, runtimeClass, true)
		return ctx.Call(ctx, m, r.args, nil)
	}

	return ctx.Call(ctx, method.Method, r.args, objI)
}

func (r *runObjectVar) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	// fetch object property
	obj, err := r.ref.Run(ctx)
	if err != nil {
		return nil, err
	}

	if r.nullsafe && obj.GetType() == phpv.ZtNull {
		return phpv.ZNULL.ZVal(), nil
	}

	objI, ok := obj.Value().(phpv.ZObjectAccess)
	if !ok {
		typeName := phpValueTypeName(obj)
		if r.writeContext {
			// PHP 8: modifying property of non-object in a write chain throws Error
			return nil, phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Attempt to modify property \"%s\" on %s", r.varName, typeName))
		}
		// PHP 8: reading property of non-object is a warning, returns null
		ctx.Warn("Attempt to read property \"%s\" on %s", r.varName, typeName, logopt.NoFuncName(true))
		return phpv.ZNULL.ZVal(), nil
	}

	// offset get
	var offt *phpv.ZVal
	if r.varName[0] == '$' {
		// variable
		offt, err = ctx.OffsetGet(ctx, r.varName[1:].ZVal())
		if err != nil {
			return nil, err
		}
	} else {
		offt = r.varName.ZVal()
	}

	// In write context (e.g. $a->b[0] = x), suppress "Undefined property" warning
	// for auto-vivification. PHP silently creates the property in this case.
	if r.writeContext {
		if oq, ok := objI.(interface {
			ObjectGetQuiet(ctx phpv.Context, key phpv.Val) (*phpv.ZVal, bool, error)
		}); ok {
			v, found, err := oq.ObjectGetQuiet(ctx, offt)
			if err != nil {
				return nil, err
			}
			if !found {
				// Auto-create the property for return-by-reference and
				// write-context auto-vivification. This matches PHP behavior
				// where accessing an undefined property in write/ref context
				// creates it as null without warning.
				if oset, ok2 := objI.(interface {
					ObjectSet(ctx phpv.Context, key phpv.Val, value *phpv.ZVal) error
				}); ok2 {
					nullVal := phpv.ZNULL.ZVal()
					if err := oset.ObjectSet(ctx, offt, nullVal); err != nil {
						return nil, err
					}
					// Re-fetch from hash table to get the actual stored entry
					v, _, err = oq.ObjectGetQuiet(ctx, offt)
					if err != nil {
						return nil, err
					}
					return v, nil
				}
				return phpv.ZNULL.ZVal(), nil
			}
			return v, nil
		}
	}

	// TODO Check access rights
	return objI.ObjectGet(ctx, offt)
}

func (r *runObjectVar) PrepareWrite(ctx phpv.Context) error {
	// If property name is dynamic ($varName), evaluate and cache it
	if r.varName[0] == '$' {
		offt, err := ctx.OffsetGet(ctx, r.varName[1:].ZVal())
		if err != nil {
			return err
		}
		r.prepared = true
		r.cachedProp = offt.Dup()
	}
	return nil
}

func (r *runObjectVar) IsCompoundWritable() {}

func (r *runObjectVar) SetWriteContext(v bool) {
	r.writeContext = v
}

func (r *runObjectVar) WriteValue(ctx phpv.Context, value *phpv.ZVal) error {
	// Set write context on the ref chain so intermediate property accesses
	// produce "Attempt to modify property" errors instead of "Attempt to read" warnings.
	if wcs, ok := r.ref.(phpv.WriteContextSetter); ok {
		wcs.SetWriteContext(true)
	}
	// write object property
	obj, err := r.ref.Run(ctx)
	if wcs, ok := r.ref.(phpv.WriteContextSetter); ok {
		wcs.SetWriteContext(false)
	}
	if err != nil {
		return err
	}

	objI, ok := obj.Value().(phpv.ZObjectAccess)
	if !ok {
		// PHP 8: attempting to set property of non-object throws Error
		typeName := phpValueTypeName(obj)
		return phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Attempt to assign property \"%s\" on %s", r.varName, typeName))
	}

	// offset set
	var offt *phpv.ZVal
	if r.prepared {
		offt = r.cachedProp
		r.prepared = false
		r.cachedProp = nil
	} else if r.varName[0] == '$' {
		// variable
		offt, err = ctx.OffsetGet(ctx, r.varName[1:].ZVal())
		if err != nil {
			return err
		}
	} else {
		offt = r.varName.ZVal()
	}

	// TODO Check access rights
	return objI.ObjectSet(ctx, offt, value)
}

// runObjectDynVar handles $obj->{expr} dynamic property access
type runObjectDynVar struct {
	ref      phpv.Runnable
	nameExpr phpv.Runnable
	l        *phpv.Loc
	nullsafe bool

	// PrepareWrite caching
	prepared   bool
	cachedName *phpv.ZVal
}

func (r *runObjectDynVar) Dump(w io.Writer) error {
	err := r.ref.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte("->{"))
	if err != nil {
		return err
	}
	err = r.nameExpr.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{'}'})
	return err
}

func (r *runObjectDynVar) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	obj, err := r.ref.Run(ctx)
	if err != nil {
		return nil, err
	}
	if r.nullsafe && obj.GetType() == phpv.ZtNull {
		return phpv.ZNULL.ZVal(), nil
	}
	objI, ok := obj.Value().(phpv.ZObjectAccess)
	if !ok {
		// Evaluate the name expression first so we can include it in the warning
		name, nameErr := r.nameExpr.Run(ctx)
		typeName := phpValueTypeName(obj)
		if nameErr == nil && name != nil {
			ctx.Warn("Attempt to read property \"%s\" on %s", name.String(), typeName, logopt.NoFuncName(true))
		} else {
			ctx.Warn("Attempt to read property on %s", typeName, logopt.NoFuncName(true))
		}
		return phpv.ZNULL.ZVal(), nil
	}
	name, err := r.nameExpr.Run(ctx)
	if err != nil {
		return nil, err
	}
	return objI.ObjectGet(ctx, name)
}

func (r *runObjectDynVar) PrepareWrite(ctx phpv.Context) error {
	name, err := r.nameExpr.Run(ctx)
	if err != nil {
		return err
	}
	r.prepared = true
	r.cachedName = name.Dup()
	return nil
}

func (r *runObjectDynVar) IsCompoundWritable() {}

func (r *runObjectDynVar) WriteValue(ctx phpv.Context, value *phpv.ZVal) error {
	obj, err := r.ref.Run(ctx)
	if err != nil {
		return err
	}
	objI, ok := obj.Value().(phpv.ZObjectAccess)
	if !ok {
		typeName := phpValueTypeName(obj)
		return phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Attempt to assign property on %s", typeName))
	}
	var name *phpv.ZVal
	if r.prepared {
		name = r.cachedName
		r.prepared = false
		r.cachedName = nil
	} else {
		name, err = r.nameExpr.Run(ctx)
		if err != nil {
			return err
		}
	}
	return objI.ObjectSet(ctx, name, value)
}

// runObjectDynFunc handles $obj->{expr}() dynamic method call
type runObjectDynFunc struct {
	ref      phpv.Runnable
	nameExpr phpv.Runnable
	args     []phpv.Runnable
	l        *phpv.Loc
	nullsafe bool
	static   bool
}

func (r *runObjectDynFunc) Dump(w io.Writer) error {
	err := r.ref.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte("->{"))
	if err != nil {
		return err
	}
	err = r.nameExpr.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte("}()"))
	return err
}

func (r *runObjectDynFunc) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	obj, err := r.ref.Run(ctx)
	if err != nil {
		return nil, err
	}
	if r.nullsafe && obj.GetType() == phpv.ZtNull {
		return phpv.ZNULL.ZVal(), nil
	}
	name, err := r.nameExpr.Run(ctx)
	if err != nil {
		return nil, err
	}
	if name.GetType() != phpv.ZtString {
		return nil, &phpv.PhpError{
			Err:  fmt.Errorf("Method name must be a string"),
			Code: phpv.E_ERROR,
			Loc:  r.l,
		}
	}
	methodName := phpv.ZString(name.String())

	if r.static && obj.GetType() == phpv.ZtString {
		// Static call: Class::{'method'}()
		className := obj.AsString(ctx)
		var class phpv.ZClass
		var objI phpv.ZObject

		switch className {
		case "self":
			if ctx.This() != nil {
				objI = ctx.This()
				class = objI.GetClass()
			} else {
				class, err = ctx.Global().GetClass(ctx, "self", false)
				if err != nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, `Cannot access "self" when no class scope is active`)
				}
			}
		case "parent":
			if ctx.This() != nil {
				if ctx.This().GetClass().GetParent() == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, `Cannot access "parent" when current class scope has no parent`)
				}
				objI = ctx.This().GetParent()
				class = objI.GetClass()
			} else {
				selfClass, selfErr := ctx.Global().GetClass(ctx, "self", false)
				if selfErr != nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, `Cannot access "parent" when no class scope is active`)
				}
				parentClass := selfClass.GetParent()
				if parentClass == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, `Cannot access "parent" when current class scope has no parent`)
				}
				class = parentClass
			}
		case "static":
			if ctx.This() != nil {
				objI = ctx.This()
				class = objI.GetClass()
			} else {
				class, err = ctx.Global().GetClass(ctx, "static", false)
				if err != nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, `Cannot access "static" when no class scope is active`)
				}
			}
		default:
			nonStatic := false
			if ctx.This() != nil {
				kin := ctx.This().GetKin(string(className))
				if kin != nil {
					objI = kin
					class = objI.GetClass()
					nonStatic = true
				}
			}
			if !nonStatic {
				class, err = ctx.Global().GetClass(ctx, className, true)
				if err != nil {
					return nil, err
				}
			}
		}

		method, ok := class.GetMethod(methodName.ToLower())
		if !ok {
			// Try __callStatic
			method, ok = class.GetMethod("__callstatic")
			if ok {
				a := phpv.NewZArray()
				callArgs := []*phpv.ZVal{methodName.ZVal(), a.ZVal()}
				for _, sub := range r.args {
					v, err := sub.Run(ctx)
					if err != nil {
						return nil, err
					}
					a.OffsetSet(ctx, nil, v)
				}
				return ctx.CallZVal(ctx, method.Method, callArgs, objI)
			}
			// Try __call on instance
			if objI != nil {
				method, ok = class.GetMethod("__call")
				if ok {
					a := phpv.NewZArray()
					callArgs := []*phpv.ZVal{methodName.ZVal(), a.ZVal()}
					for _, sub := range r.args {
						v, err := sub.Run(ctx)
						if err != nil {
							return nil, err
						}
						a.OffsetSet(ctx, nil, v)
					}
					return ctx.CallZVal(ctx, method.Method, callArgs, objI)
				}
			}
			return nil, ctx.Errorf("Call to undefined method %s::%s()", class.GetName(), methodName)
		}
		return ctx.Call(ctx, method.Method, r.args, objI)
	}

	objZ := obj.AsObject(ctx)
	if objZ == nil {
		return nil, ctx.Errorf("Call to a member function %s() on a non-object", methodName)
	}
	method, ok := objZ.GetClass().GetMethod(methodName.ToLower())
	if !ok {
		method, ok = objZ.GetClass().GetMethod("__call")
		if ok {
			a := phpv.NewZArray()
			callArgs := []*phpv.ZVal{methodName.ZVal(), a.ZVal()}
			for _, sub := range r.args {
				v, err := sub.Run(ctx)
				if err != nil {
					return nil, err
				}
				a.OffsetSet(ctx, nil, v)
			}
			return ctx.CallZVal(ctx, method.Method, callArgs, objZ)
		}
		return nil, ctx.Errorf("Call to undefined method %s::%s()", objZ.GetClass().GetName(), methodName)
	}
	return ctx.Call(ctx, method.Method, r.args, objZ)
}

func compilePaamayimNekudotayim(v phpv.Runnable, i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	var err error
	l := i.Loc()

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	ident := phpv.ZString(i.Data)

	switch i.Type {
	case tokenizer.Rune('$'):
		// C::${expr} or C::$var — dynamic static property access
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.Type == tokenizer.Rune('{') {
			// C::${expr}
			expr, err := compileExpr(nil, c)
			if err != nil {
				return nil, err
			}
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			if i.Type != tokenizer.Rune('}') {
				return nil, i.Unexpected()
			}
			return &runClassStaticDynVarRef{className: v, nameExpr: expr, l: l}, nil
		}
		// C::$var — indirect via variable
		c.backup()
		expr, err := compileExpr(nil, c)
		if err != nil {
			return nil, err
		}
		return &runClassStaticDynVarRef{className: v, nameExpr: expr, l: l}, nil
	case tokenizer.T_VARIABLE:
		// Check if followed by ( — dynamic method call: $a::$b()
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.IsSingle('(') {
			c.backup()
			args, err := compileFuncPassedArgs(c)
			if err != nil {
				return nil, err
			}
			return &runObjectFunc{ref: v, op: ident, args: args, l: l, static: true}, err
		}
		c.backup()
		return &runClassStaticVarRef{v, ident[1:], l}, nil

	case tokenizer.Rune('{'):
		// C::{'method'}() — dynamic static method call with brace-enclosed expression
		expr, err := compileExpr(nil, c)
		if err != nil {
			return nil, err
		}
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.Type != tokenizer.Rune('}') {
			return nil, i.Unexpected()
		}
		// Check if followed by ( for method call
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		c.backup()
		if i.IsSingle('(') {
			// Dynamic static method call: C::{'method'}()
			dynFunc := &runObjectDynFunc{ref: v, nameExpr: expr, l: l, static: true}
			dynFunc.args, err = compileFuncPassedArgs(c)
			return dynFunc, err
		}
		// Dynamic static property/constant access: C::{'prop'}
		return &runObjectDynVar{ref: v, nameExpr: expr, l: l}, nil

	case tokenizer.T_CLASS:
		// $obj::class or ClassName::class → get class name
		return &runClassNameOf{v, l}, nil

	default:
		// Semi-reserved keywords can be used as static method/constant names (Foo::exit, Foo::die(), etc.)
		if !i.IsSemiReserved() {
			return nil, i.Unexpected()
		}
		// Fall through to handle as identifier (same as T_STRING)
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		switch i.Type {
		case tokenizer.Rune('('):
			c.backup()
			args, err := compileFuncPassedArgs(c)
			if err != nil {
				return nil, err
			}
			if IsFirstClassCallable(args) {
				return &runFirstClassMethodCallable{ref: v, method: ident, static: true, l: l}, nil
			}
			return &runObjectFunc{ref: v, op: ident, args: args, l: l, static: true}, err
		default:
			c.backup()
			return &runClassStaticObjRef{v, ident, l}, nil
		}
	}
}

func compileObjectOperator(v phpv.Runnable, i *tokenizer.Item, c compileCtx, nullsafe bool) (phpv.Runnable, error) {
	// call a method or get a variable on an object
	l := i.Loc()

	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	// After ->, PHP keywords can be used as property/method names
	switch i.Type {
	case tokenizer.Rune('$'):
		// $obj->${expr} or $obj->$var — variable-variable property access
		// ${expr} evaluates expr, uses result as a variable name, looks up
		// that variable, and uses its value as the property name.
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.Type == tokenizer.Rune('{') {
			// $obj->${expr} — variable variable: evaluate expr, lookup variable
			varRef, err := compileRunVariableRef(i, c, l)
			if err != nil {
				return nil, err
			}
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			c.backup()
			if i.IsSingle('(') {
				dynFunc := &runObjectDynFunc{ref: v, nameExpr: varRef, l: l, nullsafe: nullsafe}
				dynFunc.args, err = compileFuncPassedArgs(c)
				return dynFunc, err
			}
			return &runObjectDynVar{ref: v, nameExpr: varRef, l: l, nullsafe: nullsafe}, nil
		}
		// $obj->$var — indirect property, variable contains property name
		c.backup()
		expr, err := compileExpr(nil, c)
		if err != nil {
			return nil, err
		}
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		c.backup()
		if i.IsSingle('(') {
			dynFunc := &runObjectDynFunc{ref: v, nameExpr: expr, l: l, nullsafe: nullsafe}
			dynFunc.args, err = compileFuncPassedArgs(c)
			return dynFunc, err
		}
		return &runObjectDynVar{ref: v, nameExpr: expr, l: l, nullsafe: nullsafe}, nil
	case tokenizer.Rune('{'):
		// Dynamic property/method: $obj->{expr}
		expr, err := compileExpr(nil, c)
		if err != nil {
			return nil, err
		}
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.Type != tokenizer.Rune('}') {
			return nil, i.Unexpected()
		}
		// Check if followed by ( for method call
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		c.backup()
		if i.IsSingle('(') {
			// Dynamic method call: $obj->{expr}()
			dynFunc := &runObjectDynFunc{ref: v, nameExpr: expr, l: l, nullsafe: nullsafe}
			dynFunc.args, err = compileFuncPassedArgs(c)
			return dynFunc, err
		}
		// Dynamic property access: $obj->{expr}
		return &runObjectDynVar{ref: v, nameExpr: expr, l: l, nullsafe: nullsafe}, nil
	case tokenizer.T_STRING, tokenizer.T_VARIABLE,
		tokenizer.T_ARRAY, tokenizer.T_LIST, tokenizer.T_CLASS,
		tokenizer.T_CALLABLE, tokenizer.T_EMPTY, tokenizer.T_ISSET,
		tokenizer.T_UNSET, tokenizer.T_ECHO, tokenizer.T_PRINT,
		tokenizer.T_FOR, tokenizer.T_FOREACH, tokenizer.T_WHILE,
		tokenizer.T_DO, tokenizer.T_SWITCH, tokenizer.T_IF,
		tokenizer.T_ELSE, tokenizer.T_ELSEIF,
		tokenizer.T_STATIC, tokenizer.T_ABSTRACT, tokenizer.T_FINAL,
		tokenizer.T_FUNCTION, tokenizer.T_NEW, tokenizer.T_CLONE,
		tokenizer.T_RETURN, tokenizer.T_TRY, tokenizer.T_CATCH,
		tokenizer.T_THROW, tokenizer.T_INTERFACE, tokenizer.T_EXTENDS,
		tokenizer.T_IMPLEMENTS, tokenizer.T_CONST, tokenizer.T_PUBLIC,
		tokenizer.T_PROTECTED, tokenizer.T_PRIVATE,
		tokenizer.T_VAR, tokenizer.T_ENUM, tokenizer.T_MATCH,
		tokenizer.T_READONLY, tokenizer.T_INCLUDE, tokenizer.T_REQUIRE,
		tokenizer.T_INCLUDE_ONCE, tokenizer.T_REQUIRE_ONCE,
		tokenizer.T_EXIT, tokenizer.T_EVAL:
		// all valid after ->
	default:
		return nil, i.Unexpected()
	}
	op := phpv.ZString(i.Data)

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	c.backup()

	if i.IsSingle('(') {
		// this is a function call
		args, err := compileFuncPassedArgs(c)
		if err != nil {
			return nil, err
		}
		if IsFirstClassCallable(args) {
			return &runFirstClassMethodCallable{ref: v, method: op, static: false, nullsafe: nullsafe, l: l}, nil
		}
		return &runObjectFunc{ref: v, op: op, args: args, l: l, nullsafe: nullsafe}, nil
	}

	return &runObjectVar{ref: v, varName: op, l: l, nullsafe: nullsafe}, nil
}

// runFirstClassMethodCallable implements ClassName::method(...) and $obj->method(...)
// first-class callable syntax.
type runFirstClassMethodCallable struct {
	ref      phpv.Runnable
	method   phpv.ZString
	static   bool
	nullsafe bool
	l        *phpv.Loc
}

func (r *runFirstClassMethodCallable) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	if err := ctx.Tick(ctx, r.l); err != nil {
		return nil, err
	}

	if r.static {
		// Static method: ClassName::method(...)
		classNameVal, err := r.ref.Run(ctx)
		if err != nil {
			return nil, err
		}
		class, err := ctx.Global().GetClass(ctx, classNameVal.AsString(ctx), false)
		if err != nil {
			return nil, err
		}
		member, ok := class.GetMethod(r.method.ToLower())
		if !ok {
			return nil, ctx.Errorf("Call to undefined method %s::%s()", class.GetName(), r.method)
		}
		callable := phpv.BindClass(member.Method, class, true)
		return phpv.NewZVal(callable), nil
	}

	// Instance method: $obj->method(...)
	refVal, err := r.ref.Run(ctx)
	if err != nil {
		return nil, err
	}

	if r.nullsafe && refVal.IsNull() {
		return phpv.ZNULL.ZVal(), nil
	}

	obj := refVal.AsObject(ctx)
	if obj == nil {
		return nil, ctx.Errorf("Call to a member function %s() on a non-object", r.method)
	}

	class := obj.GetClass()
	member, ok := class.GetMethod(r.method.ToLower())
	if !ok {
		return nil, ctx.Errorf("Call to undefined method %s::%s()", class.GetName(), r.method)
	}

	callable := phpv.Bind(member.Method, obj)
	return phpv.NewZVal(callable), nil
}

func (r *runFirstClassMethodCallable) Dump(w io.Writer) error {
	if err := r.ref.Dump(w); err != nil {
		return err
	}
	if r.static {
		w.Write([]byte("::"))
	} else {
		w.Write([]byte("->"))
	}
	w.Write([]byte(r.method))
	_, err := w.Write([]byte("(...)"))
	return err
}

func compileClassName(c compileCtx) (phpv.ZString, error) {
	var r phpv.ZString
	fullyQualified := false

	i, err := c.NextItem()
	if err != nil {
		return r, err
	}

	if i.Type == tokenizer.T_NS_SEPARATOR {
		fullyQualified = true
		i, err = c.NextItem()
		if err != nil {
			return r, err
		}
	}

	for {
		if i.Type != tokenizer.T_STRING {
			return r, i.Unexpected()
		}

		r = r + phpv.ZString(i.Data)

		i, err = c.NextItem()
		if err != nil {
			return r, err
		}
		if i.Type == tokenizer.T_NS_SEPARATOR {
			r = r + "\\"
			// Read the next part after the separator
			i, err = c.NextItem()
			if err != nil {
				return r, err
			}
			continue
		}
		// Not a namespace separator — done
		c.backup()
		if fullyQualified {
			return c.resolveClassName("\\" + r), nil
		}
		return c.resolveClassName(r), nil
	}
}

// expandNewSpreadArgs expands SpreadArg entries in new expression arguments.
// Returns a flat list of Runnables with spread args replaced by individual values.
func expandNewSpreadArgs(ctx phpv.Context, args phpv.Runnables) (phpv.Runnables, error) {
	hasSpread := false
	for _, arg := range args {
		if _, ok := arg.(phpv.NamedArgument); ok {
			continue // named args are not spread args
		}
		if _, ok := arg.(phpv.SpreadArgument); ok {
			hasSpread = true
			break
		}
	}
	if !hasSpread {
		return args, nil
	}

	result := make(phpv.Runnables, 0, len(args))
	for _, arg := range args {
		if _, ok := arg.(phpv.NamedArgument); ok {
			result = append(result, arg)
			continue
		}
		sa, ok := arg.(phpv.SpreadArgument)
		if !ok {
			result = append(result, arg)
			continue
		}
		inner := sa.Inner()
		val, err := inner.Run(ctx)
		if err != nil {
			return nil, err
		}
		if val.GetType() == phpv.ZtArray {
			arr := val.AsArray(ctx)
			for _, v := range arr.Iterate(ctx) {
				result = append(result, &runZVal{v: v.Dup().Value()})
			}
		} else if val.GetType() == phpv.ZtObject {
			obj, ok := val.Value().(*phpobj.ZObject)
			if !ok {
				typeName := "object"
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("Only arrays and Traversables can be unpacked, %s given", typeName))
			}
			if obj.GetClass().Implements(phpobj.IteratorAggregate) {
				iterResult, err := obj.CallMethod(ctx, "getIterator")
				if err != nil {
					return nil, err
				}
				if iterResult != nil && iterResult.GetType() == phpv.ZtObject {
					if iterObj, ok := iterResult.Value().(*phpobj.ZObject); ok && iterObj.GetClass().Implements(phpobj.Iterator) {
						obj = iterObj
					}
				}
			}
			if obj.GetClass().Implements(phpobj.Iterator) {
				obj.CallMethod(ctx, "rewind")
				for {
					v, err := obj.CallMethod(ctx, "valid")
					if err != nil || !v.AsBool(ctx) {
						break
					}
					value, err := obj.CallMethod(ctx, "current")
					if err != nil {
						return nil, err
					}
					result = append(result, &runZVal{v: value.Dup().Value()})
					obj.CallMethod(ctx, "next")
				}
			} else {
				typeName := string(obj.GetClass().GetName())
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("Only arrays and Traversables can be unpacked, %s given", typeName))
			}
		} else {
			typeName := val.GetType().TypeName()
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("Only arrays and Traversables can be unpacked, %s given", typeName))
		}
	}
	return result, nil
}

// phpValueTypeName returns the PHP 8 type name for a value in error messages.
// PHP 8 shows actual values for scalars: "true", "false", "null", "int", "string", "float", "array"
func phpValueTypeName(v *phpv.ZVal) string {
	switch v.GetType() {
	case phpv.ZtNull:
		return "null"
	case phpv.ZtBool:
		if bool(v.Value().(phpv.ZBool)) {
			return "true"
		}
		return "false"
	case phpv.ZtInt:
		return "int"
	case phpv.ZtFloat:
		return "float"
	case phpv.ZtString:
		return "string"
	case phpv.ZtArray:
		return "array"
	case phpv.ZtObject:
		if obj := v.AsObject(nil); obj != nil {
			return string(obj.GetClass().GetName())
		}
		return "object"
	default:
		return v.GetType().String()
	}
}
