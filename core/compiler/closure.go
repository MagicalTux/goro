package compiler

import (
	"fmt"
	"io"
	"strings"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

type ZClosure struct {
	phpv.CallableVal
	name          phpv.ZString
	enclosingFunc string         // enclosing function/method name for closure naming (PHP 8.4+)
	args          []*phpv.FuncArg
	use           []*phpv.FuncUse
	code          phpv.Runnable
	class         phpv.ZClass    // class in which this closure was defined (for parent:: and self::)
	calledClass   phpv.ZClass    // called class for late static binding (static::class)
	this          phpv.ZObject   // captured $this from enclosing method (nil for static closures and free functions)
	start         *phpv.Loc
	end           *phpv.Loc
	rref          bool // return ref?
	isStatic      bool // true for static function() {} and static fn() =>
	isArrow       bool // true for fn() => expr (arrow function)
	isGenerator   bool // true if this function contains yield
	usesThis      bool // true if the closure body references $this
	attributes    []*phpv.ZAttribute // PHP 8.0 attributes on this function
	returnType    *phpv.TypeHint     // return type declaration (nil if none)
}

// > class Closure
var Closure = &phpobj.ZClass{
	Name:         "Closure",
	H:            &phpv.ZClassHandlers{},
	InternalOnly: true,
}

// wrappedClosure wraps an arbitrary Callable as a Closure object opaque.
// Used by Closure::fromCallable() to wrap non-closure callables.
type wrappedClosure struct {
	phpv.CallableVal
	inner        phpv.Callable
	name         phpv.ZString
	args         []*phpv.FuncArg
	this         phpv.ZObject
	class        phpv.ZClass
	fromFunction bool // wrapping a named function (not a method)
	fromMethod   bool // wrapping a class method
	isStaticW    bool // true if wrapping a static method
}

func (w *wrappedClosure) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return w.inner.Call(ctx, args)
}

func (w *wrappedClosure) GetArgs() []*phpv.FuncArg {
	return w.args
}

func (w *wrappedClosure) Name() string {
	return string(w.name)
}

func (w *wrappedClosure) IsStatic() bool {
	return w.isStaticW
}

func (w *wrappedClosure) GetThis() phpv.ZObject {
	return w.this
}

func (w *wrappedClosure) GetClass() phpv.ZClass {
	return w.class
}

func (w *wrappedClosure) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	return w.Spawn(ctx)
}

func (w *wrappedClosure) Dump(wr io.Writer) error {
	_, err := fmt.Fprintf(wr, "/* wrapped closure: %s */", w.name)
	return err
}

func (w *wrappedClosure) GetAttributes() []*phpv.ZAttribute {
	if ag, ok := w.inner.(phpv.AttributeGetter); ok { return ag.GetAttributes() }
	return nil
}

func (w *wrappedClosure) Spawn(ctx phpv.Context) (*phpv.ZVal, error) {
	o, err := phpobj.NewZObjectOpaque(ctx, Closure, w)
	if err != nil {
		return nil, err
	}
	return o.ZVal(), nil
}

// isInternalClass returns true if the class is built into the engine (not user-defined).
func isInternalClass(class phpv.ZClass) bool {
	if zc, ok := class.(*phpobj.ZClass); ok {
		return zc.L == nil
	}
	return false
}

func init() {
	// put this here to avoid initialization loop problem
	Closure.H.HandleInvoke = func(ctx phpv.Context, o phpv.ZObject, args []phpv.Runnable) (*phpv.ZVal, error) {
		opaque := o.GetOpaque(Closure)
		var z *ZClosure
		var callable phpv.Callable
		switch v := opaque.(type) {
		case *ZClosure:
			z = v
			callable = v
		case *generatorClosure:
			z = v.ZClosure
			callable = v
		case *wrappedClosure:
			if v.this != nil {
				return ctx.Call(ctx, v, args, v.this)
			}
			return ctx.Call(ctx, v, args, nil)
		default:
			return nil, fmt.Errorf("invalid closure opaque type: %T", opaque)
		}
		// Use the captured $this if available (closure defined in a class method)
		if z.this != nil {
			return ctx.Call(ctx, callable, args, z.this)
		}
		// For closures without $this, don't pass anything as $this
		return ctx.Call(ctx, callable, args, nil)
	}

	// Closure comparison handler: two Closure objects are equal only if they
	// wrap the same underlying callable with the same bound $this and scope.
	Closure.H.HandleCompare = func(ctx phpv.Context, a, b phpv.ZObject) (int, error) {
		opaqueA := a.GetOpaque(Closure)
		opaqueB := b.GetOpaque(Closure)

		if opaqueA == nil || opaqueB == nil {
			if opaqueA == opaqueB {
				return 0, nil
			}
			return 1, nil
		}

		// Compare wrappedClosure (from Closure::fromCallable)
		wA, okA := opaqueA.(*wrappedClosure)
		wB, okB := opaqueB.(*wrappedClosure)
		if okA && okB {
			// Same name, same $this, same class → equal
			if wA.name != wB.name {
				return 1, nil
			}
			if wA.this != wB.this {
				return 1, nil
			}
			if wA.class != wB.class {
				return 1, nil
			}
			// Compare inner callable identity for __call/__callStatic wrappers
			if mcA, ok1 := wA.inner.(*magicCallClosure); ok1 {
				if mcB, ok2 := wB.inner.(*magicCallClosure); ok2 {
					if mcA.methodName != mcB.methodName {
						return 1, nil
					}
				} else {
					return 1, nil
				}
			}
			if mcA, ok1 := wA.inner.(*magicCallStaticClosure); ok1 {
				if mcB, ok2 := wB.inner.(*magicCallStaticClosure); ok2 {
					if mcA.methodName != mcB.methodName {
						return 1, nil
					}
				} else {
					return 1, nil
				}
			}
			return 0, nil
		}

		// Compare ZClosure
		zA, okZA := opaqueA.(*ZClosure)
		zB, okZB := opaqueB.(*ZClosure)
		if okZA && okZB {
			if zA.code != zB.code {
				return 1, nil
			}
			if zA.this != zB.this {
				return 1, nil
			}
			if zA.class != zB.class {
				return 1, nil
			}
			return 0, nil
		}

		// Different types of opaque → not equal
		return 1, nil
	}

	// Closure::bind() - static method
	Closure.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"bind": {
			Name:      "bind",
			Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
			Method: phpobj.NativeStaticMethod(func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return closureBind(ctx, args)
			}),
		},
		"bindto": {
			Name:      "bindTo",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				// bindTo is an instance method: $closure->bindTo($newThis, $newScope)
				// Prepend $this closure as first arg
				allArgs := make([]*phpv.ZVal, 0, len(args)+1)
				allArgs = append(allArgs, o.ZVal())
				allArgs = append(allArgs, args...)
				return closureBind(ctx, allArgs)
			}),
		},
		"fromcallable": {
			Name:      "fromCallable",
			Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
			Method: phpobj.NativeStaticMethod(func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) < 1 {
					return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "Closure::fromCallable() expects exactly 1 argument, 0 given")
				}
				return closureFromCallable(ctx, args[0])
			}),
		},
		"call": {
			Name:      "call",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) < 1 {
					return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "Closure::call() expects at least 1 argument, 0 given")
				}
				opaque := o.GetOpaque(Closure)
				if opaque == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, "Closure::call(): internal error - not a closure")
				}

				// First arg is newThis, rest are call args
				newThis := args[0]
				callArgs := args[1:]

				// Handle wrappedClosure (from Closure::fromCallable)
				if w, ok := opaque.(*wrappedClosure); ok {
					var thisObj phpv.ZObject
					if newThis.GetType() == phpv.ZtObject {
						if obj, ok2 := newThis.Value().(phpv.ZObject); ok2 {
							thisObj = obj
						}
					}

					// Check for incompatible method binding
					if w.fromMethod && thisObj != nil && w.class != nil {
						if !thisObj.GetClass().InstanceOf(w.class) {
							ctx.Warn("Cannot bind method %s() to object of class %s, this will be an error in PHP 9",
								w.name, thisObj.GetClass().GetName(), logopt.NoFuncName(true))
							return phpv.ZNULL.ZVal(), nil
						}
					}

					// Check for rebinding scope of function closures
					if w.fromFunction && thisObj != nil && !w.fromMethod {
						if isInternalClass(thisObj.GetClass()) {
							ctx.Warn("Cannot bind closure to scope of internal class %s, this will be an error in PHP 9",
								thisObj.GetClass().GetName(), logopt.NoFuncName(true))
							return phpv.ZNULL.ZVal(), nil
						}
						// Calling with an object on a function closure implies binding scope, warn and return null
						ctx.Warn("Cannot rebind scope of closure created from function, this will be an error in PHP 9", logopt.NoFuncName(true))
						return phpv.ZNULL.ZVal(), nil
					}

					if thisObj == nil {
						thisObj = w.this
					}
					if thisObj != nil {
						return ctx.CallZVal(ctx, w.inner, callArgs, thisObj)
					}
					return ctx.CallZVal(ctx, w.inner, callArgs)
				}

				var z *ZClosure
				switch v := opaque.(type) {
				case *ZClosure:
					z = v
				case *generatorClosure:
					z = v.ZClosure
				default:
					return nil, phpobj.ThrowError(ctx, phpobj.Error, "Closure::call(): internal error - unexpected type")
				}

				// Static closures cannot bind $this
				if z.isStatic && newThis.GetType() == phpv.ZtObject {
					ctx.Warn("Cannot bind an instance to a static closure, this will be an error in PHP 9", logopt.NoFuncName(true))
				}

				// Check for binding to internal class scope
				if newThis.GetType() == phpv.ZtObject {
					if obj, ok := newThis.Value().(phpv.ZObject); ok {
						if isInternalClass(obj.GetClass()) {
							ctx.Warn("Cannot bind closure to scope of internal class %s, this will be an error in PHP 9",
								obj.GetClass().GetName(), logopt.NoFuncName(true))
							return phpv.ZNULL.ZVal(), nil
						}
					}
				}

				bound := z.dup()
				if newThis.GetType() == phpv.ZtObject {
					if obj, ok := newThis.Value().(phpv.ZObject); ok {
						bound.this = obj
						bound.class = obj.GetClass()
					}
				}
				if bound.this != nil {
					return ctx.CallZVal(ctx, bound, callArgs, bound.this)
				}
				return ctx.CallZVal(ctx, bound, callArgs)
			}),
		},
		"getcurrent": {
			Name:      "getCurrent",
			Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
			Method: phpobj.NativeStaticMethod(func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return closureGetCurrent(ctx)
			}),
		},
		"__debuginfo": {
			Name:      "__debugInfo",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return closureDebugInfo(ctx, o)
			}),
		},
		"__invoke": {
			Name:      "__invoke",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				// Delegate to the HandleInvoke handler which knows how to call the closure
				if Closure.H != nil && Closure.H.HandleInvoke != nil {
					runnables := make([]phpv.Runnable, len(args))
					for i, a := range args {
						runnables[i] = &zvalRunnable{v: a}
					}
					return Closure.H.HandleInvoke(ctx, o, runnables)
				}
				return nil, fmt.Errorf("Closure::__invoke(): internal error")
			}),
		},
	}
}

