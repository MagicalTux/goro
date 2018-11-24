package core

import (
	"errors"
	"fmt"
	"io"
)

type zclosureCompileCtx struct {
	compileCtx
	closure *ZClosure
}

func (z *zclosureCompileCtx) getFunc() *ZClosure {
	return z.closure
}

type funcArg struct {
	varName      ZString
	ref          bool
	required     bool
	defaultValue Val
	hint         *TypeHint
}

type funcUse struct {
	varName ZString
	value   *ZVal
}

type ZClosure struct {
	name  ZString
	args  []*funcArg
	use   []*funcUse
	code  Runnable
	class *ZClass // class in which this closure was defined (for parent:: and self::)
	start *Loc
	end   *Loc
	rref  bool // return ref?
}

func (z *ZClosure) GetType() ZType {
	return ZtObject
}

func (z *ZClosure) AsVal(ctx Context, t ZType) (Val, error) {
	switch t {
	case ZtObject:
		return z, nil
	case ZtBool:
		return ZBool(true), nil
	}
	return nil, nil
}

func (z *ZClosure) ZVal() *ZVal {
	return &ZVal{z}
}

func (closure *ZClosure) Run(ctx Context) (l *ZVal, err error) {
	err = closure.compile(ctx)
	if err != nil {
		return nil, err
	}
	if closure.name != "" {
		// register function
		return nil, ctx.Global().RegisterFunction(closure.name, closure)
	}
	c := closure.dup()
	// collect use vars
	for _, s := range c.use {
		z, err := ctx.OffsetGet(ctx, s.varName.ZVal())
		if err != nil {
			return nil, err
		}
		s.value = z
	}
	return &ZVal{c}, nil
}

func (c *ZClosure) compile(ctx Context) error {
	for _, a := range c.args {
		if r, ok := a.defaultValue.(*compileDelayed); ok {
			z, err := r.Run(ctx)
			if err != nil {
				return err
			}
			a.defaultValue = z.Value()
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
		if a.ref {
			_, err = w.Write([]byte{'&'})
			if err != nil {
				return err
			}
		}
		_, err = w.Write([]byte{'$'})
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(a.varName))
		if err != nil {
			return err
		}
		if a.defaultValue != nil {
			_, err = w.Write([]byte{'='})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(w, "%#v", a.defaultValue) // TODO
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

func (z *ZClosure) Loc() *Loc {
	return z.start
}

func (z *ZClosure) getArgs() []*funcArg {
	return z.args
}

func (z *ZClosure) Call(ctx Context, args []*ZVal) (*ZVal, error) {
	// typically, we run from a clean context
	var err error

	// set use vars
	for _, u := range z.use {
		ctx.OffsetSet(ctx, u.varName.ZVal(), u.value)
	}

	// set args in new context
	for i, a := range z.args {
		if len(args) <= i || args[i] == nil {
			if a.required {
				return nil, errors.New("Uncaught ArgumentCountError: Too few arguments to function toto()")
			}
			if a.defaultValue != nil {
				if len(args) == i {
					// need to append to args
					args = append(args, nil)
				}
				args[i] = a.defaultValue.ZVal()
			} else {
				continue
			}
		}
		if args[i].IsRef() {
			ctx.OffsetSet(ctx, a.varName.ZVal(), args[i].Ref())
		} else {
			ctx.OffsetSet(ctx, a.varName.ZVal(), args[i].Nude())
		}
	}

	// call function in that context
	r, err := z.code.Run(ctx)
	if z.rref && r != nil {
		r = r.Ref()
	}
	return r, err
}

func (z *ZClosure) dup() *ZClosure {
	n := &ZClosure{}
	n.code = z.code

	if z.args != nil {
		n.args = make([]*funcArg, len(z.args))
		for k, v := range z.args {
			n.args[k] = v
		}
	}

	if z.use != nil {
		n.use = make([]*funcUse, len(z.use))
		for k, v := range z.use {
			n.use[k] = v
		}
	}

	return z
}
