package core

import (
	"io/ioutil"
	"os"

	"git.atonline.com/tristantech/gophp/tokenizer"
)

type Context struct{}

func NewContext() *Context {
	return &Context{}
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

	_ = t // XXX

	return nil
}
