package core

import (
	"errors"
	"fmt"
	"io"
)

// when classname::$something is used
type runClassStaticVarRef struct {
	className, varName ZString
	l                  *Loc
}

func (r *runClassStaticVarRef) Run(ctx Context) (*ZVal, error) {
	class, err := ctx.Global().GetClass(ctx, r.className)
	if err != nil {
		return nil, err
	}

	p, err := class.getStaticProps(ctx)
	if err != nil {
		return nil, err
	}

	return p.GetString(r.varName), nil
}

func (r *runClassStaticVarRef) WriteValue(ctx Context, value *ZVal) error {
	class, err := ctx.Global().GetClass(ctx, r.className)
	if err != nil {
		return err
	}

	p, err := class.getStaticProps(ctx)
	if err != nil {
		return err
	}

	return p.SetString(r.varName, value)
}

func (r *runClassStaticVarRef) Loc() *Loc {
	return r.l
}

func (r *runClassStaticVarRef) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s::$%s", r.className, r.varName)
	return err
}

// when classname::something is used
type runClassStaticObjRef struct {
	className, objName ZString
	l                  *Loc
}

func (r *runClassStaticObjRef) Run(ctx Context) (*ZVal, error) {
	// attempt to fetch a constant under that name
	return nil, errors.New("todo class fetch constant")
}

func (r *runClassStaticObjRef) Call(ctx Context, args []*ZVal) (*ZVal, error) {
	ctx = ctx.Parent(1) // go back one level
	// first, fetch class object
	class, err := ctx.Global().GetClass(ctx, r.className)
	if err != nil {
		return nil, err
	}

	method, ok := class.Methods[r.objName.ToLower()]
	if !ok {
		method, ok = class.Methods["__callStatic"]
		if ok {
			// found __call method
			a := NewZArray()
			callArgs := []*ZVal{r.objName.ZVal(), a.ZVal()}

			for _, sub := range args {
				a.OffsetSet(ctx, nil, sub)
			}

			return ctx.CallZVal(ctx, method.Method, callArgs, ctx.This())
		}
		return nil, fmt.Errorf("Call to undefined method %s::%s()", r.className, r.objName)
	}

	return ctx.CallZVal(ctx, method.Method, args, ctx.This())
}

func (r *runClassStaticObjRef) Loc() *Loc {
	return r.l
}

func (r *runClassStaticObjRef) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s::%s", r.className, r.objName)
	return err
}
