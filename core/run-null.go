package core

import (
	"io"

	"github.com/MagicalTux/goro/core/phpv"
)

type RunNull struct{}

func (r RunNull) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	return phpv.ZNULL.ZVal(), nil
}

func (r RunNull) Dump(w io.Writer) error {
	return nil
}
