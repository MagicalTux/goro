package core

import (
	"io"

	"github.com/MagicalTux/gophp/core/tokenizer"
)

type runnableFunctionCall struct {
	name ZString
	args []Runnable
	l    *Loc
}

type runnableFunctionCallRef struct {
	name Runnable
	args []Runnable
	l    *Loc
}

type funcGetArgs interface {
	getArgs() []*funcArg
}

func (r *runnableFunctionCall) Loc() *Loc {
	return r.l
}

func (r *runnableFunctionCall) Dump(w io.Writer) error {
	_, err := w.Write([]byte(r.name))
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{'('})
	if err != nil {
		return err
	}
	// args
	first := true
	for _, a := range r.args {
		if !first {
			_, err = w.Write([]byte{','})
			if err != nil {
				return err
			}
		}
		first = false
		err = a.Dump(w)
		if err != nil {
			return err
		}
	}
	_, err = w.Write([]byte{')'})
	return err
}

func (r *runnableFunctionCallRef) Loc() *Loc {
	return r.l
}

func (r *runnableFunctionCallRef) Dump(w io.Writer) error {
	err := r.name.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{'('})
	if err != nil {
		return err
	}
	// args
	first := true
	for _, a := range r.args {
		if !first {
			_, err = w.Write([]byte{','})
			if err != nil {
				return err
			}
		}
		first = false
		err = a.Dump(w)
		if err != nil {
			return err
		}
	}
	_, err = w.Write([]byte{')'})
	return err
}

func (r *runnableFunctionCall) Run(ctx Context) (l *ZVal, err error) {
	// grab function
	f, err := ctx.Global().GetFunction(r.name)
	if err != nil {
		return nil, err
	}

	return ctx.Call(ctx, f, r.args, nil)
}

func (r *runnableFunctionCallRef) Run(ctx Context) (l *ZVal, err error) {
	var f Callable
	var ok bool

	if f, ok = r.name.(Callable); !ok {
		v, err := r.name.Run(ctx)
		if err != nil {
			return nil, err
		}

		if f, ok = v.v.(Callable); !ok {
			v, err = v.As(ctx, ZtString)
			if err != nil {
				return nil, err
			}
			// grab function
			f, err = ctx.Global().GetFunction(v.Value().(ZString))
			if err != nil {
				return nil, err
			}
		}
	}

	return ctx.Call(ctx, f, r.args, nil)
}

