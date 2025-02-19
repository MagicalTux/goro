package compiler

import (
	"errors"
	"io"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runnableIsset struct {
	args phpv.Runnables
	l    *phpv.Loc
}

func (r *runnableIsset) Dump(w io.Writer) error {
	_, err := w.Write([]byte("isset("))
	if err != nil {
		return err
	}
	err = r.args.DumpWith(w, []byte{','})
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{')'})
	return err
}

func (r *runnableIsset) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	for _, v := range r.args {
		exists, err := checkExistence(ctx, v, false)
		if !exists || err != nil {
			return phpv.ZBool(false).ZVal(), err
		}
	}
	return phpv.ZBool(true).ZVal(), nil
}

func compileIsset(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	var err error
	is := &runnableIsset{l: i.Loc()}
	is.args, err = compileFuncPassedArgs(c)
	return is, err
}

func checkExistence(ctx phpv.Context, v phpv.Runnable, subExpr bool) (bool, error) {
	// isset should only evaluate the sub-expressions:
	// - isset(foo()) // foo is not evaluated
	// - isset((foo())['x']) // foo is evaluated
	// - isset(($x[foo()]) // foo is evaluated
	// - isset($x->foo()) // foo is not evaluated
	switch t := v.(type) {
	case *runVariable:
		return ctx.OffsetExists(ctx, t.v.ZVal())

	case *runArrayAccess:
		exists, err := checkExistence(ctx, t.value, true)
		if !exists || err != nil {
			return exists, err
		}
		value, err := t.value.Run(ctx)
		if err != nil {
			return false, err
		}

		if t.offset == nil {
			return false, errors.New("Cannot use [] for reading")
		}
		key, err := t.offset.Run(ctx)
		if err != nil {
			return false, nil
		}

		var arr phpv.ZArrayAccess
		if value.GetType() == phpv.ZtString {
			if key.GetType() != phpv.ZtInt {
				return false, nil
			}
			str := value.AsString(ctx)
			arr = phpv.ZStringArray{ZString: &str}
		} else {
			var ok bool
			arr, ok = value.Value().(phpv.ZArrayAccess)
			if !ok {
				return false, nil
			}
		}

		return arr.OffsetExists(ctx, key)

	case *runObjectVar:
		exists, err := checkExistence(ctx, t.ref, true)
		if !exists || err != nil {
			return exists, err
		}
		value, err := t.ref.Run(ctx)
		if err != nil {
			return false, err
		}
		if value.GetType() != phpv.ZtObject {
			return false, nil
		}
		obj := value.AsObject(ctx).(*phpobj.ZObject)
		return obj.HasProp(ctx, t.varName)

	default:
		if !subExpr {
			return false, ctx.Errorf(`Cannot use isset() on the result of an expression (you can use "null !== expression" instead)`)
		}
		return true, nil
	}
}
