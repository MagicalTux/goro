package core

import (
	"errors"
	"fmt"
	"io"

	"github.com/MagicalTux/gophp/core/tokenizer"
)

type runNewObject struct {
	obj    ZString
	cl     Runnable // for anonymous
	newArg Runnables
	l      *Loc
}

func (r *runNewObject) Loc() *Loc {
	return r.l
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

func (r *runNewObject) Run(ctx Context) (*ZVal, error) {
	class, err := ctx.GetGlobal().GetClass(r.obj)
	if err != nil {
		return nil, err
	}
	z, err := NewZObject(ctx, class)
	if err != nil {
		return nil, err
	}

	return z.ZVal(), nil
}

func compileNew(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	// next should be either:
	// T_CLASS (anonymous class)
	// string (name of a class)

	n := &runNewObject{l: MakeLoc(i.Loc())}

	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.Type != tokenizer.T_STRING {
		return nil, i.Unexpected()
	}

	n.obj = ZString(i.Data)

	// read constructor args
	n.newArg, err = compileFuncPassedArgs(c)

	return n, nil
}

type runObjectFunc struct {
	ref  Runnable
	op   ZString
	args Runnables
	l    *Loc
}

type runObjectVar struct {
	ref     Runnable
	varName ZString
	l       *Loc
}

func (r *runObjectFunc) Loc() *Loc {
	return r.l
}

func (r *runObjectVar) Loc() *Loc {
	return r.l
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

func (r *runObjectFunc) Run(ctx Context) (*ZVal, error) {
	// fetch object
	obj, err := r.ref.Run(ctx)
	if err != nil {
		return nil, err
	}

	op := r.op
	if op[0] == '$' {
		// variable
		var opz *ZVal
		opz, err = ctx.OffsetGet(ctx, op[1:].ZVal())
		if err != nil {
			return nil, err
		}
		opz, err = opz.As(ctx, ZtString)
		if err != nil {
			return nil, err
		}
		op = opz.Value().(ZString)
	}

	objI, ok := obj.Value().(ObjectCallable)
	if !ok {
		return nil, errors.New("variable is not an object, cannot call method")
	}

	args := make([]*ZVal, len(r.args))

	for i, subr := range r.args {
		args[i], err = subr.Run(ctx)
		if err != nil {
			return nil, err
		}
	}

	// execute call
	return objI.CallMethod(op, ctx, args)
}

func (r *runObjectVar) Run(ctx Context) (*ZVal, error) {
	// fetch object property
	obj, err := r.ref.Run(ctx)
	if err != nil {
		return nil, err
	}

	objI, ok := obj.Value().(ZObjectAccess)
	if !ok {
		// TODO make this a warning
		return nil, errors.New("variable is not an object, cannot fetch property")
	}

	// offset get
	var offt *ZVal
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

func (r *runObjectVar) WriteValue(ctx Context, value *ZVal) error {
	// write object property
	obj, err := r.ref.Run(ctx)
	if err != nil {
		return err
	}

	objI, ok := obj.Value().(ZObjectAccess)
	if !ok {
		// TODO cast to object?
		return errors.New("variable is not an object, cannot set property")
	}

	// offset set
	var offt *ZVal
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

func compileObjectOperator(v Runnable, i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	// call a method or get a variable on an object
	l := MakeLoc(i.Loc())

	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.Type != tokenizer.T_STRING && i.Type != tokenizer.T_VARIABLE {
		return nil, i.Unexpected()
	}
	op := ZString(i.Data)

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

	return compilePostExpr(&runObjectVar{ref: v, varName: op, l: l}, nil, c)
}
