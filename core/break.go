package core

import (
	"io"

	"github.com/MagicalTux/goro/core/phpv"
)

type PhpBreak struct {
	l    *phpv.Loc
	intv phpv.ZInt
}

func (b *PhpBreak) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	return nil, b
}

func (b *PhpBreak) Error() string {
	return "'break' not in the 'loop' or 'switch' context"
}

func (b *PhpBreak) Dump(w io.Writer) error {
	_, err := w.Write([]byte("break"))
	return err
}

type PhpContinue struct {
	l    *phpv.Loc
	intv phpv.ZInt
}

func (c *PhpContinue) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	return nil, c
}

func (c *PhpContinue) Error() string {
	return "'continue' not in the 'loop' context"
}

func (c *PhpContinue) Dump(w io.Writer) error {
	_, err := w.Write([]byte("continue"))
	return err
}
