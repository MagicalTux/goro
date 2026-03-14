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

	var args []*phpv.ZVal
	var byRefCleanups []*phpv.ZVal
	for i, a := range r.newArg {
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
}

func (r *runNewAnonymousClass) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "new class {...}")
	return err
}

func (r *runNewAnonymousClass) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	ctx.Tick(ctx, r.l)

	// Register the anonymous class
	err := ctx.Global().RegisterClass(r.class.Name, r.class)
	if err != nil {
		return nil, err
	}
	err = r.class.Compile(ctx)
	if err != nil {
		ctx.Global().UnregisterClass(r.class.Name)
		return nil, err
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

		class := classRunnable.(*phpobj.ZClass)
		// Generate unique anonymous class name
		class.Name = phpv.ZString(fmt.Sprintf("class@anonymous%s:%d", n.l.Filename, n.l.Line))

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
	// fetch object
	obj, err := r.ref.Run(ctx)
	if err != nil {
		return nil, err
	}

	if r.nullsafe && obj.GetType() == phpv.ZtNull {
		return phpv.ZNULL.ZVal(), nil
	}

	op := r.op
	if op[0] == '$' {
		// variable
		var opz *phpv.ZVal
		opz, err = ctx.OffsetGet(ctx, op[1:].ZVal())
		if err != nil {
			return nil, err
		}
		opz, err = opz.As(ctx, phpv.ZtString)
		if err != nil {
			return nil, err
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
					return nil, ctx.Errorf("Cannot access self:: when no class scope is active")
				}
			}

		case "parent":
			if ctx.This() != nil {
				if ctx.This().GetClass().GetParent() == nil {
					return nil, ctx.Errorf("Cannot access parent:: when current class scope has no parent")
				}
				objI = ctx.This().GetParent()
				class = objI.GetClass()
			} else {
				// In static context, resolve parent from the closure's class
				selfClass, selfErr := ctx.Global().GetClass(ctx, "self", false)
				if selfErr != nil {
					return nil, ctx.Errorf("Cannot access parent:: when no class scope is active")
				}
				parentClass := selfClass.GetParent()
				if parentClass == nil {
					return nil, ctx.Errorf("Cannot access parent:: when current class scope has no parent")
				}
				class = parentClass
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
		return nil, ctx.Errorf("variable is not an object, cannot call method")
	}

	method, ok := class.GetMethod(op)

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
		// Check for __call magic method on instance calls
		if objI != nil {
			// When using :: syntax, __call should be resolved from $this's
			// actual class (the runtime class), not the named class in the ::
			// expression. For example, if B extends A and $this is B, then
			// A::undefinedMethod() should invoke B::__call, not A::__call.
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
				return ctx.CallZVal(ctx, callMethod.Method, callArgs, callObj)
			}
		}
		return nil, ctx.Errorf("Call to undefined method %s::%s()", class.GetName(), op)
	}

	// Check if method is abstract - cannot be called directly
	if method.Modifiers.Has(phpv.ZAttrAbstract) || (method.Empty && method.Class != nil && method.Class.GetType() != phpv.ZClassTypeInterface) {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot call abstract method %s::%s()", method.Class.GetName(), method.Name))
	}

	// Check method visibility
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
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to private method %s::%s() from %s", methodClassName, method.Name, scope))
		}
	} else if method.Modifiers.Has(phpv.ZAttrProtected) {
		callerClass := ctx.Class()
		if callerClass == nil {
			if method.Name == "__construct" {
				return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to protected %s::__construct() from global scope", class.GetName()))
			}
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to protected method %s::%s() from global scope", class.GetName(), method.Name))
		}
		// Check if caller is in the same class hierarchy
		if !callerClass.InstanceOf(method.Class) && !method.Class.InstanceOf(callerClass) {
			if method.Name == "__construct" {
				return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to protected %s::__construct() from scope %s", class.GetName(), callerClass.GetName()))
			}
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to protected method %s::%s() from scope %s", class.GetName(), method.Name, callerClass.GetName()))
		}
	}

	if objI != nil {
		objI = objI.GetKin(string(method.Class.GetName()))
		class = method.Class
	}

	if objI == nil && r.static {
		// :: is used outside of class context
		if !method.Modifiers.IsStatic() {
			err = ctx.Deprecated("Non-static method %s::%s() should not be called statically", class.GetName(), method.Name, logopt.NoFuncName(true))
			if err != nil {
				return nil, err
			}
		}

		// Use method.Class (defining class) for ctx.Class() so self:: resolves correctly
		bindClass := class
		if method.Class != nil {
			bindClass = method.Class
		}
		m := phpv.BindClass(method.Method, bindClass, true)
		return ctx.Call(ctx, m, r.args, nil)
	}

	if r.static {
		// :: syntax but with an object (e.g., parent::method())
		m := phpv.BindClass(method.Method, class, true)
		return ctx.Call(ctx, m, r.args, objI)
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
		// TODO make this a warning
		return nil, ctx.Errorf("variable is not an object, cannot fetch property")
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
	// write object property
	obj, err := r.ref.Run(ctx)
	if err != nil {
		return err
	}

	objI, ok := obj.Value().(phpv.ZObjectAccess)
	if !ok {
		// TODO cast to object?
		return ctx.Errorf("variable is not an object, cannot set property")
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
		return nil, ctx.Errorf("variable is not an object, cannot fetch property")
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
		return ctx.Errorf("variable is not an object, cannot set property")
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
	methodName := phpv.ZString(name.String())
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
	default:
		return nil, i.Unexpected()
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
		return &runClassStaticVarRef{v, ident[1:], l}, nil

	case tokenizer.T_CLASS:
		// $obj::class or ClassName::class → get class name
		return &runClassNameOf{v, l}, nil

	case tokenizer.T_STRING:
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
		tokenizer.T_PROTECTED, tokenizer.T_PRIVATE:
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
		switch i.Type {
		case tokenizer.T_NS_SEPARATOR:
			r = r + "\\"
		default:
			c.backup()
			if fullyQualified {
				// Already fully qualified, strip leading \ is done by resolve
				return c.resolveClassName("\\" + r), nil
			}
			return c.resolveClassName(r), nil
		}
	}
}
