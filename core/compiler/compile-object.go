package compiler

import (
	"fmt"
	"io"
	"strings"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

// nullSafeChainProducer is implemented by compiled nodes that may produce null
// via the ?-> short-circuit mechanism. When such a node is the ref of a
// subsequent chain operation (->foo(), ->bar, [idx], ::method()), the outer
// operation must also propagate null to implement PHP's nullsafe chain semantics.
type nullSafeChainProducer interface {
	isNullSafeChain() bool
}

// isNullSafeChainRef returns true if the given Runnable may produce null as part
// of a nullsafe (?->) chain, meaning subsequent chained operations should also
// propagate null.
func isNullSafeChainRef(r phpv.Runnable) bool {
	if nsc, ok := r.(nullSafeChainProducer); ok {
		return nsc.isNullSafeChain()
	}
	return false
}

// containsNullSafe checks if an expression contains a nullsafe operator (?->)
// anywhere in its chain. This is used at compile time to detect invalid usage
// of nullsafe in write contexts (assignment, unset, by-ref, etc.).
func containsNullSafe(r phpv.Runnable) bool {
	switch v := r.(type) {
	case *runObjectVar:
		return v.nullsafe || v.nullChain || containsNullSafe(v.ref)
	case *runObjectFunc:
		return v.nullsafe || v.nullChain || containsNullSafe(v.ref)
	case *runObjectDynVar:
		return v.nullsafe || v.nullChain || containsNullSafe(v.ref)
	case *runObjectDynFunc:
		return v.nullsafe || v.nullChain || containsNullSafe(v.ref)
	case *runArrayAccess:
		return v.nullChain || containsNullSafe(v.value)
	case *runNullChainWrap:
		return true
	default:
		return false
	}
}

// wrapNullSafeChain wraps a Runnable in a runNullChainWrap if the ref is a
// nullsafe chain producer. This is used for operations like ::method(), ::$prop
// that follow a nullsafe chain — if the ref resolves to null, the wrapper
// catches it and returns null.
func wrapNullSafeChain(ref phpv.Runnable, result phpv.Runnable) phpv.Runnable {
	if isNullSafeChainRef(ref) {
		return &runNullChainWrap{inner: result, ref: ref}
	}
	return result
}

// runNullChainWrap wraps a Runnable expression that follows a nullsafe chain.
// Before evaluating the inner expression, it evaluates the ref and if it's null,
// short-circuits the entire expression to null.
type runNullChainWrap struct {
	inner phpv.Runnable
	ref   phpv.Runnable // the nullsafe chain producer (for null check)
}

func (r *runNullChainWrap) isNullSafeChain() bool { return true }

func (r *runNullChainWrap) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	// Check if the ref evaluates to null (nullsafe chain short-circuit)
	v, err := r.ref.Run(ctx)
	if err != nil {
		return nil, err
	}
	if v.GetType() == phpv.ZtNull {
		return phpv.ZNULL.ZVal(), nil
	}
	return r.inner.Run(ctx)
}

func (r *runNullChainWrap) Dump(w io.Writer) error {
	return r.inner.Dump(w)
}

type runNewObject struct {
	obj     phpv.ZString
	objSrc  phpv.ZString  // original source name for AST pretty-printing
	cl      phpv.Runnable // for anonymous
	newArg  phpv.Runnables
	l       *phpv.Loc
}

func (*runNewObject) IsFuncCallExpression() {}

