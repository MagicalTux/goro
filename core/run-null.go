package core

import "io"

type RunNull struct{}

func (r RunNull) Run(ctx Context) (*ZVal, error) {
	return ZNULL.ZVal(), nil
}

func (r RunNull) Dump(w io.Writer) error {
	return nil
}
