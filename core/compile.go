package core

import (
	"errors"
	"io"
	"log"

	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

type compileCtx struct {
	t *tokenizer.Lexer
}

var itemTypeHandler = map[tokenizer.ItemType]func(i *tokenizer.Item, c *compileCtx) (runnable, error){
	tokenizer.T_OPEN_TAG:    compileIgnore,
	tokenizer.T_INLINE_HTML: compileInlineHtml,
}

func compileIgnore(i *tokenizer.Item, c *compileCtx) (runnable, error) {
	return nil, nil
}

func (c *compileCtx) NextItem() (*tokenizer.Item, error) {
	for {
		i, err := c.t.NextItem()

		if err != nil {
			return i, err
		}

		switch i.Type {
		case tokenizer.T_WHITESPACE:
		case tokenizer.T_COMMENT:
		default:
			return i, err
		}
	}
}

func compile(t *tokenizer.Lexer) runnable {
	c := &compileCtx{
		t: t,
	}

	var res runnables

	for {
		i, err := c.NextItem()
		if err != nil {
			if err == io.EOF {
				break
			}
			return phperror{err}
		}

		h, ok := itemTypeHandler[i.Type]
		if !ok {
			log.Printf("Unsupported: %d: %s %q", i.Line, i.Type, i.Data)
			continue
		}

		r, err := h(i, c)
		if err != nil {
			return phperror{err}
		}

		res = append(res, r)
	}

	return phperror{errors.New("todo")}
}
