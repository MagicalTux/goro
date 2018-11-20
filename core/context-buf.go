package core

import "io"

type BufContext struct {
	Context
	b io.Writer
}

func NewBufContext(ctx Context, b io.Writer) Context {
	return &BufContext{ctx, b}
}

func (b *BufContext) Write(d []byte) (int, error) {
	return b.b.Write(d)
}
