package core

import (
	"io"

	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runnableFunctionCall struct {
	name phpv.ZString
	args []phpv.Runnable
	l    *phpv.Loc
}

type runnableFunctionCallRef struct {
	name phpv.Runnable
	args []phpv.Runnable
	l    *phpv.Loc
}

type funcGetArgs interface {
	getArgs() []*funcArg
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

func (r *runnableFunctionCall) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	err = ctx.Tick(ctx, r.l)
	if err != nil {
		return nil, err
	}
	// grab function
	f, err := ctx.Global().(*Global).GetFunction(ctx, r.name)
	if err != nil {
		return nil, err
	}

	return ctx.Call(ctx, f, r.args, nil)
}

func (r *runnableFunctionCallRef) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	var f phpv.Callable
	var ok bool

	err = ctx.Tick(ctx, r.l)
	if err != nil {
		return nil, err
	}

	if f, ok = r.name.(phpv.Callable); !ok {
		v, err := r.name.Run(ctx)
		if err != nil {
			return nil, err
		}

		if f, ok := v.Value().(*ZObject); ok && f.Class.HandleInvoke != nil {
			return f.Class.HandleInvoke(ctx, f, r.args)
		}

		if f, ok = v.Value().(phpv.Callable); !ok {
			v, err = v.As(ctx, phpv.ZtString)
			if err != nil {
				return nil, err
			}
			// grab function
			f, err = ctx.Global().(*Global).GetFunction(ctx, v.Value().(phpv.ZString))
			if err != nil {
				return nil, err
			}
		}
	}

	return ctx.Call(ctx, f, r.args, nil)
}

func compileFunction(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// typically T_FUNCTION is followed by:
	// - a name and parameters → this is a regular function
	// - directly parameters → this is a lambda function
	l := phpv.MakeLoc(i.Loc())

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
		f, err := compileFunctionWithName(phpv.ZString(i.Data), c, l, rref)
		if err != nil {
			return nil, err
		}
		return f, nil
	case tokenizer.Rune('('):
		// function with no name is lambda
		c.backup()
		f, err := compileFunctionWithName("", c, l, rref)
		if err != nil {
			return nil, err
		}
		return f, nil
	}

	return nil, i.Unexpected()
}

func compileSpecialFuncCall(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// special function call that comes without (), so as a keyword. Example: echo, die, etc
	has_open := false
	fn_name := phpv.ZString(i.Data)
	l := phpv.MakeLoc(i.Loc())

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

	var args []phpv.Runnable

	// parse passed arguments
	for {
		var a phpv.Runnable
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

func compileFunctionWithName(name phpv.ZString, c compileCtx, l *phpv.Loc, rref bool) (*ZClosure, error) {
	var err error

	zc := &ZClosure{
		name:  name,
		start: l,
		// TODO populate end
		rref: rref,
	}

	c = &zclosureCompileCtx{c, zc}

	args, err := compileFunctionArgs(c)
	if err != nil {
		return nil, err
	}
	zc.args = args

	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.Type == tokenizer.T_USE && name == "" {
		// anonymous function variables
		zc.use, err = compileFunctionUse(c)
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

	zc.code, err = compileBase(nil, c)
	if err != nil {
		return nil, err
	}

	return zc, nil
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

			arg.hint = ParseTypeHint(phpv.ZString(hint))
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

		arg.varName = phpv.ZString(i.Data[1:]) // skip $

		res = append(res, arg)

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle('=') {
			// we have a default value
			r, err := compileExpr(nil, c)
			if err != nil {
				return nil, err
			}
			arg.defaultValue = &compileDelayed{r}
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

		res = append(res, &funcUse{varName: phpv.ZString(i.Data[1:])}) // skip $

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
		var a phpv.Runnable
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
