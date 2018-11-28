package core

import (
	"io"

	"github.com/MagicalTux/goro/core/phpv"
)

type BufContext struct {
	phpv.Context
	b io.Writer
}

func NewBufContext(ctx phpv.Context, b io.Writer) phpv.Context {
	return &BufContext{ctx, b}
}

func (b *BufContext) Write(d []byte) (int, error) {
	if b.b == nil {
		return len(d), nil
	}
	return b.b.Write(d)
}
