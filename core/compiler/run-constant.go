package compiler

import (
	"errors"
	"io"
	"strings"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

type runConstant struct {
	c string
	l *phpv.Loc
}

func (r *runConstant) Dump(w io.Writer) error {
	_, err := w.Write([]byte(r.c))
	return err
}

func (r *runConstant) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	switch strings.ToLower(string(r.c)) {
	case "null":
		return phpv.ZNull{}.ZVal(), nil
	case "true":
		return phpv.ZBool(true).ZVal(), nil
	case "false":
		return phpv.ZBool(false).ZVal(), nil
	case "self":
		if ctx.This() == nil {
			return nil, errors.New("cannot access self:: when no class scope is active")
		}
		return ctx.This().ZVal(), nil
	case "parent":
		if ctx.This() == nil {
			return nil, errors.New("cannot access parent:: when no class scope is active")
		}
		parentClass := ctx.This().GetClass().GetParent()

		obj, err := phpobj.NewZObject(ctx, parentClass)
		if err != nil {
			return nil, err
		}
		return obj.ZVal(), nil
	}

	z, ok := ctx.Global().ConstantGet(phpv.ZString(r.c))

	if !ok {
		// TODO issue warning Use of undefined constant tata - assumed 'tata' (this will throw an Error in a future version of PHP)
		return phpv.ZString(r.c).ZVal(), nil
	}
	return z.ZVal(), nil
}