// runnableCallableWrapper wraps a Callable as a Runnable code block
type runnableCallableWrapper struct {
	callable phpv.Callable
}

func (r *runnableCallableWrapper) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	// The callable wrapper runs without arguments (they're handled by the call context)
	return r.callable.Call(ctx, nil)
}

func (r *runnableCallableWrapper) Dump(w io.Writer) error {
	_, err := w.Write([]byte("/* callable wrapper */"))
	return err
}

// zvalRunnable wraps a *ZVal as a Runnable for passing pre-evaluated values
type zvalRunnable struct {
	v *phpv.ZVal
}

func (r *zvalRunnable) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	return r.v, nil
}

func (r *zvalRunnable) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%v", r.v)
	return err
}

func (z *ZClosure) Spawn(ctx phpv.Context) (*phpv.ZVal, error) {
	o, err := phpobj.NewZObjectOpaque(ctx, Closure, z)
	if err != nil {
		return nil, err
	}
	return o.ZVal(), nil
}

func (closure *ZClosure) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	if closure.name != "" {
		// register function - don't resolve default argument values now;
		// they will be resolved lazily in callBody() so that errors
		// (like integer overflow in array spread) are thrown at call time
		// and can be caught by try/catch.

		// Validate #[\NoDiscard] on void/never functions
		for _, attr := range closure.attributes {
			if attr.ClassName == "NoDiscard" || attr.ClassName == "\\NoDiscard" {
				if closure.returnType != nil {
					if closure.returnType.Type() == phpv.ZtVoid {
						return nil, &phpv.PhpError{
							Err:  fmt.Errorf("A void function does not return a value, but #[\\NoDiscard] requires a return value"),
							Code: phpv.E_COMPILE_ERROR,
							Loc:  closure.start,
						}
					}
					if closure.returnType.Type() == phpv.ZtNever {
						return nil, &phpv.PhpError{
							Err:  fmt.Errorf("A never returning function does not return a value, but #[\\NoDiscard] requires a return value"),
							Code: phpv.E_COMPILE_ERROR,
							Loc:  closure.start,
						}
					}
				}
			}
		}

		// If the function is a generator, wrap it
		if closure.isGenerator {
			return nil, ctx.Global().RegisterFunction(closure.name, &generatorClosure{closure})
		}
		return nil, ctx.Global().RegisterFunction(closure.name, closure)
	}
	c := closure.dup()
	// Capture $this from the enclosing method (non-static closures only)
	if !c.isStatic && c.this == nil && ctx.This() != nil {
		c.this = ctx.This()
		c.class = ctx.This().GetClass()
	}
	// For static closures defined in a class method, capture the class scope
	// but not $this.
	if c.isStatic && c.class == nil && ctx.Class() != nil {
		c.class = ctx.Class()
	}
	// For non-static closures without $this (e.g. defined in a static method),
	// capture the class scope so self:: and static:: resolve correctly.
	if !c.isStatic && c.class == nil && ctx.Class() != nil {
		c.class = ctx.Class()
	}
	// For closures in property initializers (const expressions), capture the
	// compiling class scope. This allows closures defined as property defaults
	// to access private members of their defining class.
	if c.class == nil {
		if compilingClass := ctx.Global().GetCompilingClass(); compilingClass != nil {
			c.class = compilingClass
		}
	}
	// Capture the called class for late static binding (static::class).
	// For instance method contexts, the called class is the actual runtime class
	// of $this (unwrapped to get the real class, not the narrowed "kin" class).
	// For static method contexts, use CalledClass() from the FuncContext.
	if ctx.This() != nil {
		unwrapped := ctx.This()
		if uw, ok := unwrapped.(interface{ Unwrap() phpv.ZObject }); ok {
			unwrapped = uw.Unwrap()
		}
		calledClass := unwrapped.GetClass()
		if calledClass != nil && calledClass != c.class {
			c.calledClass = calledClass
		}
	} else if fc := ctx.Func(); fc != nil {
		if cc, ok := fc.(interface{ CalledClass() phpv.ZClass }); ok {
			if called := cc.CalledClass(); called != nil && called != c.class {
				c.calledClass = called
			}
		}
	}
	// run compile after dup so we re-fetch default vars each time
	err = c.Compile(ctx)
	if err != nil {
		return nil, err
	}
	// collect use vars
	for _, s := range c.use {
		if !s.Ref {
			// For by-value captures, emit "Undefined variable" warning
			// if the variable doesn't exist in the enclosing scope.
			if _, exists, _ := ctx.OffsetCheck(ctx, s.VarName); !exists {
				ctx.Warn("Undefined variable $%s", s.VarName, logopt.NoFuncName(true))
			}
		}
		z, err := ctx.OffsetGet(ctx, s.VarName.ZVal())
		if err != nil {
			return nil, err
		}
		if s.Ref {
			// reference capture: share the same ZVal between outer scope and closure
			if !z.IsRef() {
				ref := z.Ref()
				ctx.OffsetSet(ctx, s.VarName.ZVal(), ref)
				s.Value = ref
			} else {
				s.Value = z
			}
		} else {
			s.Value = z.Nude().Dup()
		}
	}
	// If the closure is a generator, wrap it
	if c.isGenerator {
		gc := &generatorClosure{c}
		o, err := phpobj.NewZObjectOpaque(ctx, Closure, gc)
		if err != nil {
			return nil, err
		}
		return o.ZVal(), nil
	}
	return c.Spawn(ctx)
}

func (c *ZClosure) Compile(ctx phpv.Context) error {
	for _, a := range c.args {
		if r, ok := a.DefaultValue.(*phpv.CompileDelayed); ok {
			z, err := r.Run(ctx)
			if err != nil {
				// If the default value can't be resolved at compile time
				// (e.g., "new parent" in a class with no parent), leave it
				// as CompileDelayed for lazy resolution at call time.
				// This matches PHP behavior where defaults like "new parent"
				// or "new self" are only evaluated when the function is called.
				continue
			}
			a.DefaultValue = z.Value()
		}
	}
	return nil
}

