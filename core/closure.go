package core

import (
	"errors"
)

type funcArg struct {
	varName      ZString
	required     bool
	defaultValue Runnable
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
	start *Loc
	end   *Loc
}

func (z *ZClosure) GetType() ZType {
	return ZtObject
}

func (z *ZClosure) As(ctx Context, t ZType) (Val, error) {
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
	if closure.name != "" {
		// register function
		return nil, ctx.GetGlobal().RegisterFunction(closure.name, closure)
	}
	c := closure.dup()
	// collect use vars
	for _, s := range c.use {
		z, err := ctx.GetVariable(s.varName)
		if err != nil {
			return nil, err
		}
		s.value = z
	}
	return &ZVal{c}, nil
}

func (z *ZClosure) Loc() *Loc {
	return z.start
}

func (z *ZClosure) Call(parent Context, args []*ZVal) (*ZVal, error) {
	ctx := NewContext(parent) // function context
	var err error

	// set use vars
	for _, u := range z.use {
		ctx.SetVariable(u.varName, u.value)
	}

	// set args in new context
	for i, a := range z.args {
		if args[i] == nil {
			if a.required {
				return nil, errors.New("Uncaught ArgumentCountError: Too few arguments to function toto()")
			}
			if a.defaultValue != nil {
				args[i], err = a.defaultValue.Run(ctx)
				if err != nil {
					return nil, err
				}
			} else {
				continue
			}
		}
		ctx.SetVariable(a.varName, args[i])
	}

	// call function in that context
	return z.code.Run(ctx)
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
