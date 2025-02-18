package compiler

import (
	"io"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runVariable struct {
	runnableChild
	v phpv.ZString
	l *phpv.Loc
}

type runVariableRef struct {
	v phpv.Runnable
	l *phpv.Loc
}

func (rv *runVariable) VarName() phpv.ZString {
	return rv.v
}

func (rv *runVariable) IsUnDefined(ctx phpv.Context) bool {
	exists, _ := ctx.OffsetExists(ctx, rv.v)
	return !exists
}

func compileRunVariableRef(i *tokenizer.Item, c compileCtx, l *phpv.Loc) (phpv.Runnable, error) {
	r := &runVariableRef{l: l}
	var err error

	if i == nil {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	if i.Type == tokenizer.Rune('{') {
		r.v, err = compileExpr(nil, c)
		if err != nil {
			return nil, err
		}

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.Type != tokenizer.Rune('}') {
			return nil, i.Unexpected()
		}
	} else {
		r.v, err = compileOneExpr(i, c)
		if err != nil {
			return nil, err
		}
	}

	return r, nil
}

func (r *runVariable) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	err := ctx.Tick(ctx, r.l)
	if err != nil {
		return nil, err
	}

	varName := r.v.String()
	if varName == "this" && ctx.This() == nil {
		return nil, ctx.Errorf("Using $this when not in object context")
	}

	res, exists, err := ctx.OffsetCheck(ctx, r.v.ZVal())
	if err != nil {
		return nil, err
	}

	if !exists {
		write := false
		switch t := r.Parent.(type) {
		case *runOperator:
			write = t.opD.write
		case *runArrayAccess, *runnableForeach, *runDestructure:
			write = true
		case *runnableFunctionCall:
			// functions that take references can be considered as "write",
			// but the param info is not available here, just assume
			// write is true, and let the functions themselves
			// check for undefined variables.
			write = true
		}

		if !write {
			if err := ctx.Notice("Undefined variable: %s",
				varName, logopt.NoFuncName(true)); err != nil {
				return phpv.ZNULL.ZVal(), err
			}
		}
	}

	if res == nil {
		res := phpv.NewZVal(phpv.ZNULL)
		res.Name = &r.v
		return res, nil
	}

	v := res.Nude()
	v.Name = &r.v
	return v, nil
}

func (r *runVariable) WriteValue(ctx phpv.Context, value *phpv.ZVal) error {
	var err error
	if value == nil {
		err = ctx.OffsetUnset(ctx, r.v.ZVal())
	} else {
		err = ctx.OffsetSet(ctx, r.v.ZVal(), value)
	}
	if err != nil {
		return r.l.Error(ctx, err)
	}
	return nil
}

func (r *runVariable) Dump(w io.Writer) error {
	_, err := w.Write([]byte{'$'})
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(r.v))
	return err
}

func (r *runVariableRef) Dump(w io.Writer) error {
	_, err := w.Write([]byte("${"))
	if err != nil {
		return err
	}
	err = r.v.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{'}'})
	return err
}

func (r *runVariableRef) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	v, err := r.v.Run(ctx)
	if err != nil {
		return nil, err
	}
	name := phpv.ZString(v.String())
	v, err = ctx.OffsetGet(ctx, v)
	if v != nil {
		v = v.Nude()
	}
	v.Name = &name
	return v, err
}

func (r *runVariableRef) WriteValue(ctx phpv.Context, value *phpv.ZVal) error {
	var err error
	v, err := r.v.Run(ctx)
	if err != nil {
		return err
	}

	if value == nil {
		err = ctx.OffsetUnset(ctx, v)
	} else {
		err = ctx.OffsetSet(ctx, v, value)
	}
	if err != nil {
		return r.l.Error(ctx, err)
	}
	return nil
}

// reference to an existing [something]
type runRef struct {
	v phpv.Runnable
	l *phpv.Loc
}

func (r *runRef) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	z, err := r.v.Run(ctx)
	if err != nil {
		return nil, err
	}
	// embed zval into another zval
	return z.Ref(), nil
}

func (r *runRef) Dump(w io.Writer) error {
	_, err := w.Write([]byte{'&'})
	if err != nil {
		return err
	}
	return r.v.Dump(w)
}
