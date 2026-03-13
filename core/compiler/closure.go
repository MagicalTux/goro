package compiler

import (
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

type ZClosure struct {
	phpv.CallableVal
	name  phpv.ZString
	args  []*phpv.FuncArg
	use   []*phpv.FuncUse
	code  phpv.Runnable
	class phpv.ZClass // class in which this closure was defined (for parent:: and self::)
	start *phpv.Loc
	end   *phpv.Loc
	rref  bool // return ref?
}

// > class Closure
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
		if s.Ref {
			// reference capture: share the same ZVal between outer scope and closure
			if !z.IsRef() {
				ref := z.Ref()
				ctx.OffsetSet(ctx, s.VarName.ZVal(), ref)
				s.Value = ref
			} else {
				s.Value = z
			}
		} else {
			s.Value = z.Nude().Dup()
		}
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

func (z *ZClosure) Loc() *phpv.Loc {
	return z.start
}

func (z *ZClosure) Name() string {
	if z.name == "" {
		if z.start != nil {
			return fmt.Sprintf("{closure:%s:%d}", z.start.Filename, z.start.Line)
		}
		return "{closure}"
	}
	return string(z.name)
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
				funcName := ctx.GetFuncName()
				requiredCount := 0
				for _, arg := range z.args {
					if arg.Required {
						requiredCount++
					}
				}
				// Build the error message with call location info
				msg := fmt.Sprintf("Too few arguments to function %s(), %d passed", funcName, len(args))
				if callLoc := ctx.Loc(); callLoc != nil {
					msg += fmt.Sprintf(" in %s on line %d", callLoc.Filename, callLoc.Line)
				}
				msg += fmt.Sprintf(" and exactly %d expected", requiredCount)
				return nil, phpobj.ThrowErrorAt(ctx, phpobj.ArgumentCountError, msg, z.start)
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
			ctx.OffsetSet(ctx, a.VarName.ZVal(), args[i].Nude().Dup())
		}
	}

	// call function in that context
	_, err = z.code.Run(ctx)
	if err != nil {
		// Check if this is an explicit return
		r, err := phperr.CatchReturn(nil, err)
		if z.rref && r != nil {
			r = r.Ref()
		}
		return r, err
	}
	// No explicit return statement - return NULL
	return phpv.ZNULL.ZVal(), nil
}

func (z *ZClosure) dup() *ZClosure {
	n := &ZClosure{}
	n.code = z.code
	n.name = z.name
	n.class = z.class
	n.start = z.start
	n.end = z.end
	n.rref = z.rref

	if z.args != nil {
		n.args = make([]*phpv.FuncArg, len(z.args))
		for k, v := range z.args {
			n.args[k] = v
		}
	}

	if z.use != nil {
		n.use = make([]*phpv.FuncUse, len(z.use))
		for k, v := range z.use {
			// deep copy so each closure instance has its own FuncUse
			u := *v
			n.use[k] = &u
		}
	}

	return n
}

func (z *ZClosure) GetClass() phpv.ZClass {
	return z.class
}

func (z *ZClosure) ReturnsByRef() bool {
	return z.rref
}