func (c *ZClosure) dumpTypeHint(w io.Writer, th *phpv.TypeHint) error {
	if th == nil {
		return nil
	}
	// TypeHint.String() already includes the ? prefix for nullable types
	_, err := w.Write([]byte(th.String()))
	return err
}

func (c *ZClosure) dumpArgs(w io.Writer) error {
	if c.rref {
		if _, err := w.Write([]byte{'&'}); err != nil {
			return err
		}
	}
	if _, err := w.Write([]byte{'('}); err != nil {
		return err
	}
	first := true
	for _, a := range c.args {
		if !first {
			if _, err := w.Write([]byte(", ")); err != nil {
				return err
			}
		}
		first = false
		if a.Hint != nil {
			if err := c.dumpTypeHint(w, a.Hint); err != nil {
				return err
			}
			if _, err := w.Write([]byte{' '}); err != nil {
				return err
			}
		}
		if a.Variadic {
			if _, err := w.Write([]byte("...")); err != nil {
				return err
			}
		}
		if a.Ref {
			if _, err := w.Write([]byte{'&'}); err != nil {
				return err
			}
		}
		if _, err := w.Write([]byte{'$'}); err != nil {
			return err
		}
		if _, err := w.Write([]byte(a.VarName)); err != nil {
			return err
		}
		if a.DefaultValue != nil {
			if _, err := w.Write([]byte(" = ")); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "%#v", a.DefaultValue); err != nil {
				return err
			}
		}
	}
	_, err := w.Write([]byte{')'})
	return err
}

func (c *ZClosure) Dump(w io.Writer) error {
	if c.isArrow {
		// Arrow function: fn(args): type => expr
		if _, err := w.Write([]byte("fn")); err != nil {
			return err
		}
		if err := c.dumpArgs(w); err != nil {
			return err
		}
		if c.returnType != nil {
			if _, err := w.Write([]byte(": ")); err != nil {
				return err
			}
			if err := c.dumpTypeHint(w, c.returnType); err != nil {
				return err
			}
		}
		if _, err := w.Write([]byte(" => ")); err != nil {
			return err
		}
		// For arrow functions, code is a runArrowReturn wrapping the expression
		if ar, ok := c.code.(*runArrowReturn); ok {
			return ar.expr.Dump(w)
		}
		return c.code.Dump(w)
	}

	// Regular closure: function(args) use(...) { ... }
	_, err := w.Write([]byte("function"))
	if c.name != "" {
		_, err = w.Write([]byte{' '})
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(c.name))
		if err != nil {
			return err
		}
	}
	if err = c.dumpArgs(w); err != nil {
		return err
	}

	if len(c.use) > 0 {
		if _, err = w.Write([]byte(" use(")); err != nil {
			return err
		}
		first := true
		for _, u := range c.use {
			if !first {
				if _, err = w.Write([]byte(", ")); err != nil {
					return err
				}
			}
			first = false
			if u.Ref {
				if _, err = w.Write([]byte{'&'}); err != nil {
					return err
				}
			}
			if _, err = fmt.Fprintf(w, "$%s", u.VarName); err != nil {
				return err
			}
		}
		if _, err = w.Write([]byte{')'}); err != nil {
			return err
		}
	}

	if c.returnType != nil {
		if _, err = w.Write([]byte(": ")); err != nil {
			return err
		}
		if err = c.dumpTypeHint(w, c.returnType); err != nil {
			return err
		}
	}

	_, err = w.Write([]byte(" {\n"))
	if err != nil {
		return err
	}

	err = c.code.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte("\n}"))
	return err
}

func (z *ZClosure) GetArgs() []*phpv.FuncArg {
	return z.args
}

func (z *ZClosure) GetReturnType() *phpv.TypeHint {
	return z.returnType
}

func (z *ZClosure) Loc() *phpv.Loc {
	return z.start
}

func (z *ZClosure) Name() string {
	if z.name == "" {
		if z.start != nil {
			// PHP 8.4+: use enclosing function name if available,
			// otherwise fall back to filename
			scope := z.start.Filename
			if z.enclosingFunc != "" {
				scope = z.enclosingFunc
			}
			return fmt.Sprintf("{closure:%s:%d}", scope, z.start.Line)
		}
		return "{closure}"
	}
	return string(z.name)
}

func (z *ZClosure) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Check #[\Deprecated] attribute
	if err := z.checkDeprecated(ctx); err != nil {
		return nil, err
	}

	// If this is a generator function, spawn a Generator object instead of
	// executing the function body directly.
	if z.isGenerator {
		name := z.Name()
		if z.this != nil {
			return phpobj.SpawnGeneratorNamed(ctx, z.callBody, args, name, z.this)
		}
		if ctx.This() != nil {
			return phpobj.SpawnGeneratorNamed(ctx, z.callBody, args, name, ctx.This())
		}
		return phpobj.SpawnGeneratorNamed(ctx, z.callBody, args, name)
	}

	return z.callBody(ctx, args)
}

// deprecationAliasName is a package-level variable that stores an alias method name
// for deprecation messages. When __call() or __callStatic() is invoked,
// the caller sets this to the original method name (e.g., "test") so that
// the deprecation message says "Method Clazz::test()" instead of "Method Clazz::__call()".
var deprecationAliasName string

// SetDeprecationAlias sets the alias name for the next deprecation check.
func SetDeprecationAlias(name string) {
	deprecationAliasName = name
}

// checkDeprecated emits a deprecation warning if this function has #[\Deprecated].
// Returns an error if the user error handler throws an exception.
func (z *ZClosure) checkDeprecated(ctx phpv.Context) error {
	if len(z.attributes) == 0 {
		return nil
	}
	for _, attr := range z.attributes {
		if attr.ClassName == "Deprecated" {
			// Resolve lazy argument expressions (e.g., forward-referenced constants)
			if err := ResolveAttrArgs(ctx, attr); err != nil {
				return err
			}

			// Validate Deprecated constructor arg types before using them
			if err := ValidateDeprecatedArgs(ctx, attr); err != nil {
				return err
			}

			funcName := z.Name()
			label := "Function"
			if z.class != nil {
				label = "Method"
				// Use alias name if set (for __call/__callStatic)
				name := funcName
				if deprecationAliasName != "" {
					name = deprecationAliasName
				}
				funcName = string(z.class.GetName()) + "::" + name
			}
			// Clear the alias after use
			deprecationAliasName = ""

			msg := FormatDeprecatedMsg(label, funcName+"()", attr)
			return ctx.UserDeprecated("%s", msg, logopt.NoFuncName(true))
		}
	}
	return nil
}

// ValidateDeprecatedArgs validates the argument types for #[\Deprecated].
// The constructor signature is: __construct(?string $message = "", ?string $since = "").
// In strict mode, int/float/bool are rejected. Arrays and objects are always rejected.
func ValidateDeprecatedArgs(ctx phpv.Context, attr *phpv.ZAttribute) error {
	for i, arg := range attr.Args {
		if arg == nil || arg.GetType() == phpv.ZtNull || arg.GetType() == phpv.ZtString {
			continue
		}
		paramName := "$message"
		paramNum := 1
		if i == 1 {
			paramName = "$since"
			paramNum = 2
		}
		if i > 1 {
			continue // no more params
		}
		switch arg.GetType() {
		case phpv.ZtInt, phpv.ZtFloat, phpv.ZtBool:
			// In non-strict mode, these coerce to string (OK)
			// In strict mode, these are rejected
			if attr.StrictTypes {
				return phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("Deprecated::__construct(): Argument #%d (%s) must be of type ?string, %s given",
						paramNum, paramName, attrArgTypeName(arg)))
			}
		default:
			// Array, object, etc. always error
			return phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("Deprecated::__construct(): Argument #%d (%s) must be of type ?string, %s given",
					paramNum, paramName, attrArgTypeName(arg)))
		}
	}
	return nil
}

// ValidateNoDiscardArgs validates the argument types for #[\NoDiscard].
// The constructor signature is: __construct(?string $message = "").
// In strict mode, int/float/bool are rejected. Arrays and objects are always rejected.
func ValidateNoDiscardArgs(ctx phpv.Context, attr *phpv.ZAttribute) error {
	if len(attr.Args) == 0 {
		return nil
	}
	arg := attr.Args[0]
	if arg == nil || arg.GetType() == phpv.ZtNull || arg.GetType() == phpv.ZtString {
		return nil
	}
	switch arg.GetType() {
	case phpv.ZtInt, phpv.ZtFloat, phpv.ZtBool:
		if attr.StrictTypes {
			return phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("NoDiscard::__construct(): Argument #1 ($message) must be of type ?string, %s given",
					attrArgTypeName(arg)))
		}
	default:
		return phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("NoDiscard::__construct(): Argument #1 ($message) must be of type ?string, %s given",
				attrArgTypeName(arg)))
	}
	return nil
}

