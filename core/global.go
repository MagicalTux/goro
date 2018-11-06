package core

import (
	"context"
	"io"
	"os"

	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

type Global struct {
	context.Context
	p *Process

	out io.Writer
}

func NewGlobal(ctx context.Context, p *Process) *Global {
	return &Global{
		Context: ctx,
		p:       p,
		out:     os.Stdout,
	}
}

func (g *Global) RunFile(fn string) error {
	f, err := os.Open(fn)
	if err != nil {
		return err
	}

	defer f.Close()

	// tokenize
	t := tokenizer.NewLexer(f, fn)

	// compile
	c := compile(t)

	ctx := NewContext(g)

	_, err = c.run(ctx)
	return err
}

func (g *Global) Write(v []byte) (int, error) {
	return g.out.Write(v)
}

func (g *Global) GetVariable(name string) (*ZVal, error) {
	// TODO
	return nil, nil
}
