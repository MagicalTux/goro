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
	args []*funcArg
	use  []*funcUse
	code Runnable
}

func (z *ZClosure) GetType() ZType {
	return ZtObject
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