// attrArgTypeName returns the PHP type name for an attribute argument value.
func attrArgTypeName(v *phpv.ZVal) string {
	switch v.GetType() {
	case phpv.ZtObject:
		if obj, ok := v.Value().(phpv.ZObject); ok {
			return string(obj.GetClass().GetName())
		}
		return "object"
	default:
		return v.GetType().TypeName()
	}
}

// attrResolveLoc is the location override used during attribute argument resolution.
// When set, nested deprecation warnings (e.g., accessing deprecated constants
// during attribute arg evaluation) use this location instead of their own
// compile-time location. This ensures that `#[\Deprecated(OTHER_CONST)]` reports
// the line of the statement that triggered the access, not the attribute declaration.
var attrResolveLoc *phpv.Loc

// ResolveAttrArgs evaluates any lazy argument expressions on the attribute.
// This must be called before reading attr.Args when expressions couldn't be
// fully evaluated at compile time (e.g., forward-referenced constants).
// The caller should set ctx location (via ctx.Tick) to the access site before
// calling this function so that nested deprecation warnings report the correct location.
func ResolveAttrArgs(ctx phpv.Context, attr *phpv.ZAttribute) error {
	if attr.ArgExprs == nil {
		return nil
	}
	attr.Resolving = true
	// Save the resolve location override. During attribute argument resolution,
	// nested constant accesses should report the outer access site location.
	savedResolveLoc := attrResolveLoc
	if attrResolveLoc == nil {
		// First level of resolution: capture the current location as the override
		attrResolveLoc = ctx.Loc()
	}
	defer func() {
		attr.Resolving = false
		attrResolveLoc = savedResolveLoc
	}()
	for i, expr := range attr.ArgExprs {
		if expr != nil {
			// Clear the expression BEFORE evaluating to prevent infinite
			// recursion when the expression references the same constant
			// (e.g., #[\Deprecated(TEST)] const TEST = "from itself").
			attr.ArgExprs[i] = nil
			val, err := expr.Run(ctx)
			if err != nil {
				return err
			}
			if val != nil {
				attr.Args[i] = val
			}
		}
	}
	return nil
}

// FormatDeprecatedMsg formats a deprecation message from a #[\Deprecated] attribute.
// Format rules (matching PHP 8.4+):
//   - No args:             "Function foo() is deprecated"
//   - Message only:        "Function foo() is deprecated, msg"
//   - Since only:          "Function foo() is deprecated since 1.0"
//   - Message + since:     "Function foo() is deprecated since 1.0, msg"
//   - Empty message:       "Function foo() is deprecated" (same as no args)
func FormatDeprecatedMsg(label, name string, attr *phpv.ZAttribute) string {
	msg := fmt.Sprintf("%s %s is deprecated", label, name)

	// Extract message (arg 0) and since (arg 1)
	var message, since string
	if len(attr.Args) > 0 && attr.Args[0] != nil && attr.Args[0].GetType() != phpv.ZtNull {
		message = attr.Args[0].String()
	}
	if len(attr.Args) > 1 && attr.Args[1] != nil && attr.Args[1].GetType() != phpv.ZtNull {
		since = attr.Args[1].String()
	}

	if since != "" {
		msg += " since " + since
	}
	if message != "" {
		msg += ", " + message
	}

	return msg
}

// callBody is the actual function body execution, used both for regular calls
// and by SpawnGenerator for generator functions (bypasses generator check).
func (z *ZClosure) callBody(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// typically, we run from a clean context
	var err error

	// set use vars
	for _, u := range z.use {
		if u.Ref {
			// Reference capture: share the same ZVal between closure and outer scope
			ctx.OffsetSet(ctx, u.VarName.ZVal(), u.Value)
		} else {
			// Value capture: duplicate so modifications in the closure
			// body don't persist across invocations
			ctx.OffsetSet(ctx, u.VarName.ZVal(), u.Value.Dup())
		}
	}

	// set args in new context
	for i, a := range z.args {
		if len(args) <= i || args[i] == nil {
			if a.Required {
				funcName := ctx.GetFuncName()
				requiredCount := 0
				for _, arg := range z.args {
					if arg.Required {
						requiredCount++
					}
				}
				// Build the error message with call location.
				// When the function is called from an internal context (e.g., array_walk callback),
				// PHP omits the "in FILE on line LINE" from the exception message.
				callLoc := ctx.Loc()
				isInternalCall := false
				if fc, ok := ctx.(*phpctx.FuncContext); ok {
					if fc.InternalLoc() != nil {
						isInternalCall = true
					}
				}
				var msg string
				if !isInternalCall && callLoc != nil && callLoc.Filename != "" {
					msg = fmt.Sprintf("Too few arguments to function %s(), %d passed in %s on line %d", funcName, len(args), callLoc.Filename, callLoc.Line)
				} else {
					msg = fmt.Sprintf("Too few arguments to function %s(), %d passed", funcName, len(args))
				}
				if requiredCount < len(z.args) {
					msg += fmt.Sprintf(" and at least %d expected", requiredCount)
				} else {
					msg += fmt.Sprintf(" and exactly %d expected", requiredCount)
				}
				return nil, phpobj.ThrowErrorAt(ctx, phpobj.ArgumentCountError, msg, z.start)
			}
			if a.DefaultValue != nil {
				if len(args) == i {
					// need to append to args
					args = append(args, nil)
				}
				// Resolve CompileDelayed defaults lazily at call time
				if cd, ok := a.DefaultValue.(*phpv.CompileDelayed); ok {
					z, err := cd.Run(ctx)
					if err != nil {
						return nil, err
					}
					a.DefaultValue = z.Value()
				}
				args[i] = a.DefaultValue.ZVal()
			} else {
				continue
			}
		}
		if args[i].IsRef() {
			ctx.OffsetSet(ctx, a.VarName.ZVal(), args[i].Ref())
		} else {
			argVal := args[i].Nude().Dup()
			// Coerce value to match type hint (PHP non-strict mode only)
			// In strict mode, type checking is done in callZValImpl, no coercion needed
			// Skip coercion for union/intersection types - they handle their own checking
			if a.Hint != nil && argVal.GetType() != phpv.ZtNull && len(a.Hint.Union) == 0 && len(a.Hint.Intersection) == 0 && !ctx.Global().GetStrictTypes() {
				hintType := a.Hint.Type()
				if hintType != phpv.ZtMixed && hintType != phpv.ZtObject && argVal.GetType() != hintType {
					// Emit implicit conversion deprecation for float->int
					if hintType == phpv.ZtInt && argVal.GetType() == phpv.ZtFloat {
						v, err2 := phpv.FloatToIntImplicit(ctx, argVal.Value().(phpv.ZFloat))
						if err2 != nil {
							return nil, err2
						}
						argVal = v.ZVal()
					} else {
						// NAN coercion warnings are handled by ZFloat.AsVal
						// which is called through argVal.As()
						if coerced, err2 := argVal.As(ctx, hintType); err2 == nil && coerced != nil {
							argVal = coerced.ZVal()
						}
					}
				}
			}
			ctx.OffsetSet(ctx, a.VarName.ZVal(), argVal)
		}
	}

	// call function in that context
	_, err = z.code.Run(ctx)
	if err != nil {
		// Check if this is an explicit return
		r, err := phperr.CatchReturn(nil, err)
		if err != nil {
			return r, err
		}
		if z.rref && r != nil {
			r = r.Ref()
		}
		// Validate and coerce return type (skip for generator bodies - the return type applies
		// to the Generator object, not the internal return value)
		if z.returnType != nil && !z.isGenerator {
			if err := z.checkReturnType(ctx, r); err != nil {
				return nil, err
			}
			// Coerce return value to declared type (PHP non-strict mode)
			r = z.coerceReturnValue(ctx, r)
		}
		return r, nil
	}
	// No explicit return statement - return NULL
	// For void return type, returning without a value is fine
	// For never return type, falling through is an error
	if z.returnType != nil && z.returnType.Type() != phpv.ZtVoid && !z.isGenerator {
		if z.returnType.Type() == phpv.ZtNever {
			funcName := ctx.GetFuncName()
			label := "function"
			if z.class != nil {
				label = "method"
			}
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("%s(): never-returning %s must not implicitly return", funcName, label))
		}
		if err := z.checkReturnTypeNone(ctx); err != nil {
			return nil, err
		}
	}
	return phpv.ZNULL.ZVal(), nil
}

