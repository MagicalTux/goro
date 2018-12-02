package compiler

import (
	"io"

	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runInstanceOf struct {
	v phpv.Runnable
	l *phpv.Loc
	c phpv.ZString
}

func compileInstanceOf(v phpv.Runnable, i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	var err error
	r := &runInstanceOf{l: i.Loc(), v: v}

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
	// first, check class in parameter
	c, err := ctx.Global().GetClass(ctx, r.c, false)
	if err != nil {
		// the only error possible is if class does not exists (because autoload=false, with autoload=true the autoload function could throw an error/etc)
		// if a class is not loaded or does not exist, an existing object cannot extend it (yet)
		// TODO check against object classes in the future once objects can be passed between processes
		return phpv.ZBool(false).ZVal(), nil
	}

	// now check value
	v, err := r.v.Run(ctx)
	if err != nil {
		return nil, err
	}
	if v.GetType() != phpv.ZtObject {
		// not an object
		return phpv.ZBool(false).ZVal(), nil
	}

	o := v.Value().(phpv.ZObject)
	final := o.GetClass().InstanceOf(c)

	return phpv.ZBool(final).ZVal(), nil
}
