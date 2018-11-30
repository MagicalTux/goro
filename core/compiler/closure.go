package compiler

import (
	"errors"
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

type ZClosure struct {
	name  phpv.ZString
	args  []*phpv.FuncArg
	use   []*phpv.FuncUse
	code  phpv.Runnable
	class phpv.ZClass // class in which this closure was defined (for parent:: and self::)
	start *phpv.Loc
	end   *phpv.Loc
	rref  bool // return ref?
}

//> class Closure
var Closure = &phpobj.ZClass{
	Name: "Closure",
	H:    &phpv.ZClassHandlers{},
}

func init() {
	// put this here to avoid initialization loop problem
	Closure.H.HandleInvoke = func(ctx phpv.Context, o phpv.ZObject, args []phpv.Runnable) (*phpv.ZVal, error) {
		z := o.GetOpaque(Closure).(*ZClosure)
		return ctx.Call(ctx, z, args, o)
	}
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
		// register function
		err = closure.Compile(ctx)
		if err != nil {
			return nil, err
		}
		return nil, ctx.Global().RegisterFunction(closure.name, closure)
	}
	c := closure.dup()
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
		s.Value = z
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

func (z *ZClosure) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	// typically, we run from a clean context
	var err error

	// set use vars
	for _, u := range z.use {
		ctx.OffsetSet(ctx, u.VarName.ZVal(), u.Value)
	}

	// set args in new context
	for i, a := range z.args {
		if len(args) <= i || args[i] == nil {
			if a.Required {
				return nil, errors.New("Uncaught ArgumentCountError: Too few arguments to function toto()")
			}
			if a.DefaultValue != nil {
				if len(args) == i {
					// need to append to args
					args = append(args, nil)
				}
				args[i] = a.DefaultValue.ZVal()
			} else {
				continue
			}
		}
		if args[i].IsRef() {
			ctx.OffsetSet(ctx, a.VarName.ZVal(), args[i].Ref())
		} else {
			ctx.OffsetSet(ctx, a.VarName.ZVal(), args[i].Nude())
		}
	}

	// call function in that context
	r, err := phperr.CatchReturn(z.code.Run(ctx))
	if z.rref && r != nil {
		r = r.Ref()
	}
	return r, err
}

func (z *ZClosure) dup() *ZClosure {
	n := &ZClosure{}
	n.code = z.code

	if z.args != nil {
		n.args = make([]*phpv.FuncArg, len(z.args))
		for k, v := range z.args {
			n.args[k] = v
		}
	}

	if z.use != nil {
		n.use = make([]*phpv.FuncUse, len(z.use))
		for k, v := range z.use {
			n.use[k] = v
		}
	}

	return z
}

func (z *ZClosure) GetClass() phpv.ZClass {
	return z.class
}
