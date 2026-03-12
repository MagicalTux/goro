package compiler

import (
	"io"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runInstanceOf struct {
	v        phpv.Runnable
	l        *phpv.Loc
	c        phpv.ZString  // static class name
	classVar phpv.Runnable // dynamic class name (variable)
}

func compileInstanceOf(v phpv.Runnable, i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	r := &runInstanceOf{l: i.Loc(), v: v}

	// Check if next token is a variable
	next, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if next.Type == tokenizer.T_VARIABLE {
		r.classVar = &runVariable{v: phpv.ZString(next.Data[1:]), l: next.Loc()}
		return r, nil
	}

	c.backup()
	r.c, err = compileClassName(c)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (r *runInstanceOf) Dump(w io.Writer) error {
	err := r.v.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(" instanceof " + string(r.c)))
	return err
}

func (r *runInstanceOf) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	var className phpv.ZString
	if r.classVar != nil {
		// Dynamic class name from variable
		classVal, err := r.classVar.Run(ctx)
		if err != nil {
			return nil, err
		}
		className = classVal.AsString(ctx)
	} else {
		className = r.c
	}

	// first, check class in parameter
	c, err := ctx.Global().GetClass(ctx, className, false)
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	// now check value
	v, err := r.v.Run(ctx)
	if err != nil {
		return nil, err
	}
	if v.GetType() != phpv.ZtObject {
		return phpv.ZBool(false).ZVal(), nil
	}

	o := v.Value().(phpv.ZObject)
	// Use original class, not CurrentClass from GetKin
	objClass := o.GetClass()
	if zo, ok := o.(*phpobj.ZObject); ok {
		objClass = zo.Class
	}
	final := objClass.InstanceOf(c)

	return phpv.ZBool(final).ZVal(), nil
}
