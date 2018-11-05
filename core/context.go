package core

import (
	"context"
	"io"
	"os"

	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

type Context interface {
	context.Context
	io.Writer

	RunFile(string) error
}

type phpContext struct {
	context.Context
	p *Process

	out io.Writer
}

func NewContext(ctx context.Context, p *Process) Context {
	return &phpContext{
		Context: ctx,
		p:       p,
		out:     os.Stdout,
	}
}

func (ctx *phpContext) RunFile(fn string) error {
	f, err := os.Open(fn)
	if err != nil {
		return err
	}

	defer f.Close()

	// tokenize
	t := tokenizer.NewLexer(f)

	// compile
	c := compile(t)

	_, err = c.run(ctx)
	return err
}

func (ctx *phpContext) Write(v []byte) (int, error) {
	return ctx.out.Write(v)
}
