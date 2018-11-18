package core

import "io"

type PhpBreak struct {
	l    *Loc
	intv ZInt
}

func (b *PhpBreak) Run(ctx Context) (*ZVal, error) {
	return nil, b
}

func (b *PhpBreak) Error() string {
	return "'break' not in the 'loop' or 'switch' context"
}

func (b *PhpBreak) Loc() *Loc {
	return b.l
}

func (b *PhpBreak) Dump(w io.Writer) error {
	_, err := w.Write([]byte("break"))
	return err
}

type PhpContinue struct {
	l    *Loc
	intv ZInt
}

func (c *PhpContinue) Run(ctx Context) (*ZVal, error) {
	return nil, c
}

func (c *PhpContinue) Error() string {
	return "'continue' not in the 'loop' context"
}

func (c *PhpContinue) Loc() *Loc {
	return c.l
}

func (c *PhpContinue) Dump(w io.Writer) error {
	_, err := w.Write([]byte("continue"))
	return err
}
