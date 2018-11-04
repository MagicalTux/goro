package core

import (
	"errors"
	"io/ioutil"
	"log"
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

	// test
	for {
		x, v := t.NextItem()
		if x == tokenizer.T_EOF {
			break
		}
		if x == tokenizer.ItemError {
			return errors.New(v)
		}

		log.Printf("got token %s %q", x, v)
	}

	return nil
}
