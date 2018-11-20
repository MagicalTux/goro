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
	return nil, errors.New("todo fetch var from class")
}

func (r *runClassStaticVarRef) WriteValue(ctx Context, value *ZVal) error {
	return errors.New("todo set class static value")
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
	// first, fetch class object
	class, err := ctx.Global().GetClass(r.className)
	if err != nil {
		return nil, err
	}

	method, ok := class.Methods[r.objName.ToLower()]
	if !ok {
		return nil, fmt.Errorf("Call to undefined method %s::%s()", r.className, r.objName)
	}

	return method.Method.Call(ctx, args)
}

func (r *runClassStaticObjRef) Loc() *Loc {
	return r.l
}

func (r *runClassStaticObjRef) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s::%s", r.className, r.objName)
	return err
}
