package compiler

import (
	"fmt"
	"io"
	"strings"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

type ZClosure struct {
	phpv.CallableVal
	name        phpv.ZString
	args        []*phpv.FuncArg
	use         []*phpv.FuncUse
	code        phpv.Runnable
	class       phpv.ZClass    // class in which this closure was defined (for parent:: and self::)
	this        phpv.ZObject   // captured $this from enclosing method (nil for static closures and free functions)
	start       *phpv.Loc
	end         *phpv.Loc
	rref        bool // return ref?
	isStatic    bool // true for static function() {} and static fn() =>
	isGenerator bool // true if this function contains yield
	attributes  []*phpv.ZAttribute // PHP 8.0 attributes on this function
	returnType  *phpv.TypeHint     // return type declaration (nil if none)
}

// > class Closure
var Closure = &phpobj.ZClass{
	Name: "Closure",
	H:    &phpv.ZClassHandlers{},
}

// wrappedClosure wraps an arbitrary Callable as a Closure object opaque.
// Used by Closure::fromCallable() to wrap non-closure callables.
type wrappedClosure struct {
	phpv.CallableVal
	inner    phpv.Callable
	name     phpv.ZString
	args     []*phpv.FuncArg
	this     phpv.ZObject
	class    phpv.ZClass
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
	return false
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

func (w *wrappedClosure) Spawn(ctx phpv.Context) (*phpv.ZVal, error) {
	o, err := phpobj.NewZObjectOpaque(ctx, Closure, w)
	if err != nil {
		return nil, err
	}
	return o.ZVal(), nil
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
		"__debuginfo": {
			Name:      "__debugInfo",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return closureDebugInfo(ctx, o)
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
	// but not $this
	if c.isStatic && c.class == nil && ctx.Class() != nil {
		c.class = ctx.Class()
	}
	// run compile after dup so we re-fetch default vars each time
	err = c.Compile(ctx)
	if err != nil {
		return nil, err
	}
	// collect use vars
	for _, s := range c.use {
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
				return err
			}
			a.DefaultValue = z.Value()
		}
	}
	return nil
}

func (c *ZClosure) Dump(w io.Writer) error {
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
	_, err = w.Write([]byte{'('})
	if err != nil {
		return err
	}
	first := true
	for _, a := range c.args {
		if !first {
			_, err = w.Write([]byte{','})
			if err != nil {
				return err
			}
		}
		first = false
		if a.Ref {
			_, err = w.Write([]byte{'&'})
			if err != nil {
				return err
			}
		}
		_, err = w.Write([]byte{'$'})
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(a.VarName))
		if err != nil {
			return err
		}
		if a.DefaultValue != nil {
			_, err = w.Write([]byte{'='})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(w, "%#v", a.DefaultValue) // TODO
			if err != nil {
				return err
			}
		}
	}

	if c.use != nil {
		// TODO use
	}

	_, err = w.Write([]byte{'{'})
	if err != nil {
		return err
	}

	err = c.code.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{'}'})
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
			return fmt.Sprintf("{closure:%s:%d}", z.start.Filename, z.start.Line)
		}
		return "{closure}"
	}
	return string(z.name)
}

func (z *ZClosure) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// Check #[\Deprecated] attribute
	z.checkDeprecated(ctx)

	// If this is a generator function, spawn a Generator object instead of
	// executing the function body directly.
	if z.isGenerator {
		return phpobj.SpawnGenerator(ctx, z.callBody, args)
	}

	return z.callBody(ctx, args)
}

