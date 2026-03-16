package compiler

import (
	"io"

	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runnableUnset struct {
	args phpv.Runnables
	l    *phpv.Loc
}

func (r *runnableUnset) Dump(w io.Writer) error {
	_, err := w.Write([]byte("unset("))
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

func (r *runnableUnset) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	for _, v := range r.args {
		if x, ok := v.(phpv.Writable); ok {
			// Skip reading the value for certain types:
			// - ArrayAccess: unset should only call offsetUnset, not offsetGet
			// - Object properties: reading triggers checkStaticPropertyAccess which
			//   would cause a duplicate notice (WriteValue also triggers it)
			_, isArrayAccess := v.(*runArrayAccess)
			_, isObjectVar := v.(*runObjectVar)
			if !isArrayAccess && !isObjectVar {
				zv, runErr := v.Run(ctx)
				if runErr != nil {
					return nil, runErr
				}
				if err := callDestructorIfNeeded(ctx, zv); err != nil {
					return nil, err
				}
			}
			if err := x.WriteValue(ctx, nil); err != nil {
				return nil, err
			}
		} else {
			return nil, ctx.Errorf("unable to unset value")
		}
	}
	return nil, nil
}

// callDestructorIfNeeded checks if a ZVal holds an object with __destruct,
// and if so, decrements the reference count and calls the destructor if
// the count reaches zero.
func callDestructorIfNeeded(ctx phpv.Context, zv *phpv.ZVal) error {
	if zv == nil || zv.GetType() != phpv.ZtObject {
		return nil
	}
	obj := zv.Value()
	// Call HandleDecRef if the class defines it (e.g. Closure releasing captured $this)
	if zobj, ok := obj.(phpv.ZObject); ok {
		if cls := zobj.GetClass(); cls != nil {
			if h := cls.Handlers(); h != nil && h.HandleDecRef != nil {
				h.HandleDecRef(ctx, zobj)
			}
		}
	}
	if refObj, ok := obj.(interface {
		DecRef(phpv.Context) error
	}); ok {
		return refObj.DecRef(ctx)
	}
	return nil
}

func compileUnset(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	var err error
	un := &runnableUnset{l: i.Loc()}
	un.args, err = compileFuncPassedArgs(c)
	return un, err
}
