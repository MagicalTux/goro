package core

import (
	"errors"
	"io"
	"log"

	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

func compile(t *tokenizer.Lexer) runnable {
	// test
	for {
		i, err := t.NextItem()
		if err != nil {
			if err == io.EOF {
				break
			}
			return phperror{err}
		}

		log.Printf("%d: %s %q", i.Line, i.Type, i.Data)
	}

	return phperror{errors.New("todo")}
}
