package core

import (
	"errors"

	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

func compileBreak(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	// return this as a runtime element and not a compile time error so switch and loops will catch it
	return &PhpError{errors.New("'break' not in the 'loop' or 'switch' context"), MakeLoc(i.Loc()), PhpBreak}, nil
}

func compileContinue(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	// return this as a runtime element and not a compile time error so switch and loops will catch it
	return &PhpError{errors.New("'continue' not in the 'loop' context"), MakeLoc(i.Loc()), PhpContinue}, nil
}