func (r *runNewObject) Dump(w io.Writer) error {
	name := r.objSrc
	if name == "" {
		name = r.obj
	}
	_, err := fmt.Fprintf(w, "new %s(", name)
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
		// If the value is an object, use its class name (PHP: new $obj)
		if v.GetType() == phpv.ZtObject {
			if obj, ok := v.Value().(phpv.ZObject); ok {
				className = obj.GetClass().GetName()
			}
		} else {
			className = v.AsString(ctx)
		}
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

	// Reorder named arguments based on constructor parameter positions
	if funcArgs != nil {
		expandedArgs, err = reorderNewNamedArgs(ctx, funcArgs, expandedArgs)
		if err != nil {
			return nil, err
		}
	}

	var args []*phpv.ZVal
	var byRefCleanups []*phpv.ZVal
	for i, a := range expandedArgs {
		// nil entry means a named-argument gap (optional parameter with default value)
		if a == nil {
			args = append(args, phpv.ZNULL.ZVal())
			continue
		}

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
	_, err := w.Write([]byte("new class"))
	if err != nil {
		return err
	}
	// Include constructor args if any
	if len(r.constructorArgs) > 0 {
		_, err = w.Write([]byte("("))
		if err != nil {
			return err
		}
		for i, arg := range r.constructorArgs {
			if i > 0 {
				_, err = w.Write([]byte(", "))
				if err != nil {
					return err
				}
			}
			err = arg.Dump(w)
			if err != nil {
				return err
			}
		}
		_, err = w.Write([]byte(")"))
		if err != nil {
			return err
		}
	}
	// Include extends clause
	if r.class.ExtendsStr != "" {
		_, err = fmt.Fprintf(w, " extends %s", r.class.ExtendsStr)
		if err != nil {
			return err
		}
	}
	// Include implements clause
	if len(r.class.ImplementsStr) > 0 {
		_, err = w.Write([]byte(" implements "))
		if err != nil {
			return err
		}
		for i, iface := range r.class.ImplementsStr {
			if i > 0 {
				_, err = w.Write([]byte(", "))
				if err != nil {
					return err
				}
			}
			_, err = w.Write([]byte(iface))
			if err != nil {
				return err
			}
		}
	}
	// Print the class body (methods and properties from the AST)
	_, err = w.Write([]byte(" {\n}"))
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

	// Determine constructor parameter info for by-ref handling
	var funcArgs []*phpv.FuncArg
	if constructor := getConstructor(r.class); constructor != nil {
		if fga, ok := constructor.(phpv.FuncGetArgs); ok {
			funcArgs = fga.GetArgs()
		}
	}

	// Evaluate constructor arguments, handling by-reference params
	var args []*phpv.ZVal
	var byRefCleanups []*phpv.ZVal
	for i, a := range r.constructorArgs {
		isRefParam := funcArgs != nil && i < len(funcArgs) && funcArgs[i].Ref

		// For by-ref params, enable write context for auto-vivification
		if isRefParam {
			if wcs, ok := a.(phpv.WriteContextSetter); ok {
				wcs.SetWriteContext(true)
			}
		}

		v, err := a.Run(ctx)
		if err != nil {
			if isRefParam {
				if wcs, ok := a.(phpv.WriteContextSetter); ok {
					wcs.SetWriteContext(false)
				}
			}
			return nil, err
		}

		if isRefParam {
			if cw, isCW := a.(phpv.CompoundWritable); isCW && !v.IsRef() {
				// Ensure the element exists (auto-vivification for $undef[0])
				cw.WriteValue(ctx, v.Dup())
				// Re-read to get the actual hash table entry
				v, _ = a.Run(ctx)
				// Make the hash table entry into a reference in-place
				v.MakeRef()
				byRefCleanups = append(byRefCleanups, v)
			}
			if wcs, ok := a.(phpv.WriteContextSetter); ok {
				wcs.SetWriteContext(false)
			}
		}

		args = append(args, v)
	}

	// Create instance
	obj, err := phpobj.NewZObject(ctx, r.class, args...)

	// Unwrap by-ref hash table entries after constructor returns
	for _, ref := range byRefCleanups {
		ref.UnRefIfAlone()
	}

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

	// Handle modifiers before anonymous class: new readonly class { }
	var anonClassAttr phpv.ZClassAttr
	if next.Type == tokenizer.T_READONLY {
		anonClassAttr |= phpv.ZClassReadonly
		next, err = c.NextItem()
		if err != nil {
			return nil, err
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
		// Apply class modifiers from new readonly class { }
		if anonClassAttr != 0 {
			class.Attr |= anonClassAttr
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
		class.Attr |= phpv.ZClassAnon

		return &runNewAnonymousClass{
			class:           class,
			constructorArgs: constructorArgs,
			l:               n.l,
		}, nil
	} else if next.IsSingle('(') {
		// new (EXPR) — dynamic class name from expression (PHP 8.5+)
		expr, err := compileExpr(nil, c)
		if err != nil {
			return nil, err
		}
		closeP, err := c.NextItem()
		if err != nil {
			return nil, err
		}
		if !closeP.IsSingle(')') {
			return nil, closeP.Unexpected()
		}
		n.cl = expr
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
		n.obj, n.objSrc, err = compileClassNameWithSource(c)
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
	ref        phpv.Runnable
	op         phpv.ZString
	args       phpv.Runnables
	l          *phpv.Loc
	static     bool
	nullsafe   bool
	nullChain  bool // propagate null from inner nullsafe chain
	isThisRef  bool // true when the receiver is $this (affects __call vs __callStatic dispatch)
}

func (r *runObjectFunc) isNullSafeChain() bool { return r.nullsafe || r.nullChain }

func (*runObjectFunc) IsFuncCallExpression() {}

// isThisVariable checks if a Runnable is a $this variable reference
func isThisVariable(r phpv.Runnable) bool {
	if rv, ok := r.(*runVariable); ok && rv.v == "this" {
		return true
	}
	return false
}

type runObjectVar struct {
	ref              phpv.Runnable
	varName          phpv.ZString
	l                *phpv.Loc
	writeContext     bool // set when reading as part of a write chain (suppress undefined property warnings)
	compoundWriteCtx bool // set for compound assignment (+=, etc.) on null receiver
	incDecCtx        bool // set when in a ++/-- context (for "Attempt to increment/decrement property" error)
	nullsafe         bool
	nullChain        bool // propagate null from inner nullsafe chain

	// PrepareWrite caching
	prepared   bool
	cachedProp *phpv.ZVal
}

func (r *runObjectVar) isNullSafeChain() bool { return r.nullsafe || r.nullChain }

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

	if (r.nullsafe || r.nullChain) && obj.GetType() == phpv.ZtNull {
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
		varName := op[1:]
		// Check if the variable is defined - trigger "Undefined variable" warning if not
		opz, varFound, checkErr := ctx.OffsetCheck(ctx, varName.ZVal())
		if checkErr != nil {
			return nil, checkErr
		}
		if !varFound || opz == nil || opz.IsNull() {
			// Variable is undefined - trigger warning (PHP E_WARNING)
			warnErr := ctx.Warn("Undefined variable $%s", varName, logopt.NoFuncName(true))
			if warnErr != nil {
				return nil, warnErr
			}
		}
		if opz == nil {
			opz = phpv.ZNULL.ZVal()
		}
		// Method name must be a string
		if opz.GetType() != phpv.ZtString {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Method name must be a string")
		}
		op = opz.Value().(phpv.ZString)
	}

	var objI phpv.ZObject
	var class phpv.ZClass
	objFromVariable := false // true when objI came from an object-typed variable (not class name resolution)
	switch obj.GetType() {
	case phpv.ZtObject:
		objI = obj.Value().(*phpobj.ZObject).Unwrap()
		objFromVariable = true
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

		switch strings.ToLower(string(className)) {
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
			// Late static binding: resolve to the runtime class.
			// Unwrap the object to get the actual class (not the narrowed "kin" class)
			// since static:: should always use the most-derived class.
			if ctx.This() != nil {
				objI = ctx.This()
				if uw, ok2 := objI.(interface{ Unwrap() phpv.ZObject }); ok2 {
					objI = uw.Unwrap()
				}
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
		// Cannot call constructor via static syntax without instance context
		if r.static && op.ToLower() == "__construct" && objI == nil {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot call constructor")
		}

		// Check for __invoke method on objects with HandleInvoke
		if objI != nil && op.ToLower() == "__invoke" && class.Handlers() != nil && class.Handlers().HandleInvoke != nil {
			return class.Handlers().HandleInvoke(ctx, objI, r.args)
		}

		// For :: syntax on an object variable that is NOT $this (e.g. $a::method()),
		// __callStatic takes priority over __call.
		if r.static && objFromVariable && !r.isThisRef {
			callClass := class
			if callStaticMethod, hasCallStatic := callClass.GetMethod("__callstatic"); hasCallStatic {
				a := phpv.NewZArray()
				callArgs := []*phpv.ZVal{op.ZVal(), a.ZVal()}
				for _, sub := range r.args {
					var key *phpv.ZVal
					inner := sub
					if na, ok := sub.(phpv.NamedArgument); ok {
						key = na.ArgName().ZVal()
						inner = na.Inner()
					}
					val, err := inner.Run(ctx)
					if err != nil {
						return nil, err
					}
					a.OffsetSet(ctx, key, val)
				}
				SetDeprecationAlias(string(op))
				SetNoDiscardAlias(string(op))
				if err := EmitNoDiscardForMagicCall(ctx, callStaticMethod.Method, callClass.GetName(), string(op)); err != nil {
					return nil, err
				}
				return ctx.CallZVal(ctx, phpv.BindClass(callStaticMethod.Method, callClass, true), callArgs, objI)
			}
		}

		// Check for __call magic method on instance calls.
		// When there's an instance context (objI != nil), __call takes priority
		// over __callStatic for: self::, static::, parent::, $this::, ClassName:: in hierarchy.
		if objI != nil {
			callClass := class
			callObj := objI
			if r.static && ctx.This() != nil {
				callClass = ctx.This().GetClass()
				callObj = ctx.This()
			}
			if callMethod, hasCall := callClass.GetMethod("__call"); hasCall {
				// Evaluate arguments, preserving named arg keys
				a := phpv.NewZArray()
				for _, arg := range r.args {
					var key *phpv.ZVal
					inner := arg
					if na, ok := arg.(phpv.NamedArgument); ok {
						key = na.ArgName().ZVal()
						inner = na.Inner()
					}
					val, err := inner.Run(ctx)
					if err != nil {
						return nil, err
					}
					a.OffsetSet(ctx, key, val.Dup())
				}
				callArgs := []*phpv.ZVal{op.ZVal(), a.ZVal()}
				// Set deprecation/NoDiscard alias so warnings say the called method name
				SetDeprecationAlias(string(op))
				SetNoDiscardAlias(string(op))
				// Emit NoDiscard warning before the call body executes
				if err := EmitNoDiscardForMagicCall(ctx, callMethod.Method, callClass.GetName(), string(op)); err != nil {
					return nil, err
				}
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
					var key *phpv.ZVal
					inner := sub
					if na, ok := sub.(phpv.NamedArgument); ok {
						key = na.ArgName().ZVal()
						inner = na.Inner()
					}
					val, err := inner.Run(ctx)
					if err != nil {
						return nil, err
					}
					a.OffsetSet(ctx, key, val)
				}
				// Set deprecation/NoDiscard alias so warnings say the called method name
				SetDeprecationAlias(string(op))
				SetNoDiscardAlias(string(op))
				if err := EmitNoDiscardForMagicCall(ctx, callStaticMethod.Method, callClass.GetName(), string(op)); err != nil {
					return nil, err
				}
				// Wrap in MethodCallable so stack trace shows class and :: type
				return ctx.CallZVal(ctx, phpv.BindClass(callStaticMethod.Method, callClass, true), callArgs, objI)
			}
		}
		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to undefined method %s::%s()", class.GetName(), op))
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
		} else if !callerClass.InstanceOf(method.Class) && !method.Class.InstanceOf(callerClass) && !callerClass.InstanceOf(class) && !class.InstanceOf(callerClass) {
			// Also check if caller and target share a common ancestor that declares this method.
			// This handles sibling classes (e.g., B1 and B2 both extend A, B2 calls B1::method
			// where method was originally declared in A).
			protectedVisible := false
			if method.Class != nil {
				rootClass := method.Class
				for rootClass.GetParent() != nil {
					if pm, ok := rootClass.GetParent().GetMethod(method.Name); ok && pm.Modifiers.Has(phpv.ZAttrProtected) {
						rootClass = rootClass.GetParent()
					} else {
						break
					}
				}
				if callerClass.InstanceOf(rootClass) {
					protectedVisible = true
				}
			}
			if !protectedVisible {
				if method.Name == "__construct" {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to protected %s::__construct() from scope %s", class.GetName(), callerClass.GetName()))
				}
				methodNotVisible = true
				visErrMsg = fmt.Sprintf("Call to protected method %s::%s() from scope %s", class.GetName(), method.Name, callerClass.GetName())
			}
		}
	}
	if methodNotVisible {
		// For static calls without instance, check __callStatic first
		if r.static && objI == nil {
			if callStaticMethod, hasCallStatic := class.GetMethod("__callstatic"); hasCallStatic {
				a := phpv.NewZArray()
				callArgs := []*phpv.ZVal{op.ZVal(), a.ZVal()}
				for _, sub := range r.args {
					val, err := sub.Run(ctx)
					if err != nil {
						return nil, err
					}
					a.OffsetSet(ctx, nil, val)
				}
				SetDeprecationAlias(string(op))
				SetNoDiscardAlias(string(op))
				if err := EmitNoDiscardForMagicCall(ctx, callStaticMethod.Method, class.GetName(), string(op)); err != nil {
					return nil, err
				}
				return ctx.CallZVal(ctx, phpv.BindClass(callStaticMethod.Method, class, true), callArgs, nil)
			}
		}
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
				SetNoDiscardAlias(string(op))
				if err := EmitNoDiscardForMagicCall(ctx, callMethod.Method, callClass.GetName(), string(op)); err != nil {
					return nil, err
				}
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
		// For trait-aliased methods, preserve the alias name in the stack trace
		calledName := string(op)
		if m.Callable.Name() != calledName {
			m.AliasName = calledName
		}
		return ctx.Call(ctx, m, r.args, nil)
	}

	if r.static {
		// :: syntax but with an object (e.g., parent::method(), self::method())
		// Not truly static: $this is forwarded, so use Static=false for the binding.
		// Preserve the CalledClass from the current context for late static binding (LSB).
		var calledClass phpv.ZClass
		if fc, ok := ctx.(interface{ CalledClass() phpv.ZClass }); ok {
			calledClass = fc.CalledClass()
		}
		m := phpv.BindClassLSB(method.Method, class, calledClass, false)
		// For trait-aliased methods, preserve the alias name in the stack trace
		calledName := string(op)
		if m.Callable.Name() != calledName {
			m.AliasName = calledName
		}
		return ctx.Call(ctx, m, r.args, objI)
	}

	// Static methods don't get $this even when called via instance ($obj->staticMethod())
	// Use BindClassLSB to preserve the runtime class for late static binding (get_called_class)
	if method.Modifiers.IsStatic() {
		m := phpv.BindClassLSB(method.Method, class, runtimeClass, true)
		return ctx.Call(ctx, m, r.args, nil)
	}

	// For Closure::__invoke() calls (e.g. $closure->__invoke($arg) or $closure->__INVOKE($arg)),
	// delegate to the HandleInvoke handler so that by-ref parameters work correctly.
	// The NativeMethod wrapper for __invoke doesn't carry parameter info.
	if method.Name.ToLower() == "__invoke" && class.Handlers() != nil && class.Handlers().HandleInvoke != nil {
		return class.Handlers().HandleInvoke(ctx, objI, r.args)
	}

	// For trait methods, the callable (ZClosure) may report the trait class via GetClass(),
	// which overrides the narrowed $this class in callZValImpl. Wrap in a MethodCallable
	// so the class is set explicitly to the declaring class.
	// Also, for trait-aliased methods, the callable's Name() returns the original trait name,
	// so we set AliasName to the called name.
	callable := phpv.Callable(method.Method)
	if method.FromTrait != nil || method.Method.Name() != string(op) {
		mc := phpv.BindClass(method.Method, class, false)
		if method.Method.Name() != string(op) {
			mc.AliasName = string(op)
		}
		callable = mc
	}

	return ctx.Call(ctx, callable, r.args, objI)
}

func (r *runObjectVar) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	// fetch object property
	obj, err := r.ref.Run(ctx)
	if err != nil {
		return nil, err
	}

	if (r.nullsafe || r.nullChain) && obj.GetType() == phpv.ZtNull {
		return phpv.ZNULL.ZVal(), nil
	}

	objI, ok := obj.Value().(phpv.ZObjectAccess)
	if !ok {
		typeName := phpValueTypeName(obj)
		if r.compoundWriteCtx {
			// PHP 8: compound assignment (+=, -=, /=, etc.) on null receiver
			return nil, phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Attempt to modify property \"%s\" on %s", r.varName, typeName))
		}
		if r.writeContext {
			// PHP 8: modifying property of non-object in a write chain throws Error
			return nil, phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Attempt to modify property \"%s\" on %s", r.varName, typeName))
		}
		// PHP 8: check if this is a ++/-- context
		if r.incDecCtx {
			return nil, phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Attempt to increment/decrement property \"%s\" on %s", r.varName, typeName))
		}
		// PHP 8: reading property of non-object is a warning, returns null
		ctx.Warn("Attempt to read property \"%s\" on %s", r.varName, typeName, logopt.NoFuncName(true))
		return phpv.ZNULL.ZVal(), nil
	}

	// offset get
	var offt *phpv.ZVal
	if r.prepared && r.cachedProp != nil {
		offt = r.cachedProp
		// Don't consume the cache - it may be needed by WriteValue later
	} else if r.varName[0] == '$' {
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
				// Before auto-vivifying, check asymmetric visibility.
				// If the property has private(set) or protected(set) and we're
				// outside the allowed scope, the auto-vivification would be
				// an indirect modification.
				if zobj, ok2 := objI.(*phpobj.ZObject); ok2 {
					propName := r.varName
					if len(propName) > 0 && propName[0] != '$' {
						if err := checkAsymmetricVisibilityIndirect(ctx, zobj, propName); err != nil {
							return nil, err
						}
					}
				}
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

// CheckReadonlyRef checks if creating a reference to this object property would
// violate readonly or asymmetric visibility constraints.
func (r *runObjectVar) CheckReadonlyRef(ctx phpv.Context) error {
	obj, err := r.ref.Run(ctx)
	if err != nil {
		return nil // don't block on evaluation errors
	}
	if obj.GetType() != phpv.ZtObject {
		return nil
	}
	zobj, ok := obj.Value().(*phpobj.ZObject)
	if !ok {
		return nil
	}
	propName := r.varName
	if len(propName) > 0 && propName[0] == '$' {
		// Dynamic property name - skip check
		return nil
	}
	if zobj.IsReadonlyProperty(propName) && zobj.IsReadonlyPropertyInitialized(propName) {
		return phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Cannot indirectly modify readonly property %s::$%s", zobj.GetClass().GetName(), propName))
	}
	// Check asymmetric visibility for reference creation.
	// For object-typed properties, PHP returns a copy of the object handle
	// instead of throwing, so the reference just points to a local copy.
	// We only throw for non-object property types.
	if prop := zobj.FindDeclaredProp(propName); prop != nil && prop.TypeHint != nil {
		if prop.TypeHint.Type() == phpv.ZtObject || prop.TypeHint.ClassName() != "" {
			// Object-typed property with private(set): return copy, not error
			return nil
		}
	}
	if err := checkAsymmetricVisibilityIndirect(ctx, zobj, propName); err != nil {
		return err
	}
	return nil
}

func (r *runObjectVar) SetWriteContext(v bool) {
	r.writeContext = v
}

func (r *runObjectVar) WriteValue(ctx phpv.Context, value *phpv.ZVal) error {
	// Set write context so that child runVariable (the receiver) suppresses
	// "Undefined variable" warnings — PHP only emits the property-level error.
	r.writeContext = true
	// Set write context on the ref chain so intermediate property accesses
	// produce "Attempt to modify property" errors instead of "Attempt to read" warnings.
	if wcs, ok := r.ref.(phpv.WriteContextSetter); ok {
		wcs.SetWriteContext(true)
	}
	// write object property
	obj, err := r.ref.Run(ctx)
	r.writeContext = false
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
		verb := "assign"
		if value != nil && value.IsRef() {
			verb = "modify"
		}
		return phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Attempt to %s property \"%s\" on %s", verb, r.varName, typeName))
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
	ref       phpv.Runnable
	nameExpr  phpv.Runnable
	l         *phpv.Loc
	nullsafe  bool
	nullChain bool // propagate null from inner nullsafe chain

	// PrepareWrite caching
	prepared   bool
	cachedName *phpv.ZVal
}

