package core

import (
	"errors"
)

type funcArg struct {
	varName      string
	required     bool
	defaultValue runnable
}

type ZClosure struct {
	args []*funcArg
	code runnable
}

func (z *ZClosure) GetType() ZType {
	return ZtObject
}

func (z *ZClosure) Call(parent Context, args []*ZVal) (*ZVal, error) {
	ctx := NewContext(parent) // function context
	var err error

	// set args in new context
	for i, a := range z.args {
		if args[i] == nil {
			if a.required {
				return nil, errors.New("Uncaught ArgumentCountError: Too few arguments to function toto()")
			}
			if a.defaultValue != nil {
				args[i], err = a.defaultValue.run(ctx)
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
	return z.code.run(ctx)
}
