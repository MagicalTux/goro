package compiler

import (
	"fmt"
	"io"

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
	isGenerator bool // true if this function contains yield
	attributes  []*phpv.ZAttribute // PHP 8.0 attributes on this function
}

// > class Closure
var Closure = &phpobj.ZClass{
	Name: "Closure",
	H:    &phpv.ZClassHandlers{},
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
		default:
			return nil, fmt.Errorf("invalid closure opaque type: %T", opaque)
		}
		// Use the captured $this if available (closure defined in a class method)
		if z.this != nil {
			return ctx.Call(ctx, callable, args, z.this)
		}
		return ctx.Call(ctx, callable, args, o)
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
		// fromCallable is complex - for now just stub it
		"fromcallable": {
			Name:      "fromCallable",
			Modifiers: phpv.ZAttrPublic | phpv.ZAttrStatic,
			Method: phpobj.NativeStaticMethod(func(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) < 1 {
					return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError, "Closure::fromCallable() expects exactly 1 argument, 0 given")
				}
				// If it's already a Closure object, return it
				if args[0].GetType() == phpv.ZtObject {
					if obj, ok := args[0].Value().(*phpobj.ZObject); ok {
						if obj.GetClass() == Closure {
							return args[0], nil
						}
					}
				}
				return nil, phpobj.ThrowError(ctx, phpobj.Error, "Closure::fromCallable() not fully implemented")
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

				var z *ZClosure
				switch v := opaque.(type) {
				case *ZClosure:
					z = v
				case *generatorClosure:
					z = v.ZClosure
				default:
					return nil, phpobj.ThrowError(ctx, phpobj.Error, "Closure::call(): internal error - unexpected type")
				}

				// First arg is newThis, rest are call args
				newThis := args[0]
				callArgs := args[1:]

				bound := z.dup()
				if newThis.GetType() == phpv.ZtObject {
					if obj, ok := newThis.Value().(phpv.ZObject); ok {
						bound.this = obj
						bound.class = obj.GetClass()
					}
				}
				return bound.Call(ctx, callArgs)
			}),
		},
		"__invoke": {
			Name:      "__invoke",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				// __invoke delegates to the HandleInvoke handler
				opaque := o.GetOpaque(Closure)
				var callable phpv.Callable
				switch v := opaque.(type) {
				case *ZClosure:
					callable = v
				case *generatorClosure:
					callable = v
				default:
					return nil, fmt.Errorf("invalid closure opaque type: %T", opaque)
				}
				return callable.Call(ctx, args)
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
	if c.this == nil && ctx.This() != nil {
		c.this = ctx.This()
		c.class = ctx.This().GetClass()
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

			msg := fmt.Sprintf("%s %s() is deprecated", label, funcName)
			if len(attr.Args) > 0 && attr.Args[0].GetType() == phpv.ZtString {
				msg += ", " + attr.Args[0].String()
			}

			ctx.Deprecated("%s", msg)
			return
		}
	}
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
			ctx.OffsetSet(ctx, a.VarName.ZVal(), args[i].Nude().Dup())
		}
	}

	// call function in that context
	_, err = z.code.Run(ctx)
	if err != nil {
		// Check if this is an explicit return
		r, err := phperr.CatchReturn(nil, err)
		if z.rref && r != nil {
			r = r.Ref()
		}
		return r, err
	}
	// No explicit return statement - return NULL
	return phpv.ZNULL.ZVal(), nil
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
	n.isGenerator = z.isGenerator
	n.attributes = z.attributes

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

func (z *ZClosure) ReturnsByRef() bool {
	return z.rref
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
		bound.this = nil
	} else if newThis.GetType() == phpv.ZtObject {
		if obj, ok2 := newThis.Value().(phpv.ZObject); ok2 {
			bound.this = obj
			bound.class = obj.GetClass()
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
				}
			}
		} else if scopeArg.GetType() == phpv.ZtObject {
			if obj, ok2 := scopeArg.Value().(phpv.ZObject); ok2 {
				bound.class = obj.GetClass()
			}
		} else if scopeArg.GetType() == phpv.ZtNull {
			bound.class = nil
		}
	}

	return bound.Spawn(ctx)
}

// closureDebugInfo builds the __debugInfo array for a Closure object.
// For user-defined closures this returns: name, file, line, [static], [this], [parameter]
// For internal function closures it returns: function, [parameter]
func closureDebugInfo(ctx phpv.Context, o *phpobj.ZObject) (*phpv.ZVal, error) {
	opaque := o.GetOpaque(Closure)
	if opaque == nil {
		return phpv.NewZArray().ZVal(), nil
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