func (r *runObjectDynVar) isNullSafeChain() bool { return r.nullsafe || r.nullChain }

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
	if (r.nullsafe || r.nullChain) && obj.GetType() == phpv.ZtNull {
		return phpv.ZNULL.ZVal(), nil
	}
	objI, ok := obj.Value().(phpv.ZObjectAccess)
	if !ok {
		// Evaluate the name expression first so we can include it in the warning
		var name *phpv.ZVal
		var nameErr error
		if r.prepared && r.cachedName != nil {
			name = r.cachedName
		} else {
			name, nameErr = r.nameExpr.Run(ctx)
		}
		typeName := phpValueTypeName(obj)
		if nameErr == nil && name != nil {
			ctx.Warn("Attempt to read property \"%s\" on %s", name.String(), typeName, logopt.NoFuncName(true))
		} else {
			ctx.Warn("Attempt to read property on %s", typeName, logopt.NoFuncName(true))
		}
		return phpv.ZNULL.ZVal(), nil
	}
	// Use cached name from PrepareWrite if available (e.g. ??= memoization)
	var name *phpv.ZVal
	if r.prepared && r.cachedName != nil {
		name = r.cachedName
		// Don't consume the cache - it may be needed by WriteValue later
	} else {
		name, err = r.nameExpr.Run(ctx)
		if err != nil {
			return nil, err
		}
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
	ref       phpv.Runnable
	nameExpr  phpv.Runnable
	args      []phpv.Runnable
	l         *phpv.Loc
	nullsafe  bool
	nullChain bool // propagate null from inner nullsafe chain
	static    bool
}

