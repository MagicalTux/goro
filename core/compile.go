package core

import (
	"io"

	"github.com/MagicalTux/gophp/core/tokenizer"
)

type compileCtx interface {
	Context

	ExpectSingle(r rune) error
	NextItem() (*tokenizer.Item, error)
	backup()
	getClass() *ZClass
	peekType() tokenizer.ItemType
}

type compileRootCtx struct {
	Context

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

func (c *compileRootCtx) getClass() *ZClass {
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

func Compile(parent Context, t *tokenizer.Lexer) (Runnable, error) {
	c := &compileRootCtx{
		Context: parent,
		t:       t,
	}

	r, err := compileBaseUntil(nil, c, tokenizer.T_EOF)
	if err != nil && err != io.EOF {
		return nil, err
	}

	if list, ok := r.(Runnables); ok {
		// check for any function
		for i, elem := range list {
			switch obj := elem.(type) {
			case *ZClosure:
				if obj.name != "" {
					err := c.Global().RegisterFunction(obj.name, obj)
					if err != nil {
						return nil, obj.Loc().Error(err)
					}
					list[i] = obj.Loc()
				}
			case *ZClass:
				if obj.Name != "" {
					err := c.Global().RegisterClass(obj.Name, obj)
					if err != nil {
						return nil, obj.Loc().Error(err)
					}
					list[i] = obj.Loc()
				}
			}
		}
	}

	return r, nil
}