// checkReturnTypeNone validates when a function falls through without a return statement.
// Uses "none returned" in the error message (PHP behavior for implicit returns).
// In PHP 8.5, functions with mixed return type that fall through without returning
// a value throw a TypeError (tested by mixed_return_weak_error.phpt).
func (z *ZClosure) checkReturnTypeNone(ctx phpv.Context) error {
	rt := z.returnType

	// mixed type: includes null, so implicit return (falling through) is allowed.
	// The function implicitly returns null which satisfies the mixed type.
	if rt.Type() == phpv.ZtMixed && len(rt.Union) == 0 && len(rt.Intersection) == 0 {
		return nil
	}

	// nullable types accept null/none (fall-through returns null implicitly)
	if rt.Nullable {
		return nil
	}

	// Check if null would pass (e.g., for nullable union types)
	if rt.Check(ctx, phpv.ZNULL.ZVal()) {
		return nil
	}

	funcName := ctx.GetFuncName()
	return phpobj.ThrowError(ctx, phpobj.TypeError,
		fmt.Sprintf("%s(): Return value must be of type %s, none returned", funcName, rt.String()))
}

// checkReturnType validates the return value against the declared return type.
// Throws TypeError if the value does not match.
func (z *ZClosure) checkReturnType(ctx phpv.Context, retVal *phpv.ZVal) error {
	rt := z.returnType

	// void means the function must not return a value (only bare "return;" or fall-through)
	if rt.Type() == phpv.ZtVoid {
		if retVal != nil && !retVal.IsNull() {
			funcName := ctx.GetFuncName()
			return phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("%s(): Return value must be of type void, %s returned", funcName, phpv.ZValTypeName(retVal)))
		}
		return nil
	}

	// mixed accepts anything
	if rt.Type() == phpv.ZtMixed {
		return nil
	}

	if retVal == nil {
		retVal = phpv.ZNULL.ZVal()
	}

	if !rt.Check(ctx, retVal) {
		funcName := ctx.GetFuncName()
		return phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("%s(): Return value must be of type %s, %s returned", funcName, rt.String(), phpv.ZValTypeName(retVal)))
	}
	return nil
}

// coerceReturnValue coerces the return value to the declared return type in non-strict mode.
func (z *ZClosure) coerceReturnValue(ctx phpv.Context, r *phpv.ZVal) *phpv.ZVal {
	if z.returnType == nil || r == nil {
		return r
	}
	rt := z.returnType
	// Don't coerce for void, never, mixed, or nullable types returning null
	if rt.Type() == phpv.ZtVoid || rt.Type() == phpv.ZtNever || rt.Type() == phpv.ZtMixed {
		return r
	}
	// Don't coerce intersection types
	if len(rt.Intersection) > 0 {
		return r
	}
	// For union types, try to coerce to the first matching scalar alternative
	if len(rt.Union) > 0 {
		// If the value already passes the type check, no coercion needed
		if rt.Check(ctx, r) {
			return r
		}
		// Try coercing to each scalar union member
		for _, alt := range rt.Union {
			if alt.Type() == phpv.ZtBool || alt.Type() == phpv.ZtInt || alt.Type() == phpv.ZtFloat || alt.Type() == phpv.ZtString {
				if coerced, err := r.As(ctx, alt.Type()); err == nil && coerced != nil {
					if alt.Check(ctx, coerced) {
						return coerced
					}
				}
			}
		}
		return r
	}
	// Don't coerce object types
	if rt.Type() == phpv.ZtObject {
		return r
	}
	// If the return value already matches, no coercion needed
	if r.GetType() == rt.Type() {
		return r
	}
	// Null handling
	if r.IsNull() {
		return r
	}
	// Coerce scalar types (with implicit conversion warnings)
	hintType := rt.Type()
	if hintType == phpv.ZtInt && r.GetType() == phpv.ZtFloat {
		v, _ := phpv.FloatToIntImplicit(ctx, r.Value().(phpv.ZFloat))
		return v.ZVal()
	}
	if hintType == phpv.ZtInt || hintType == phpv.ZtFloat || hintType == phpv.ZtString || hintType == phpv.ZtBool {
		if coerced, err := r.As(ctx, hintType); err == nil && coerced != nil {
			return coerced
		}
	}
	return r
}

func (z *ZClosure) dup() *ZClosure {
	n := &ZClosure{}
	n.code = z.code
	n.name = z.name
	n.enclosingFunc = z.enclosingFunc
	n.class = z.class
	n.calledClass = z.calledClass
	n.this = z.this
	n.start = z.start
	n.end = z.end
	n.rref = z.rref
	n.isStatic = z.isStatic
	n.isGenerator = z.isGenerator
	n.usesThis = z.usesThis
	n.attributes = z.attributes
	n.returnType = z.returnType

	if z.args != nil {
		n.args = make([]*phpv.FuncArg, len(z.args))
		for k, v := range z.args {
			n.args[k] = v
		}
	}

	if z.use != nil {
		n.use = make([]*phpv.FuncUse, len(z.use))
		for k, v := range z.use {
			// deep copy so each closure instance has its own FuncUse
			u := *v
			n.use[k] = &u
		}
	}

	return n
}

func (z *ZClosure) GetClass() phpv.ZClass {
	return z.class
}

func (z *ZClosure) GetCalledClass() phpv.ZClass {
	if z.calledClass != nil {
		return z.calledClass
	}
	return z.class
}

func (z *ZClosure) IsStatic() bool {
	return z.isStatic
}

func (z *ZClosure) IsGenerator() bool {
	return z.isGenerator
}

func (z *ZClosure) GetThis() phpv.ZObject {
	return z.this
}

func (z *ZClosure) GetUseVars() []*phpv.FuncUse {
	return z.use
}

func (z *ZClosure) ReturnsByRef() bool {
	return z.rref
}

// GetAttributes returns the PHP attributes on this function/closure.
func (z *ZClosure) GetAttributes() []*phpv.ZAttribute {
	return z.attributes
}