func (r *runObjectDynFunc) isNullSafeChain() bool { return r.nullsafe || r.nullChain }

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
	if (r.nullsafe || r.nullChain) && obj.GetType() == phpv.ZtNull {
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

		switch strings.ToLower(string(className)) {
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
			// Late static binding: unwrap to get the actual runtime class
			if ctx.This() != nil {
				objI = ctx.This()
				if uw, ok2 := objI.(interface{ Unwrap() phpv.ZObject }); ok2 {
					objI = uw.Unwrap()
				}
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
				SetDeprecationAlias(string(methodName))
				SetNoDiscardAlias(string(methodName))
				if err := EmitNoDiscardForMagicCall(ctx, method.Method, class.GetName(), string(methodName)); err != nil {
					return nil, err
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
					SetDeprecationAlias(string(methodName))
					SetNoDiscardAlias(string(methodName))
					if err := EmitNoDiscardForMagicCall(ctx, method.Method, class.GetName(), string(methodName)); err != nil {
						return nil, err
					}
					return ctx.CallZVal(ctx, method.Method, callArgs, objI)
				}
			}
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to undefined method %s::%s()", class.GetName(), methodName))
		}
		return ctx.Call(ctx, method.Method, r.args, objI)
	}

	objZ := obj.AsObject(ctx)
	if objZ == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to a member function %s() on null", methodName))
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
			SetDeprecationAlias(string(methodName))
			SetNoDiscardAlias(string(methodName))
			if err := EmitNoDiscardForMagicCall(ctx, method.Method, objZ.GetClass().GetName(), string(methodName)); err != nil {
				return nil, err
			}
			return ctx.CallZVal(ctx, method.Method, callArgs, objZ)
		}
		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to undefined method %s::%s()", objZ.GetClass().GetName(), methodName))
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
		// C::${expr}, C::$var, or C::$$var() — dynamic static property/method access
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
		// C::$var or C::$$var — indirect via variable
		// The first $ was already consumed. If the next token is T_VARIABLE,
		// this is $$var (variable variable). Wrap in a variable ref.
		c.backup()
		var expr phpv.Runnable
		// Use compileRunVariableRef which handles $$var correctly
		expr, err = compileRunVariableRef(nil, c, l)
		if err != nil {
			return nil, err
		}
		// Check if followed by ( — dynamic method call: C::$$var()
		peek, peekErr := c.NextItem()
		if peekErr != nil {
			return nil, peekErr
		}
		if peek.IsSingle('(') {
			c.backup()
			args, err := compileFuncPassedArgs(c)
			if err != nil {
				return nil, err
			}
			return &runObjectDynFunc{ref: v, nameExpr: expr, l: l, static: true, args: args}, nil
		}
		c.backup()
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
			return &runObjectFunc{ref: v, op: ident, args: args, l: l, static: true, isThisRef: isThisVariable(v)}, err
		}

		// Check for parent::$prop::get()/::set() — property hook parent call
		if i.Type == tokenizer.T_PAAMAYIM_NEKUDOTAYIM {
			// Check if v is "parent"
			isParent := false
			if zv, ok := v.(*runZVal); ok {
				if s, ok2 := zv.v.(phpv.ZString); ok2 && strings.EqualFold(string(s), "parent") {
					isParent = true
				}
			}
			if isParent {
				// Look for get/set identifier
				hookItem, err := c.NextItem()
				if err != nil {
					return nil, err
				}
				if hookItem.Type == tokenizer.T_STRING {
					hookName := strings.ToLower(hookItem.Data)
					if hookName == "get" || hookName == "set" {
						propName := phpv.ZString(ident[1:]) // strip $ prefix

						// Compile-time validation: must be inside a class context
						if c.getClass() == nil {
							return nil, &phpv.PhpError{
								Err:  fmt.Errorf("Cannot use \"parent\" when no class scope is active"),
								Code: phpv.E_COMPILE_ERROR,
								Loc:  l,
							}
						}

						// Compile-time validation: must be inside a property hook
						hookFunc := c.getFunc()
						if hookFunc == nil {
							return nil, &phpv.PhpError{
								Err:  fmt.Errorf("Must not use parent::$%s::%s() outside a property hook", propName, hookName),
								Code: phpv.E_COMPILE_ERROR,
								Loc:  l,
							}
						}
						funcName := string(hookFunc.name)
						// Hook functions are named "$propName::get" or "$propName::set"
						if !strings.HasPrefix(funcName, "$") || !strings.Contains(funcName, "::") {
							return nil, &phpv.PhpError{
								Err:  fmt.Errorf("Must not use parent::$%s::%s() outside a property hook", propName, hookName),
								Code: phpv.E_COMPILE_ERROR,
								Loc:  l,
							}
						}
						parts := strings.SplitN(funcName[1:], "::", 2) // strip $ and split
						currentProp := parts[0]
						currentHook := parts[1]

						// Must be same property
						if currentProp != string(propName) {
							return nil, &phpv.PhpError{
								Err:  fmt.Errorf("Must not use parent::$%s::%s() in a different property ($%s)", propName, hookName, currentProp),
								Code: phpv.E_COMPILE_ERROR,
								Loc:  l,
							}
						}

						// Must be same hook type
						if currentHook != hookName {
							return nil, &phpv.PhpError{
								Err:  fmt.Errorf("Must not use parent::$%s::%s() in a different property hook (%s)", propName, hookName, currentHook),
								Code: phpv.E_COMPILE_ERROR,
								Loc:  l,
							}
						}

						// Parse arguments: parent::$prop::get() or parent::$prop::set($value)
						hookArgs, err := compileFuncPassedArgs(c)
						if err != nil {
							return nil, err
						}

						return &runParentPropHookCall{
							propName: propName,
							hookType: hookName,
							argExprs: hookArgs,
							l:        l,
						}, nil
					}
				}
				// Not get/set — back up both tokens and fall through to normal static var ref
				c.backup() // back up the hookItem
				c.backup() // back up the ::
				return &runClassStaticVarRef{v, ident[1:], l}, nil
			}
			// Not parent — back up the :: and fall through
			c.backup()
			return &runClassStaticVarRef{v, ident[1:], l}, nil
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
		// Dynamic class constant fetch: C::{expr}
		return &runClassDynConst{className: v, nameExpr: expr, l: l}, nil

	case tokenizer.T_CLASS:
		// Check if followed by ( — if so, this is a static method call Obj::class()
		// not the ::class name fetch
		peek, peekErr := c.NextItem()
		if peekErr != nil {
			return nil, peekErr
		}
		if peek.IsSingle('(') {
			c.backup()
			args, err := compileFuncPassedArgs(c)
			if err != nil {
				return nil, err
			}
			if IsFirstClassCallable(args) {
				return &runFirstClassMethodCallable{ref: v, method: ident, static: true, l: l}, nil
			}
			return &runObjectFunc{ref: v, op: ident, args: args, l: l, static: true, isThisRef: isThisVariable(v)}, err
		}
		c.backup()
		// $obj::class or ClassName::class → get class name
		return &runClassNameOf{className: v, l: l}, nil

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
			return &runObjectFunc{ref: v, op: ident, args: args, l: l, static: true, isThisRef: isThisVariable(v)}, err
		default:
			c.backup()
			return &runClassStaticObjRef{v, ident, l}, nil
		}
	}
}

