package core

import (
	"io"
)

type runIncDec struct {
	inc  bool // if true: increase
	post bool // if true, return value before execution
	v    Runnable
	l    *Loc
}

func (r *runIncDec) Loc() *Loc {
	return r.l
}

func (r *runIncDec) Dump(w io.Writer) error {
	var err error

	if r.post {
		err = r.v.Dump(w)
		if err != nil {
			return err
		}
		if r.inc {
			_, err = w.Write([]byte("++"))
		} else {
			_, err = w.Write([]byte("--"))
		}
		return err
	} else {
		if r.inc {
			_, err = w.Write([]byte("++"))
		} else {
			_, err = w.Write([]byte("--"))
		}
		if err != nil {
			return err
		}
		return r.v.Dump(w)
	}
}

func (r *runIncDec) Run(ctx Context) (*ZVal, error) {
	w, ok := r.v.(Writable)
	if !ok {
		return nil, r.Loc().Errorf("invalid operator for value")
	}

	v, err := r.v.Run(ctx)
	if err != nil {
		return nil, r.l.Error(err)
	}

	v = v.Dup()
	original := v

	v, err = v.AsNumeric(ctx)
	if err != nil {
		return nil, err
	}

	var res Val
	switch n := v.Value().(type) {
	case ZInt:
		if r.inc {
			res = n + 1
		} else {
			res = n - 1
		}
	case ZFloat:
		if r.inc {
			res = n + 1
		} else {
			res = n - 1
		}
	default:
		return nil, r.l.Errorf("could not handle type %T", v.v)
	}

	if r.post {
		// return original value
		w.WriteValue(ctx, res.ZVal())
		return original, nil
	} else {
		// return updated value
		v = res.ZVal()
		w.WriteValue(ctx, v)
		return v, nil
	}
}