// closureBind implements Closure::bind($closure, $newThis, $newScope = "static")
func closureBind(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
			fmt.Sprintf("Closure::bind() expects at least 2 arguments, %d given", len(args)))
	}

	// First arg: the closure
	closureVal := args[0]
	if closureVal.GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"Closure::bind(): Argument #1 ($closure) must be of type Closure, "+closureVal.GetType().TypeName()+" given")
	}
	closureObj, ok := closureVal.Value().(*phpobj.ZObject)
	if !ok || closureObj.GetClass() != Closure {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"Closure::bind(): Argument #1 ($closure) must be of type Closure")
	}

	opaque := closureObj.GetOpaque(Closure)
	if opaque == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Closure::bind(): internal error - not a closure")
	}

	// Handle wrappedClosure from fromCallable
	if w, ok2 := opaque.(*wrappedClosure); ok2 {
		newThis := args[1]

		// Determine new scope
		var newScope phpv.ZClass
		scopeIsExplicit := false
		scopeIsStatic := true // default: "static" means keep current scope
		if len(args) > 2 && args[2] != nil {
			scopeArg := args[2]
			scopeIsStatic = false
			if scopeArg.GetType() == phpv.ZtString {
				scopeName := phpv.ZString(scopeArg.String())
				if scopeName == "static" {
					scopeIsStatic = true
				} else {
					scopeIsExplicit = true
					cls, err := ctx.Global().GetClass(ctx, scopeName, true)
					if err == nil && cls != nil {
						newScope = cls
					}
				}
			} else if scopeArg.GetType() == phpv.ZtObject {
				scopeIsExplicit = true
				if obj, ok3 := scopeArg.Value().(phpv.ZObject); ok3 {
					newScope = obj.GetClass()
				}
			} else if scopeArg.GetType() == phpv.ZtNull {
				scopeIsExplicit = true
				newScope = nil
			}
		}

		// For fromFunction closures: cannot rebind scope
		if w.fromFunction && scopeIsExplicit {
			if newScope != nil && isInternalClass(newScope) {
				ctx.Warn("Cannot bind closure to scope of internal class %s, this will be an error in PHP 9",
					newScope.GetName(), logopt.NoFuncName(true))
				return phpv.ZNULL.ZVal(), nil
			}
			if newScope != nil {
				ctx.Warn("Cannot rebind scope of closure created from function, this will be an error in PHP 9", logopt.NoFuncName(true))
				return phpv.ZNULL.ZVal(), nil
			}
		}

		// For fromMethod closures: check scope changes and binding constraints
		if w.fromMethod {
			// Cannot bind instance to static method
			if w.isStaticW && newThis.GetType() == phpv.ZtObject {
				ctx.Warn("Cannot bind an instance to a static closure, this will be an error in PHP 9", logopt.NoFuncName(true))
				return phpv.ZNULL.ZVal(), nil
			}

			// Cannot rebind scope (any scope change)
			if scopeIsExplicit || (!scopeIsStatic && !scopeIsExplicit) {
				resolvedScope := w.class
				if scopeIsExplicit {
					resolvedScope = newScope
				}
				if resolvedScope != w.class {
					ctx.Warn("Cannot rebind scope of closure created from method, this will be an error in PHP 9", logopt.NoFuncName(true))
					return phpv.ZNULL.ZVal(), nil
				}
			}

			// Cannot unbind $this from instance method
			if newThis.GetType() == phpv.ZtNull && w.this != nil {
				ctx.Warn("Cannot unbind $this of method, this will be an error in PHP 9", logopt.NoFuncName(true))
				return phpv.ZNULL.ZVal(), nil
			}

			// Cannot bind to incompatible object
			if newThis.GetType() == phpv.ZtObject {
				if obj, ok3 := newThis.Value().(phpv.ZObject); ok3 && w.class != nil {
					if !obj.GetClass().InstanceOf(w.class) {
						ctx.Warn("Cannot bind method %s() to object of class %s, this will be an error in PHP 9",
							w.name, obj.GetClass().GetName(), logopt.NoFuncName(true))
						return phpv.ZNULL.ZVal(), nil
					}
				}
			}
		}

		// Check for internal class scope
		if !w.fromFunction && !w.fromMethod && scopeIsExplicit && newScope != nil && isInternalClass(newScope) {
			ctx.Warn("Cannot bind closure to scope of internal class %s, this will be an error in PHP 9",
				newScope.GetName(), logopt.NoFuncName(true))
			return phpv.ZNULL.ZVal(), nil
		}

		boundW := &wrappedClosure{
			inner:        w.inner,
			name:         w.name,
			args:         w.args,
			this:         w.this,
			class:        w.class,
			fromFunction: w.fromFunction,
			fromMethod:   w.fromMethod,
			isStaticW:    w.isStaticW,
		}
		if newThis.GetType() == phpv.ZtNull {
			boundW.this = nil
		} else if newThis.GetType() == phpv.ZtObject {
			if obj, ok3 := newThis.Value().(phpv.ZObject); ok3 {
				boundW.this = obj
				if !scopeIsExplicit {
					boundW.class = obj.GetClass()
				}
			}
		}
		if scopeIsExplicit && !scopeIsStatic {
			boundW.class = newScope
		}
		return boundW.Spawn(ctx)
	}

	var z *ZClosure
	switch v := opaque.(type) {
	case *ZClosure:
		z = v
	case *generatorClosure:
		z = v.ZClosure
	default:
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Closure::bind(): internal error - unexpected type")
	}

	// Second arg: newThis (object or null)
	newThis := args[1]
	bound := z.dup()

	if newThis.GetType() == phpv.ZtNull {
		// Binding null to a non-static closure that uses $this in its body
		// should warn and return the bound closure (PHP 8.4+)
		if !bound.isStatic && z.usesThis && z.this != nil {
			ctx.Warn("Cannot unbind $this of closure using $this, this will be an error in PHP 9", logopt.NoFuncName(true))
			return phpv.ZNULL.ZVal(), nil
		}
		bound.this = nil
	} else if newThis.GetType() == phpv.ZtObject {
		if bound.isStatic {
			ctx.Warn("Cannot bind an instance to a static closure, this will be an error in PHP 9", logopt.NoFuncName(true))
		}
		if obj, ok2 := newThis.Value().(phpv.ZObject); ok2 {
			bound.this = obj
			// If no scope was explicitly set and the closure has no scope,
			// give it a "dummy" scope of "Closure" (PHP behavior)
			if bound.class == nil && (len(args) <= 2 || args[2] == nil) {
				bound.class = Closure
			}
		}
	} else {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			"Closure::bind(): Argument #2 ($newThis) must be of type ?object, "+newThis.GetType().TypeName()+" given")
	}

	// Third arg: newScope (optional, default "static")
	if len(args) > 2 && args[2] != nil {
		scopeArg := args[2]
		if scopeArg.GetType() == phpv.ZtString {
			scopeName := phpv.ZString(scopeArg.String())
			if scopeName != "static" {
				// Check for internal class
				cls, err := ctx.Global().GetClass(ctx, scopeName, true)
				if err == nil && cls != nil {
					if isInternalClass(cls) {
						ctx.Warn("Cannot bind closure to scope of internal class %s, this will be an error in PHP 9",
							cls.GetName(), logopt.NoFuncName(true))
						return phpv.ZNULL.ZVal(), nil
					}
					bound.class = cls
					// Reset calledClass so static::class resolves to the new scope
					bound.calledClass = nil
				} else {
					ctx.Warn("Class \"%s\" not found", scopeName, logopt.NoFuncName(true))
				}
			}
		} else if scopeArg.GetType() == phpv.ZtObject {
			if obj, ok2 := scopeArg.Value().(phpv.ZObject); ok2 {
				bound.class = obj.GetClass()
				bound.calledClass = nil
			}
		} else if scopeArg.GetType() == phpv.ZtNull {
			bound.class = nil
			bound.calledClass = nil
		} else {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				"Closure::bindTo(): Argument #2 ($newScope) must be of type object|string|null, "+scopeArg.GetType().TypeName()+" given")
		}
	}

	// PHP invariant: if a non-static closure has $this bound, it must have a scope.
	// Assign "Closure" as dummy scope when no scope was set.
	if bound.this != nil && bound.class == nil {
		bound.class = Closure
	}

	return bound.Spawn(ctx)
}

// callerClass returns the class context of the actual caller of Closure::fromCallable(),
// walking up the context chain past the Closure class's own method context.
func callerClass(ctx phpv.Context) phpv.ZClass {
	// Walk up the parent chain to find a non-Closure class context.
	// The immediate ctx is inside Closure::fromCallable (class = Closure).
	// The actual caller is one or more levels up.
	c := ctx
	for c != nil {
		cls := c.Class()
		if cls != nil && cls != Closure {
			return cls
		}
		p := c.Parent(1)
		if p == c || p == nil {
			break
		}
		c = p
	}
	return ctx.Class()
}