func compileObjectOperator(v phpv.Runnable, i *tokenizer.Item, c compileCtx, nullsafe bool) (phpv.Runnable, error) {
	// call a method or get a variable on an object
	l := i.Loc()

	// Determine if this operation is part of a nullsafe chain.
	// If not explicitly nullsafe but the ref is a nullsafe chain producer,
	// propagate the null-chain flag so the entire chain short-circuits.
	chainProp := !nullsafe && isNullSafeChainRef(v)

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
				dynFunc := &runObjectDynFunc{ref: v, nameExpr: varRef, l: l, nullsafe: nullsafe, nullChain: chainProp}
				dynFunc.args, err = compileFuncPassedArgs(c)
				return dynFunc, err
			}
			return &runObjectDynVar{ref: v, nameExpr: varRef, l: l, nullsafe: nullsafe, nullChain: chainProp}, nil
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
			dynFunc := &runObjectDynFunc{ref: v, nameExpr: expr, l: l, nullsafe: nullsafe, nullChain: chainProp}
			dynFunc.args, err = compileFuncPassedArgs(c)
			return dynFunc, err
		}
		return &runObjectDynVar{ref: v, nameExpr: expr, l: l, nullsafe: nullsafe, nullChain: chainProp}, nil
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
			dynFunc := &runObjectDynFunc{ref: v, nameExpr: expr, l: l, nullsafe: nullsafe, nullChain: chainProp}
			dynFunc.args, err = compileFuncPassedArgs(c)
			if err != nil {
				return nil, err
			}
			if IsFirstClassCallable(dynFunc.args) {
				return &runFirstClassDynMethodCallable{ref: v, nameExpr: expr, l: l}, nil
			}
			return dynFunc, nil
		}
		// Dynamic property access: $obj->{expr}
		return &runObjectDynVar{ref: v, nameExpr: expr, l: l, nullsafe: nullsafe, nullChain: chainProp}, nil
	case tokenizer.T_VARIABLE:
		// dynamic member access (handled below)
	default:
		// All semi-reserved keywords are valid after -> ($obj->exit(), $obj->trait, etc.)
		if !i.IsSemiReserved() {
			return nil, i.Unexpected()
		}
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
		return &runObjectFunc{ref: v, op: op, args: args, l: l, nullsafe: nullsafe, nullChain: chainProp}, nil
	}

	return &runObjectVar{ref: v, varName: op, l: l, nullsafe: nullsafe, nullChain: chainProp}, nil
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
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to undefined method %s::%s()", class.GetName(), r.method))
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
		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to a member function %s() on null", r.method))
	}

	class := obj.GetClass()
	_, ok := class.GetMethod(r.method.ToLower())
	if !ok {
		// Check for __call magic method
		if callMethod, hasCall := class.GetMethod("__call"); hasCall {
			w := &wrappedClosure{
				inner: &magicCallClosure{callMethod: callMethod.Method, methodName: r.method, instance: obj},
				name:  phpv.ZString(string(class.GetName()) + "::__call"),
				this:  obj,
				class: class,
			}
			return w.Spawn(ctx)
		}
		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to undefined method %s::%s()", class.GetName(), r.method))
	}

	// Build a proper Closure object via closureFromCallable so that
	// ->bindTo() and other Closure methods work correctly.
	arr := phpv.NewZArray()
	arr.OffsetSet(ctx, phpv.ZInt(0), obj.ZVal())
	arr.OffsetSet(ctx, phpv.ZInt(1), r.method.ZVal())
	return closureFromCallable(ctx, arr.ZVal())
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

