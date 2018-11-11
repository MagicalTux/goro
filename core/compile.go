package core

import (
	"io"

	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

type compileCtx struct {
	Context

	t *tokenizer.Lexer

	next *tokenizer.Item
	last *tokenizer.Item
}

func (c *compileCtx) NextItem() (*tokenizer.Item, error) {
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

func (c *compileCtx) backup() {
	c.next, c.last = c.last, nil
}

func Compile(parent Context, t *tokenizer.Lexer) Runnable {
	c := &compileCtx{
		Context: parent,
		t:       t,
	}

	r, err := compileBase(nil, c)
	if err != nil && err != io.EOF {
		return &PhpError{err, nil, PhpErrorFatal}
	}

	if list, ok := r.(Runnables); ok {
		// check for any function
		for i, elem := range list {
			switch obj := elem.(type) {
			case *ZClosure:
				if obj.name != "" {
					err := c.GetGlobal().RegisterFunction(obj.name, obj)
					if err != nil {
						return obj.Loc().Error(err)
					}
					list[i] = obj.Loc()
				}
			case *ZClass:
				if obj.Name != "" {
					err := c.GetGlobal().RegisterClass(obj.Name, obj)
					if err != nil {
						return obj.Loc().Error(err)
					}
					list[i] = obj.Loc()
				}
			}
		}
	}

	return r
}