// checkDeprecated emits a deprecation warning if this function has #[\Deprecated]
func (z *ZClosure) checkDeprecated(ctx phpv.Context) {
	if len(z.attributes) == 0 {
		return
	}
	for _, attr := range z.attributes {
		if attr.ClassName == "Deprecated" {
			funcName := z.Name()
			label := "Function"
			if z.class != nil {
				label = "Method"
				funcName = string(z.class.GetName()) + "::" + funcName
			}

			msg := FormatDeprecatedMsg(label, funcName+"()", attr)
			ctx.UserDeprecated("%s", msg, logopt.NoFuncName(true))
			return
		}
	}
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
	if len(attr.Args) > 0 && attr.Args[0].GetType() == phpv.ZtString {
		message = attr.Args[0].String()
	}
	if len(attr.Args) > 1 && attr.Args[1].GetType() == phpv.ZtString {
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
				// Build the error message with call location info
				msg := fmt.Sprintf("Too few arguments to function %s(), %d passed", funcName, len(args))
				if callLoc := ctx.Loc(); callLoc != nil {
					msg += fmt.Sprintf(" in %s on line %d", callLoc.Filename, callLoc.Line)
				}
				msg += fmt.Sprintf(" and exactly %d expected", requiredCount)
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
			// Coerce value to match type hint (PHP non-strict mode)
			if a.Hint != nil && argVal.GetType() != phpv.ZtNull {
				hintType := a.Hint.Type()
				if hintType != phpv.ZtMixed && hintType != phpv.ZtObject && argVal.GetType() != hintType {
					if coerced, err2 := argVal.As(ctx, hintType); err2 == nil && coerced != nil {
						argVal = coerced.ZVal()
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
		// Validate return type
		if z.returnType != nil {
			if err := z.checkReturnType(ctx, r); err != nil {
				return nil, err
			}
		}
		return r, nil
	}
	// No explicit return statement - return NULL
	// For void return type, returning without a value is fine
	if z.returnType != nil && z.returnType.Type() != phpv.ZtVoid {
		if err := z.checkReturnTypeNone(ctx); err != nil {
			return nil, err
		}
	}
	return phpv.ZNULL.ZVal(), nil
}

// checkReturnTypeNone validates when a function falls through without a return statement.
// Uses "none returned" in the error message (PHP behavior for implicit returns).
func (z *ZClosure) checkReturnTypeNone(ctx phpv.Context) error {
	rt := z.returnType

	// mixed and nullable types accept null/none
	if rt.Type() == phpv.ZtMixed || rt.Nullable {
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

func (z *ZClosure) dup() *ZClosure {
	n := &ZClosure{}
	n.code = z.code
	n.name = z.name
	n.class = z.class
	n.this = z.this
	n.start = z.start
	n.end = z.end
	n.rref = z.rref
	n.isStatic = z.isStatic
	n.isGenerator = z.isGenerator
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

func (z *ZClosure) IsStatic() bool {
	return z.isStatic
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
		boundW := &wrappedClosure{
			inner: w.inner,
			name:  w.name,
			args:  w.args,
			this:  w.this,
			class: w.class,
		}
		if newThis.GetType() == phpv.ZtNull {
			boundW.this = nil
		} else if newThis.GetType() == phpv.ZtObject {
			if obj, ok3 := newThis.Value().(phpv.ZObject); ok3 {
				boundW.this = obj
				boundW.class = obj.GetClass()
			}
		}
		if len(args) > 2 && args[2] != nil {
			scopeArg := args[2]
			if scopeArg.GetType() == phpv.ZtString {
				scopeName := phpv.ZString(scopeArg.String())
				if scopeName != "static" {
					cls, err := ctx.Global().GetClass(ctx, scopeName, true)
					if err == nil && cls != nil {
						boundW.class = cls
					}
				}
			} else if scopeArg.GetType() == phpv.ZtObject {
				if obj, ok3 := scopeArg.Value().(phpv.ZObject); ok3 {
					boundW.class = obj.GetClass()
				}
			} else if scopeArg.GetType() == phpv.ZtNull {
				boundW.class = nil
			}
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
		// Binding null to a non-static closure that already has $this bound
		// should warn and return null in PHP
		if !bound.isStatic && z.this != nil {
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
				cls, err := ctx.Global().GetClass(ctx, scopeName, true)
				if err == nil && cls != nil {
					bound.class = cls
				} else {
					ctx.Warn("Class \"%s\" not found", scopeName, logopt.NoFuncName(true))
				}
			}
		} else if scopeArg.GetType() == phpv.ZtObject {
			if obj, ok2 := scopeArg.Value().(phpv.ZObject); ok2 {
				bound.class = obj.GetClass()
			}
		} else if scopeArg.GetType() == phpv.ZtNull {
			bound.class = nil
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

		// Handle "self::" and "parent::" in string callables
		sLower := s.ToLower()
		if strings.HasPrefix(string(sLower), "self::") || strings.HasPrefix(string(sLower), "parent::") {
			prefix := string(s[:strings.Index(string(s), "::")])
			methodName := s[strings.Index(string(s), "::")+2:]

			ctx.Deprecated("Use of \"%s\" in callables is deprecated", prefix)

			callerClass := ctx.Class()
			if callerClass == nil {
				return nil, phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("Cannot use \"%s\" when no class scope is active", prefix))
			}
			var class phpv.ZClass
			if strings.EqualFold(prefix, "parent") {
				class = callerClass.GetParent()
				if class == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error,
						fmt.Sprintf("Cannot use \"parent\" when current class scope has no parent"))
				}
			} else {
				class = callerClass
			}

			member, ok := class.GetMethod(methodName.ToLower())
			if !ok {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("Closure::fromCallable(): Argument #1 ($callback) must be a valid callback, class \"%s\" does not have a method \"%s\"", class.GetName(), methodName))
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
				inner: callable,
				name:  phpv.ZString(string(class.GetName()) + "::" + string(member.Name)),
				class: class,
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
			w := &wrappedClosure{
				inner: phpv.BindClass(member.Method, class, true),
				name:  phpv.ZString(string(className) + "::" + string(member.Name)),
				class: class,
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
			inner: fn,
			name:  s,
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
		callerClass := ctx.Class()
		declaringClass := class
		if member.Class != nil {
			declaringClass = member.Class
		}
		if member.Modifiers.IsPrivate() {
			if callerClass == nil || callerClass.GetName() != declaringClass.GetName() {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("Closure::fromCallable(): Argument #1 ($callback) must be a valid callback, cannot access private method %s::%s()", class.GetName(), member.Name))
			}
		} else if member.Modifiers.IsProtected() {
			if callerClass == nil || (!callerClass.InstanceOf(declaringClass) && !declaringClass.InstanceOf(callerClass)) {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("Closure::fromCallable(): Argument #1 ($callback) must be a valid callback, cannot access protected method %s::%s()", class.GetName(), member.Name))
			}
		}

		var callable phpv.Callable
		if instance != nil {
			callable = phpv.Bind(member.Method, instance)
		} else {
			callable = phpv.BindClass(member.Method, class, true)
		}

		w := &wrappedClosure{
			inner: callable,
			name:  phpv.ZString(string(class.GetName()) + "::" + string(member.Name)),
			class: class,
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

	// "static" key: captured use() variables
	if len(z.use) > 0 {
		staticArr := phpv.NewZArray()
		for _, u := range z.use {
			val := u.Value
			if val == nil {
				val = phpv.ZNULL.ZVal()
			}
			staticArr.OffsetSet(ctx, u.VarName, val)
		}
		arr.OffsetSet(ctx, phpv.ZString("static"), staticArr.ZVal())
	}

	// "this" key: captured $this
	if z.this != nil {
		arr.OffsetSet(ctx, phpv.ZString("this"), z.this.ZVal())
	}

	// "parameter" key: function parameters
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

	return arr.ZVal(), nil
}
