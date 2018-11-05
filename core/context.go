package core

import (
	"context"
	"io"
	"io/ioutil"
	"log"
	"os"

	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

type Context struct {
	context.Context
}

func NewContext(ctx context.Context) *Context {
	return &Context{ctx}
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

	// test
	for {
		i, err := t.NextItem()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		log.Printf("%d: %s %q", i.Line, i.Type, i.Data)
	}

	return nil
}
