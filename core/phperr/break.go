package phperr

import (
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/phpv"
)

type PhpBreak struct {
	L       *phpv.Loc
	Intv    phpv.ZInt
	Initial phpv.ZInt
}

func (b *PhpBreak) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	b.Intv = b.Initial
	return nil, b
}

func (b *PhpBreak) Error() string {
	return fmt.Sprintf("'break' not in the 'loop' or 'switch' context in %s at line %d", b.L.Filename, b.L.Line)
}

func (b *PhpBreak) Dump(w io.Writer) error {
	_, err := w.Write([]byte("break"))
	return err
}

type PhpContinue struct {
	L       *phpv.Loc
	Intv    phpv.ZInt
	Initial phpv.ZInt
}

func (c *PhpContinue) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	c.Intv = c.Initial
	return nil, c
}

func (c *PhpContinue) Error() string {
	return fmt.Sprintf("'continue' not in the 'loop' context in %s at line %d", c.L.Filename, c.L.Line)
}

func (c *PhpContinue) Dump(w io.Writer) error {
	_, err := w.Write([]byte("continue"))
	return err
}
