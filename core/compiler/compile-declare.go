package compiler

import (
	"io"
	"strings"

	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

// compileDeclare handles:
//
//	declare(strict_types=1);
//	declare(ticks=N);
//	declare(encoding='...');
//	declare(...) { ... }
//	declare(...): ... enddeclare;
func compileDeclare(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	l := i.Loc()

	// Expect '('
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}
	if !i.IsSingle('(') {
		return nil, i.Unexpected()
	}

	// Parse directive(s)
	var directives []declareDirective

	for {
		// Read directive name
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.Type != tokenizer.T_STRING {
			return nil, i.Unexpected()
		}
		name := strings.ToLower(i.Data)

		// Expect '='
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if !i.IsSingle('=') {
			return nil, i.Unexpected()
		}

		// Read value
		val, err := compileExpr(nil, c)
		if err != nil {
			return nil, err
		}

		directives = append(directives, declareDirective{name: name, val: val})

		// Check for ',' or ')'
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.IsSingle(')') {
			break
		}
		if !i.IsSingle(',') {
			return nil, i.Unexpected()
		}
	}

	// Check what follows: ';', '{', or ':'
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.IsSingle(';') {
		// Statement form: declare(strict_types=1);
		// For ticks, we need a runnable; for strict_types/encoding, nothing to run
		return buildDeclareRunnable(directives, nil, l), nil
	}

	if i.IsSingle('{') {
		// Block form: declare(...) { ... }
		c.backup()
		body, err := compileBaseSingle(nil, c)
		if err != nil {
			return nil, err
		}
		return buildDeclareRunnable(directives, body, l), nil
	}

	if i.IsSingle(':') {
		// Alternative syntax: declare(...): ... enddeclare;
		c.backup()
		body, err := compileDeclareAltBlock(c)
		if err != nil {
			return nil, err
		}
		return buildDeclareRunnable(directives, body, l), nil
	}

	return nil, i.Unexpected()
}

// compileDeclareAltBlock handles: : ... enddeclare;
func compileDeclareAltBlock(c compileCtx) (phpv.Runnable, error) {
	// consume the ':'
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}
	if !i.IsSingle(':') {
		return nil, i.Unexpected()
	}

	var res phpv.Runnables

	for {
		i, err = c.NextItem()
		if err != nil {
			return res, err
		}
		if i.Type == tokenizer.T_ENDDECLARE {
			// Expect ';' after enddeclare
			i, err = c.NextItem()
			if err != nil {
				return res, err
			}
			if !i.IsExpressionEnd() {
				return nil, i.Unexpected()
			}
			return res, nil
		}

		t, err := compileBaseSingle(i, c)
		if t != nil {
			res = append(res, t)
		}
		if err != nil {
			return res, err
		}
	}
}

type declareDirective struct {
	name string
	val  phpv.Runnable
}

func buildDeclareRunnable(directives []declareDirective, body phpv.Runnable, l *phpv.Loc) phpv.Runnable {
	// For strict_types and encoding, there's nothing to do at runtime
	// (strict_types is handled at compile time in type checking).
	// For ticks, we'd need to set up tick handlers.
	// For now, if there's a body, return it; otherwise return nil.
	hasTicks := false
	for _, d := range directives {
		if d.name == "ticks" {
			hasTicks = true
		}
	}

	if body == nil && !hasTicks {
		return nil
	}

	if body != nil {
		return body
	}

	// ticks without body - nothing to run
	return nil
}

// runnableDeclare wraps a body with declare directives
type runnableDeclare struct {
	body phpv.Runnable
	l    *phpv.Loc
}

func (r *runnableDeclare) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	if r.body == nil {
		return nil, nil
	}
	return r.body.Run(ctx)
}

func (r *runnableDeclare) Dump(w io.Writer) error {
	_, err := w.Write([]byte("declare(...)"))
	if err != nil {
		return err
	}
	if r.body != nil {
		_, err = w.Write([]byte(" { "))
		if err != nil {
			return err
		}
		err = r.body.Dump(w)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(" }"))
	}
	return err
}