// closureFromCallable implements Closure::fromCallable($callable).
// It converts any callable to a Closure object.
func closureFromCallable(ctx phpv.Context, arg *phpv.ZVal) (*phpv.ZVal, error) {
	// If it's already a Closure object, return it as-is
	if arg.GetType() == phpv.ZtObject {
		if obj, ok := arg.Value().(*phpobj.ZObject); ok {
			if obj.GetClass() == Closure {
				return arg, nil
			}
			// Object with __invoke method
			if f, hasInvoke := obj.GetClass().GetMethod("__invoke"); hasInvoke {
				w := &wrappedClosure{
					inner: phpv.Bind(f.Method, obj),
					name:  phpv.ZString(string(obj.GetClass().GetName()) + "::__invoke"),
					this:  obj,
					class: obj.GetClass(),
				}
				if fga, ok2 := f.Method.(phpv.FuncGetArgs); ok2 {
					w.args = fga.GetArgs()
				}
				return w.Spawn(ctx)
			}
			// Object with HandleInvoke
			if h := obj.GetClass().Handlers(); h != nil && h.HandleInvoke != nil {
				w := &wrappedClosure{
					inner: &invokeHandlerCallable{obj: obj, handler: h.HandleInvoke},
					name:  phpv.ZString(string(obj.GetClass().GetName()) + "::__invoke"),
					this:  obj,
					class: obj.GetClass(),
				}
				return w.Spawn(ctx)
			}
		}
	}

	// String callable: "functionName" or "Class::method"
	if arg.GetType() == phpv.ZtString {
		s := arg.AsString(ctx)

		// Handle "self::", "parent::", and "static::" in string callables
		sLower := s.ToLower()
		if strings.HasPrefix(string(sLower), "self::") || strings.HasPrefix(string(sLower), "parent::") || strings.HasPrefix(string(sLower), "static::") {
			prefix := string(s[:strings.Index(string(s), "::")])
			methodName := s[strings.Index(string(s), "::")+2:]

			ctx.Deprecated("Use of \"%s\" in callables is deprecated", prefix, logopt.NoFuncName(true))

			callerCls := callerClass(ctx)
			if callerCls == nil {
				return nil, phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("Cannot use \"%s\" when no class scope is active", prefix))
			}
			var class phpv.ZClass
			if strings.EqualFold(prefix, "parent") {
				class = callerCls.GetParent()
				if class == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error,
						fmt.Sprintf("Cannot use \"parent\" when current class scope has no parent"))
				}
			} else if strings.EqualFold(prefix, "static") {
				// "static" uses late static binding
				if this := ctx.This(); this != nil {
					class = this.GetClass()
				} else {
					class = callerCls
				}
			} else {
				class = callerCls
			}

			member, ok := class.GetMethod(methodName.ToLower())
			if !ok {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("Closure::fromCallable(): Argument #1 ($callback) must be a valid callback, class \"%s\" does not have a method \"%s\"", class.GetName(), methodName))
			}

			// Check visibility for self::/parent:: callables
			declaringClass := class
			if member.Class != nil {
				declaringClass = member.Class
			}
			if member.Modifiers.IsPrivate() {
				if callerCls == nil || callerCls.GetName() != declaringClass.GetName() {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						fmt.Sprintf("Closure::fromCallable(): Argument #1 ($callback) must be a valid callback, cannot access private method %s::%s()", class.GetName(), member.Name))
				}
			} else if member.Modifiers.IsProtected() {
				if callerCls == nil || (!callerCls.InstanceOf(declaringClass) && !declaringClass.InstanceOf(callerCls)) {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						fmt.Sprintf("Closure::fromCallable(): Argument #1 ($callback) must be a valid callback, cannot access protected method %s::%s()", class.GetName(), member.Name))
				}
			}

			// For instance methods called within an object context, use $this
			thisObj := ctx.This()
			var callable phpv.Callable
			if thisObj != nil && !member.Modifiers.IsStatic() {
				callable = phpv.Bind(member.Method, thisObj)
			} else {
				callable = phpv.BindClass(member.Method, class, true)
			}

			w := &wrappedClosure{
				inner:      callable,
				name:       phpv.ZString(string(class.GetName()) + "::" + string(member.Name)),
				class:      class,
				fromMethod: true,
				isStaticW:  member.Modifiers.IsStatic(),
			}
			if thisObj != nil {
				w.this = thisObj
			}
			if fga, ok2 := member.Method.(phpv.FuncGetArgs); ok2 {
				w.args = fga.GetArgs()
			}
			return w.Spawn(ctx)
		}

		if idx := strings.Index(string(s), "::"); idx >= 0 {
			className := s[:idx]
			methodName := s[idx+2:]
			class, err := ctx.Global().GetClass(ctx, className, true)
			if err != nil {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("Closure::fromCallable(): Argument #1 ($callback) must be a valid callback, class \"%s\" not found", className))
			}
			member, ok := class.GetMethod(methodName.ToLower())
			if !ok {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("Closure::fromCallable(): Argument #1 ($callback) must be a valid callback, class \"%s\" does not have a method \"%s\"", className, methodName))
			}
			// Check visibility
			callerCls2 := callerClass(ctx)
			declaringClass := class
			if member.Class != nil {
				declaringClass = member.Class
			}
			if member.Modifiers.IsPrivate() {
				if callerCls2 == nil || callerCls2.GetName() != declaringClass.GetName() {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						fmt.Sprintf("Closure::fromCallable(): Argument #1 ($callback) must be a valid callback, cannot access private method %s::%s()", class.GetName(), member.Name))
				}
			} else if member.Modifiers.IsProtected() {
				if callerCls2 == nil || (!callerCls2.InstanceOf(declaringClass) && !declaringClass.InstanceOf(callerCls2)) {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						fmt.Sprintf("Closure::fromCallable(): Argument #1 ($callback) must be a valid callback, cannot access protected method %s::%s()", class.GetName(), member.Name))
				}
			}
			w := &wrappedClosure{
				inner:      phpv.BindClass(member.Method, class, true),
				name:       phpv.ZString(string(className) + "::" + string(member.Name)),
				class:      class,
				fromMethod: true,
				isStaticW:  member.Modifiers.IsStatic(),
			}
			if fga, ok2 := member.Method.(phpv.FuncGetArgs); ok2 {
				w.args = fga.GetArgs()
			}
			return w.Spawn(ctx)
		}
		// Plain function name
		fn, err := ctx.Global().GetFunction(ctx, s)
		if err != nil {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("Closure::fromCallable(): Argument #1 ($callback) must be a valid callback, function \"%s\" not found or invalid function name", s))
		}
		w := &wrappedClosure{
			inner:        fn,
			name:         s,
			fromFunction: true,
		}
		if fga, ok := fn.(phpv.FuncGetArgs); ok {
			w.args = fga.GetArgs()
		}
		return w.Spawn(ctx)
	}

	// Array callable: [$obj, "method"] or ["ClassName", "method"]
	if arg.GetType() == phpv.ZtArray {
		arr := arg.Array()
		first, err := arr.OffsetGet(ctx, phpv.ZInt(0))
		if err != nil {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				"Closure::fromCallable(): Argument #1 ($callback) must be a valid callback, array callback must have exactly two members")
		}
		second, err := arr.OffsetGet(ctx, phpv.ZInt(1))
		if err != nil {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				"Closure::fromCallable(): Argument #1 ($callback) must be a valid callback, array callback must have exactly two members")
		}

		methodName := second.AsString(ctx)
		var class phpv.ZClass
		var instance phpv.ZObject

		if first.GetType() == phpv.ZtString {
			class, err = ctx.Global().GetClass(ctx, first.AsString(ctx), true)
			if err != nil {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("Closure::fromCallable(): Argument #1 ($callback) must be a valid callback, class \"%s\" not found", first.AsString(ctx)))
			}
		} else if first.GetType() == phpv.ZtObject {
			instance = first.AsObject(ctx)
			class = instance.GetClass()
		} else {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				"Closure::fromCallable(): Argument #1 ($callback) must be a valid callback, first array member is not a valid class name or object")
		}

		member, ok := class.GetMethod(methodName.ToLower())
		if !ok {
			// Check for __call / __callStatic
			if instance != nil {
				if callMethod, hasCall := class.GetMethod("__call"); hasCall {
					origName := methodName
					w := &wrappedClosure{
						inner: &magicCallClosure{callMethod: callMethod.Method, methodName: origName, instance: instance},
						name:  phpv.ZString(string(class.GetName()) + "::__call"),
						this:  instance,
						class: class,
					}
					return w.Spawn(ctx)
				}
			} else {
				if callStaticMethod, hasCallStatic := class.GetMethod("__callstatic"); hasCallStatic {
					origName := methodName
					w := &wrappedClosure{
						inner: &magicCallStaticClosure{callMethod: callStaticMethod.Method, methodName: origName, class: class},
						name:  phpv.ZString(string(class.GetName()) + "::__callStatic"),
						class: class,
					}
					return w.Spawn(ctx)
				}
			}
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("Closure::fromCallable(): Argument #1 ($callback) must be a valid callback, class \"%s\" does not have a method \"%s\"", class.GetName(), methodName))
		}

		// Check visibility
		callerCls3 := callerClass(ctx)
		declaringClass := class
		if member.Class != nil {
			declaringClass = member.Class
		}
		if member.Modifiers.IsPrivate() {
			if callerCls3 == nil || callerCls3.GetName() != declaringClass.GetName() {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("Closure::fromCallable(): Argument #1 ($callback) must be a valid callback, cannot access private method %s::%s()", class.GetName(), member.Name))
			}
		} else if member.Modifiers.IsProtected() {
			if callerCls3 == nil || (!callerCls3.InstanceOf(declaringClass) && !declaringClass.InstanceOf(callerCls3)) {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("Closure::fromCallable(): Argument #1 ($callback) must be a valid callback, cannot access protected method %s::%s()", class.GetName(), member.Name))
			}
		}

		// Non-static method cannot be called statically
		if instance == nil && !member.Modifiers.IsStatic() {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("Failed to create closure from callable: non-static method %s::%s() cannot be called statically", class.GetName(), member.Name))
		}

		var callable phpv.Callable
		if instance != nil {
			callable = phpv.Bind(member.Method, instance)
		} else {
			callable = phpv.BindClass(member.Method, class, true)
		}

		// Use the declaring class for the closure's name (e.g., SplDoublyLinkedList::count)
		// but keep the called class for the scope (for late static binding)
		displayClass := class
		if member.Class != nil && member.Class != class {
			displayClass = member.Class
		}
		w := &wrappedClosure{
			inner:      callable,
			name:       phpv.ZString(string(displayClass.GetName()) + "::" + string(member.Name)),
			class:      class,
			fromMethod: true,
			isStaticW:  member.Modifiers.IsStatic(),
		}
		if fga, ok2 := member.Method.(phpv.FuncGetArgs); ok2 {
			w.args = fga.GetArgs()
		}
		if instance != nil {
			w.this = instance
		}
		return w.Spawn(ctx)
	}

	return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
		"Closure::fromCallable(): Argument #1 ($callback) must be a valid callback, no array, string or object given")
}

