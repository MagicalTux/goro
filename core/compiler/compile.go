package compiler

import (
	"io"

	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type compileCtx interface {
	phpv.Context

	ExpectSingle(r rune) error
	NextItem() (*tokenizer.Item, error)
	backup()
	getClass() *phpobj.ZClass
	getFunc() *ZClosure
	peekType() tokenizer.ItemType
}

type compileRootCtx struct {
	phpv.Context

	t *tokenizer.Lexer

	next *tokenizer.Item
	last *tokenizer.Item
}

func (c *compileRootCtx) ExpectSingle(r rune) error {
	// read one item, check if rune, if not fallback & return error
	i, err := c.NextItem()
	if err != nil {
		return err
	}

	if !i.IsSingle(r) {
		c.backup()
		return i.Unexpected()
	}
	return nil
}

func (c *compileRootCtx) getClass() *phpobj.ZClass {
	return nil
}

func (c *compileRootCtx) getFunc() *ZClosure {
	return nil
}

func (c *compileRootCtx) peekType() tokenizer.ItemType {
	if c.next != nil {
		return c.next.Type
	}

	n, err := c.NextItem()
	if err != nil {
		return -1
	}
	c.backup()
	return n.Type
}

func (c *compileRootCtx) NextItem() (*tokenizer.Item, error) {
	if c.next != nil {
		c.last, c.next = c.next, nil
		return c.last, nil
	}
	for {
		i, err := c.t.NextItem()

		if err != nil {
			return i, err
		}

		switch i.Type {
		case tokenizer.T_WHITESPACE:
		case tokenizer.T_COMMENT:
		default:
			c.last = i
			return i, err
		}
	}
}

func (c *compileRootCtx) backup() {
	c.next, c.last = c.last, nil
}

func init() {
	phpctx.Compile = Compile
}

func Compile(parent phpv.Context, t *tokenizer.Lexer) (phpv.Runnable, error) {
	c := &compileRootCtx{
		Context: parent,
		t:       t,
	}

	r, err := compileBaseUntil(nil, c, tokenizer.T_EOF)
	if err != nil && err != io.EOF {
		return nil, err
	}

	if list, ok := r.(phpv.Runnables); ok {
		// check for any function
		for i, elem := range list {
			switch obj := elem.(type) {
			case *ZClosure:
				if obj.name != "" {
					c.Global().RegisterLazyFunc(obj.name, list, i)
				}
			case *phpobj.ZClass:
				// TODO first index classes by name, instanciate in right order
				if obj.Name != "" {
					c.Global().RegisterLazyClass(obj.Name, list, i)
				}
			}
		}
	}

	return r, nil
}
