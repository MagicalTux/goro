package core

import (
	"context"
	"io/ioutil"
	"os"

	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

type Context struct {
	context.Context
	p *Process
}

func NewContext(ctx context.Context, p *Process) *Context {
	return &Context{ctx, p}
}

func (ctx *Context) RunFile(fn string) error {
	f, err := os.Open(fn)
	if err != nil {
		return err
	}

	defer f.Close()

	// read whole file
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	// tokenize
	t := tokenizer.NewLexer(b)

	// compile
	c := compile(t)

	_, err = c.run(ctx)
	return err
}