// runFirstClassDynMethodCallable implements $obj->{expr}(...) first-class callable syntax.
type runFirstClassDynMethodCallable struct {
	ref      phpv.Runnable
	nameExpr phpv.Runnable
	l        *phpv.Loc
}

func (r *runFirstClassDynMethodCallable) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	if err := ctx.Tick(ctx, r.l); err != nil {
		return nil, err
	}

	refVal, err := r.ref.Run(ctx)
	if err != nil {
		return nil, err
	}

	nameVal, err := r.nameExpr.Run(ctx)
	if err != nil {
		return nil, err
	}

	obj := refVal.AsObject(ctx)
	if obj == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Call to a member function on non-object")
	}

	methodName := nameVal.AsString(ctx)

	// Build a Closure via closureFromCallable
	arr := phpv.NewZArray()
	arr.OffsetSet(ctx, phpv.ZInt(0), obj.ZVal())
	arr.OffsetSet(ctx, phpv.ZInt(1), methodName.ZVal())
	return closureFromCallable(ctx, arr.ZVal())
}

func (r *runFirstClassDynMethodCallable) Dump(w io.Writer) error {
	if err := r.ref.Dump(w); err != nil {
		return err
	}
	w.Write([]byte("->{"))
	if err := r.nameExpr.Dump(w); err != nil {
		return err
	}
	_, err := w.Write([]byte("}(...)"))
	return err
}

