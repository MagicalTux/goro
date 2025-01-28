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
	class, err := ctx.Global().GetClass(ctx, r.obj, true)
	if err != nil {
		return nil, err
	}

	var args []*phpv.ZVal
	for _, r := range r.newArg {
		arg, err := r.Run(ctx)
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
	}

	z, err := phpobj.NewZObject(ctx, class, args...)
	if err != nil {
		return nil, err
	}

	return z.ZVal(), nil
}

func compileNew(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// next should be either:
	// T_CLASS (anonymous class)
	// string (name of a class)
	var err error

	n := &runNewObject{l: i.Loc()}

	n.obj, err = compileClassName(c)
	if err != nil {
		return nil, err
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
	ref    phpv.Runnable
	op     phpv.ZString
	args   phpv.Runnables
	l      *phpv.Loc
	static bool
}

type runObjectVar struct {
	ref     phpv.Runnable
	varName phpv.ZString
	l       *phpv.Loc
}

func (r *runObjectFunc) Dump(w io.Writer) error {
	err := r.ref.Dump(w)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "->%s(", r.op)
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
	_, err = fmt.Fprintf(w, "->%s", r.varName)
	return err
}

func (r *runObjectFunc) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	ctx.Tick(ctx, r.l)
	// fetch object
	obj, err := r.ref.Run(ctx)
	if err != nil {
		return nil, err
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
			if ctx.This() == nil {
				return nil, ctx.Errorf("Cannot access self:: when no method scope is active")
			}
			if ctx.This().GetClass() == nil {
				return nil, ctx.Errorf("Cannot access self:: when no class scope is active")
			}
			objI = ctx.This()
			class = objI.GetClass()

		case "parent":
			if ctx.This() == nil {
				return nil, ctx.Errorf("Cannot access parent:: when no method scope is active")
			}
			if ctx.This().GetClass() == nil {
				return nil, ctx.Errorf("Cannot access parent:: when no class scope is active")
			}
			if ctx.This().GetClass().GetParent() == nil {
				return nil, ctx.Errorf("Cannot access parent:: when current class scope has no parent")
			}
			objI = ctx.This().GetParent()
			class = objI.GetClass()

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
	if !ok {
		return nil, ctx.Errorf("Call to undefine method %s:%s()", class.GetName(), op)
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

		m := phpv.BindClass(method.Method, class, false)
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

	// TODO Check access rights
	return objI.ObjectGet(ctx, offt)
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
	if r.varName[0] == '$' {
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
	case tokenizer.T_VARIABLE:
		return &runClassStaticVarRef{v, ident[1:], l}, nil

	case tokenizer.T_STRING:
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		switch i.Type {
		case tokenizer.Rune('('):
			c.backup()
			v := &runObjectFunc{ref: v, op: ident, l: l, static: true}
			v.args, err = compileFuncPassedArgs(c)
			return v, err
		default:
			c.backup()
			return &runClassStaticObjRef{v, ident, l}, nil
		}
	}
}

func compileObjectOperator(v phpv.Runnable, i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// call a method or get a variable on an object
	l := i.Loc()

	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.Type != tokenizer.T_STRING && i.Type != tokenizer.T_VARIABLE {
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
		v := &runObjectFunc{ref: v, op: op, l: l}

		// parse args
		v.args, err = compileFuncPassedArgs(c)
		return v, err
	}

	return &runObjectVar{ref: v, varName: op, l: l}, nil
}

func compileClassName(c compileCtx) (phpv.ZString, error) {
	var r phpv.ZString

	i, err := c.NextItem()
	if err != nil {
		return r, err
	}

	if i.Type == tokenizer.T_NS_SEPARATOR {
		r = "\\"
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
			return r, nil
		}
	}
}