// invokeHandlerCallable wraps a HandleInvoke handler as a Callable
type invokeHandlerCallable struct {
	phpv.CallableVal
	obj     phpv.ZObject
	handler func(ctx phpv.Context, o phpv.ZObject, args []phpv.Runnable) (*phpv.ZVal, error)
}

func (i *invokeHandlerCallable) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	runnables := make([]phpv.Runnable, len(args))
	for idx, a := range args {
		runnables[idx] = &zvalRunnable{v: a}
	}
	return i.handler(ctx, i.obj, runnables)
}

// magicCallClosure wraps __call for Closure::fromCallable
type magicCallClosure struct {
	phpv.CallableVal
	callMethod phpv.Callable
	methodName phpv.ZString
	instance   phpv.ZObject
}

func (m *magicCallClosure) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	argsArr := phpv.NewZArray()
	for _, a := range args {
		argsArr.OffsetSet(ctx, nil, a)
	}
	return ctx.CallZVal(ctx, m.callMethod, []*phpv.ZVal{m.methodName.ZVal(), argsArr.ZVal()}, m.instance)
}

// magicCallStaticClosure wraps __callStatic for Closure::fromCallable
type magicCallStaticClosure struct {
	phpv.CallableVal
	callMethod phpv.Callable
	methodName phpv.ZString
	class      phpv.ZClass
}

func (m *magicCallStaticClosure) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	argsArr := phpv.NewZArray()
	for _, a := range args {
		argsArr.OffsetSet(ctx, nil, a)
	}
	return ctx.CallZVal(ctx, m.callMethod, []*phpv.ZVal{m.methodName.ZVal(), argsArr.ZVal()})
}

// closureGetCurrent implements Closure::getCurrent().
// Returns the Closure object for the currently executing anonymous closure.
// Throws an Error if called outside of a closure context.
func closureGetCurrent(ctx phpv.Context) (*phpv.ZVal, error) {
	// Walk up the call stack looking for a closure context.
	// The current context is the getCurrent() call itself,
	// so we need to go to the parent to find the caller.
	cur := ctx.Parent(1)
	for cur != nil {
		fc := cur.Func()
		if fc == nil {
			break
		}
		// Check if the FuncContext has a Callable method
		if callableGetter, ok := fc.(interface{ Callable() phpv.Callable }); ok {
			callable := callableGetter.Callable()
			if callable == nil {
				break
			}
			// Check if it's a ZClosure (anonymous function)
			switch c := callable.(type) {
			case *ZClosure:
				if c.name == "" {
					// Anonymous closure - return its Closure object
					return c.Spawn(ctx)
				}
				// Named function wrapped as closure via foo(...) - not a closure
				return nil, phpobj.ThrowError(ctx, phpobj.Error, "Current function is not a closure")
			case *generatorClosure:
				if c.ZClosure.name == "" {
					return c.ZClosure.Spawn(ctx)
				}
				return nil, phpobj.ThrowError(ctx, phpobj.Error, "Current function is not a closure")
			case *wrappedClosure:
				// Wrapped function from fromCallable - not a closure
				return nil, phpobj.ThrowError(ctx, phpobj.Error, "Current function is not a closure")
			}
			// Some other callable (named function, etc.)
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Current function is not a closure")
		}
		// Try to go up another level
		cur = cur.Parent(1)
	}
	return nil, phpobj.ThrowError(ctx, phpobj.Error, "Current function is not a closure")
}

func closureDebugInfo(ctx phpv.Context, o *phpobj.ZObject) (*phpv.ZVal, error) {
	opaque := o.GetOpaque(Closure)
	if opaque == nil {
		return phpv.NewZArray().ZVal(), nil
	}

	// Handle wrappedClosure (from Closure::fromCallable)
	if w, ok := opaque.(*wrappedClosure); ok {
		arr := phpv.NewZArray()
		arr.OffsetSet(ctx, phpv.ZString("function"), phpv.ZString(w.name).ZVal())
		if len(w.args) > 0 {
			paramArr := phpv.NewZArray()
			for _, a := range w.args {
				paramKey := "$" + string(a.VarName)
				var paramVal string
				if a.Required {
					paramVal = "<required>"
				} else {
					paramVal = "<optional>"
				}
				paramArr.OffsetSet(ctx, phpv.ZString(paramKey), phpv.ZString(paramVal).ZVal())
			}
			arr.OffsetSet(ctx, phpv.ZString("parameter"), paramArr.ZVal())
		}
		// "static" key: static variables from the wrapped function
		if innerClosure, ok2 := w.inner.(*ZClosure); ok2 {
			staticVars := collectStaticVars(innerClosure.code)
			if len(staticVars) > 0 {
				staticArr := phpv.NewZArray()
				for _, sv := range staticVars {
					val := sv.z
					if val == nil {
						val = phpv.ZNULL.ZVal()
					}
					staticArr.OffsetSet(ctx, sv.varName, val)
				}
				arr.OffsetSet(ctx, phpv.ZString("static"), staticArr.ZVal())
			}
		}
		if w.this != nil {
			arr.OffsetSet(ctx, phpv.ZString("this"), w.this.ZVal())
		}
		return arr.ZVal(), nil
	}

	var z *ZClosure
	switch v := opaque.(type) {
	case *ZClosure:
		z = v
	case *generatorClosure:
		z = v.ZClosure
	default:
		return phpv.NewZArray().ZVal(), nil
	}

	arr := phpv.NewZArray()

	if z.name != "" && z.start == nil {
		// Internal/named function wrapped as closure: use "function" key
		arr.OffsetSet(ctx, phpv.ZString("function"), phpv.ZString(z.name).ZVal())
	} else {
		// User-defined closure: use name/file/line
		name := z.Name()
		arr.OffsetSet(ctx, phpv.ZString("name"), phpv.ZString(name).ZVal())
		if z.start != nil {
			arr.OffsetSet(ctx, phpv.ZString("file"), phpv.ZString(z.start.Filename).ZVal())
			arr.OffsetSet(ctx, phpv.ZString("line"), phpv.ZInt(z.start.Line).ZVal())
		}
	}

	// "parameter" key: function parameters (must come before static/this to match PHP order)
	if len(z.args) > 0 {
		paramArr := phpv.NewZArray()
		for _, a := range z.args {
			paramKey := "$" + string(a.VarName)
			var paramVal string
			if a.Required {
				paramVal = "<required>"
			} else {
				paramVal = "<optional>"
			}
			paramArr.OffsetSet(ctx, phpv.ZString(paramKey), phpv.ZString(paramVal).ZVal())
		}
		arr.OffsetSet(ctx, phpv.ZString("parameter"), paramArr.ZVal())
	}

	// "static" key: captured use() variables AND static variables from the code body
	hasStatic := false
	staticArr := phpv.NewZArray()

	// use() variables
	for _, u := range z.use {
		hasStatic = true
		val := u.Value
		if val == nil {
			val = phpv.ZNULL.ZVal()
		}
		staticArr.OffsetSet(ctx, u.VarName, val)
	}

	// Static variables from the function body (static $x = ...)
	if z.code != nil {
		staticVars := collectStaticVars(z.code)
		for _, sv := range staticVars {
			hasStatic = true
			val := sv.z
			if val == nil {
				val = phpv.ZNULL.ZVal()
			}
			staticArr.OffsetSet(ctx, sv.varName, val)
		}
	}

	if hasStatic {
		arr.OffsetSet(ctx, phpv.ZString("static"), staticArr.ZVal())
	}

	// "this" key: captured $this
	if z.this != nil {
		arr.OffsetSet(ctx, phpv.ZString("this"), z.this.ZVal())
	}

	return arr.ZVal(), nil
}

// collectStaticVars walks a Runnable tree and collects all staticVarInfo from runStaticVar nodes.
func collectStaticVars(r phpv.Runnable) []*staticVarInfo {
	if r == nil {
		return nil
	}
	switch v := r.(type) {
	case *runStaticVar:
		return v.vars
	case phpv.Runnables:
		var result []*staticVarInfo
		for _, sub := range v {
			result = append(result, collectStaticVars(sub)...)
		}
		return result
	default:
		return nil
	}
}