func compileClassName(c compileCtx) (phpv.ZString, error) {
	resolved, _, err := compileClassNameWithSource(c)
	return resolved, err
}

// compileClassNameWithSource parses a class name and returns both the resolved
// name (for runtime use) and the source name as written (for AST pretty-printing).
func compileClassNameWithSource(c compileCtx) (phpv.ZString, phpv.ZString, error) {
	var r phpv.ZString
	fullyQualified := false

	i, err := c.NextItem()
	if err != nil {
		return r, r, err
	}

	if i.Type == tokenizer.T_NS_SEPARATOR {
		fullyQualified = true
		i, err = c.NextItem()
		if err != nil {
			return r, r, err
		}
	}

	for {
		// Semi-reserved keywords (like 'enum') can be used as class names
		if i.Type != tokenizer.T_STRING && !i.IsSemiReserved() {
			return r, r, i.Unexpected()
		}

		r = r + phpv.ZString(i.Data)

		i, err = c.NextItem()
		if err != nil {
			return r, r, err
		}
		if i.Type == tokenizer.T_NS_SEPARATOR {
			r = r + "\\"
			// Read the next part after the separator
			i, err = c.NextItem()
			if err != nil {
				return r, r, err
			}
			continue
		}
		// Not a namespace separator — done
		c.backup()
		if fullyQualified {
			return c.resolveClassName("\\" + r), "\\" + r, nil
		}
		return c.resolveClassName(r), r, nil
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

// reorderNewNamedArgs reorders arguments for new expressions based on constructor
// parameter positions. Named arguments are placed at their parameter position,
// while positional arguments remain in order. Returns an error for duplicate or
// unknown named parameters.
func reorderNewNamedArgs(ctx phpv.Context, funcArgs []*phpv.FuncArg, args phpv.Runnables) (phpv.Runnables, error) {
	// Check if any args are named
	hasNamed := false
	for _, arg := range args {
		if _, ok := arg.(phpv.NamedArgument); ok {
			hasNamed = true
			break
		}
	}
	if !hasNamed {
		return args, nil
	}

	// Build result array sized to max(len(funcArgs), len(args))
	size := len(funcArgs)
	if len(args) > size {
		size = len(args)
	}
	result := make(phpv.Runnables, size)
	filled := make([]bool, size)

	// Place positional arguments first
	positionalEnd := 0
	for i, arg := range args {
		if _, ok := arg.(phpv.NamedArgument); ok {
			break
		}
		if i < size {
			result[i] = arg
			filled[i] = true
		}
		positionalEnd = i + 1
	}

	// Check if the last funcArg is variadic
	hasVariadic := false
	if len(funcArgs) > 0 {
		hasVariadic = funcArgs[len(funcArgs)-1].Variadic
	}

	// Place named arguments at their parameter positions
	for _, arg := range args[positionalEnd:] {
		na, ok := arg.(phpv.NamedArgument)
		if !ok {
			continue
		}
		name := na.ArgName()
		found := false
		for j, fa := range funcArgs {
			if fa.VarName == name {
				if filled[j] {
					return nil, phpobj.ThrowError(ctx, phpobj.Error,
						fmt.Sprintf("Named parameter $%s overwrites previous argument", name))
				}
				result[j] = arg
				filled[j] = true
				found = true
				break
			}
		}
		if !found {
			if hasVariadic {
				result = append(result, arg)
			} else {
				return nil, phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("Unknown named parameter $%s", name))
			}
		}
	}

	// Trim trailing nil entries
	for len(result) > 0 && result[len(result)-1] == nil {
		result = result[:len(result)-1]
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
