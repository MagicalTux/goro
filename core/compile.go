package core

import (
	"errors"
	"fmt"
	"io"
	"log"

	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

type compileCtx struct {
	t *tokenizer.Lexer
}

func compile(t *tokenizer.Lexer) runnable {
	c := &compileCtx{
		t: t,
	}
	_ = c // XXX

	var res runnables

	for {
		i, err := t.NextItem()
		if err != nil {
			if err == io.EOF {
				break
			}
			return phperror{err}
		}

		switch i.Type {
		case tokenizer.T_INLINE_HTML:
			// passthru
			res = append(res, runInlineHtml(i.Data))
		default:
			return phperror{fmt.Errorf("unexpected token %s", i.Type)}
		}

		log.Printf("%d: %s %q", i.Line, i.Type, i.Data)
	}

	return phperror{errors.New("todo")}
}
