package core

import "io"

type runVariable struct {
	v ZString
	l *Loc
}

func (r *runVariable) Run(ctx Context) (*ZVal, error) {
	res, err := ctx.OffsetGet(ctx, r.v.ZVal())
	return res, err
}

func (r *runVariable) WriteValue(ctx Context, value *ZVal) error {
	err := ctx.OffsetSet(ctx, r.v.ZVal(), value.Dup())
	if err != nil {
		return r.l.Error(err)
	}
	return nil
}

func (r *runVariable) Loc() *Loc {
	return r.l
}

func (r *runVariable) Dump(w io.Writer) error {
	_, err := w.Write([]byte{'$'})
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(r.v))
	return err
}

// reference to an existing [something]
type runRef struct {
	v Runnable
	l *Loc
}

func (r *runRef) Loc() *Loc {
	return r.l
}

func (r *runRef) Run(ctx Context) (*ZVal, error) {
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
