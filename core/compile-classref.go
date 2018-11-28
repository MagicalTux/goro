package core

import (
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/phpv"
)

// when classname::$something is used
type runClassStaticVarRef struct {
	className, varName phpv.ZString
	l                  *phpv.Loc
}

func (r *runClassStaticVarRef) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	class, err := ctx.Global().(*Global).GetClass(ctx, r.className)
	if err != nil {
		return nil, err
	}

	p, err := class.getStaticProps(ctx)
	if err != nil {
		return nil, err
	}

	return p.GetString(r.varName), nil
}

func (r *runClassStaticVarRef) WriteValue(ctx phpv.Context, value *phpv.ZVal) error {
	class, err := ctx.Global().(*Global).GetClass(ctx, r.className)
	if err != nil {
		return err
	}

	p, err := class.getStaticProps(ctx)
	if err != nil {
		return err
	}

	return p.SetString(r.varName, value)
}

func (r *runClassStaticVarRef) Loc() *phpv.Loc {
	return r.l
}

func (r *runClassStaticVarRef) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s::$%s", r.className, r.varName)
	return err
}

// when classname::something is used
type runClassStaticObjRef struct {
	className, objName phpv.ZString
	l                  *phpv.Loc
}

func (r *runClassStaticObjRef) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	class, err := ctx.Global().(*Global).GetClass(ctx, r.className)
	if err != nil {
		return nil, err
	}

	v, ok := class.Const[r.objName]
	if !ok {
		return phpv.ZNull{}.ZVal(), nil
	}

	return v.ZVal(), nil
}

func (r *runClassStaticObjRef) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	ctx = ctx.Parent(1) // go back one level
	// first, fetch class object
	class, err := ctx.Global().(*Global).GetClass(ctx, r.className)
	if err != nil {
		return nil, err
	}

	method, ok := class.Methods[r.objName.ToLower()]
	if !ok {
		method, ok = class.Methods["__callStatic"]
		if ok {
			// found __call method
			a := phpv.NewZArray()
			callArgs := []*phpv.ZVal{r.objName.ZVal(), a.ZVal()}

			for _, sub := range args {
				a.OffsetSet(ctx, nil, sub)
			}

			return ctx.CallZVal(ctx, method.Method, callArgs, ctx.This())
		}
		return nil, fmt.Errorf("Call to undefined method %s::%s()", r.className, r.objName)
	}

	return ctx.CallZVal(ctx, method.Method, args, ctx.This())
}

func (r *runClassStaticObjRef) Loc() *phpv.Loc {
	return r.l
}

func (r *runClassStaticObjRef) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s::%s", r.className, r.objName)
	return err
}
