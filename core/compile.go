package core

import "git.atonline.com/tristantech/gophp/core/tokenizer"

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

func compile(parent Context, t *tokenizer.Lexer) runnable {
	c := &compileCtx{
		Context: parent,
		t:       t,
	}

	r, err := compileBase(nil, c)
	if err != nil {
		return phperror{err}
	}

	if list, ok := r.(runnables); ok {
		// check for any function
		for _, elem := range list {
			switch obj := elem.(type) {
			case *runnableFunction:
				if obj.name != "" {
					err := c.RegisterFunction(obj.name, obj.closure)
					if err != nil {
						return phperror{err}
					}
				}
			}
		}
	}

	return r
}