func compileFunction(i *tokenizer.Item, c compileCtx) (Runnable, error) {
	// typically T_FUNCTION is followed by:
	// - a name and parameters → this is a regular function
	// - directly parameters → this is a lambda function
	l := MakeLoc(i.Loc())

	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	rref := false
	if i.IsSingle('&') {
		// this is a ref return function
		rref = true

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	switch i.Type {
	case tokenizer.T_STRING:
		// regular function definition
		return compileFunctionWithName(ZString(i.Data), c, l, rref)
	case tokenizer.Rune('('):
		// function with no name is lambda
		c.backup()
		return compileFunctionWithName("", c, l, rref)
	}

	return nil, i.Unexpected()
}

func compileSpecialFuncCall(i *tokenizer.Item, c compileCtx) (Runnable, error) {
	// special function call that comes without (), so as a keyword. Example: echo, die, etc
	has_open := false
	fn_name := ZString(i.Data)
	l := MakeLoc(i.Loc())

	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.IsSingle(';') {
		c.backup()
		return &runnableFunctionCall{fn_name, nil, l}, nil
	}

	if i.IsSingle('(') {
		has_open = true
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(')') {
			return &runnableFunctionCall{fn_name, nil, l}, nil
		}
		if i.IsSingle(';') {
			c.backup()
			return &runnableFunctionCall{fn_name, nil, l}, nil
		}
	}

	var args []Runnable

	// parse passed arguments
	for {
		var a Runnable
		a, err = compileExpr(i, c)
		if err != nil {
			return nil, err
		}

		args = append(args, a)

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(',') {
			// read and parse next argument
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			continue
		}
		if has_open && i.IsSingle(')') {
			return &runnableFunctionCall{fn_name, args, l}, nil
		}
		if !has_open && i.IsExpressionEnd() {
			c.backup()
			return &runnableFunctionCall{fn_name, args, l}, nil
		}

		return nil, i.Unexpected()
	}
}

func compileFunctionWithName(name ZString, c compileCtx, l *Loc, rref bool) (*ZClosure, error) {
	var err error
	var use []*funcUse
	args, err := compileFunctionArgs(c)
	if err != nil {
		return nil, err
	}

	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.Type == tokenizer.T_USE && name == "" {
		// anonymous function variables
		use, err = compileFunctionUse(c)
		if err != nil {
			return nil, err
		}

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	if !i.IsSingle('{') {
		return nil, i.Unexpected()
	}

	body, err := compileBase(nil, c)
	if err != nil {
		return nil, err
	}

	return &ZClosure{
		name:  name,
		use:   use,
		args:  args,
		code:  body,
		start: l,
		// TODO populate end
		rref: rref,
	}, nil
}

func compileFunctionArgs(c compileCtx) (res []*funcArg, err error) {
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if !i.IsSingle('(') {
		return nil, i.Unexpected()
	}

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.IsSingle(')') {
		return
	}

	// parse arguments
	for {
		arg := &funcArg{}
		arg.required = true // typically

		if i.Type == tokenizer.T_STRING {
			// this is a function parameter type hint
			hint := i.Data

			for {
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}

				if i.Type != tokenizer.T_NS_SEPARATOR {
					break
				}

				// going to be a ns there!
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
				if i.Type != tokenizer.T_STRING {
					// ending with a ns_separator?
					return nil, i.Unexpected()
				}
				hint = hint + "\\" + i.Data
			}

			arg.hint = ParseTypeHint(ZString(hint))
		}

		if i.IsSingle('&') {
			arg.ref = true
			i, err = c.NextItem()
			if err != nil {
				return
			}
		}
		// in a function delcaration, we must have a T_VARIABLE now
		if i.Type != tokenizer.T_VARIABLE {
			return nil, i.Unexpected()
		}

		arg.varName = ZString(i.Data[1:]) // skip $

		res = append(res, arg)

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle('=') {
			// we have a default value
			arg.defaultValue, err = compileExpr(nil, c)
			if err != nil {
				return nil, err
			}
			arg.required = false

			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
		}

		if i.IsSingle(',') {
			// read and parse next argument
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			continue
		}

		if i.IsSingle(')') {
			return // end of arguments
		}

		return nil, i.Unexpected()
	}
}

func compileFunctionUse(c compileCtx) (res []*funcUse, err error) {
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if !i.IsSingle('(') {
		return nil, i.Unexpected()
	}

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.IsSingle(')') {
		return
	}

	// parse arguments
	for {
		// in a function delcaration, we must have a T_VARIABLE now
		if i.Type != tokenizer.T_VARIABLE {
			return nil, i.Unexpected()
		}

		res = append(res, &funcUse{varName: ZString(i.Data[1:])}) // skip $

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(',') {
			// read and parse next argument
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			continue
		}

		if i.IsSingle(')') {
			return // end of arguments
		}

		return nil, i.Unexpected()
	}
}

func compileFuncPassedArgs(c compileCtx) (res Runnables, err error) {
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if !i.IsSingle('(') {
		return nil, i.Unexpected()
	}

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.IsSingle(')') {
		return
	}

	// parse passed arguments
	for {
		var a Runnable
		a, err = compileExpr(i, c)
		if err != nil {
			return nil, err
		}

		res = append(res, a)

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(',') {
			// read and parse next argument
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			continue
		}

		if i.IsSingle(')') {
			return // end of arguments
		}

		return nil, i.Unexpected()
	}
}
