package compiler

import (
	"fmt"
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

		// Validate: ticks value must be a literal integer
		if name == "ticks" {
			if _, isLiteral := val.(*runZVal); !isLiteral {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("declare(ticks) value must be a literal"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  l,
				}
			}
		}

		// Validate: encoding directive
		if name == "encoding" {
			lit, isLiteral := val.(*runZVal)
			if !isLiteral {
				// Non-literal expression (e.g., M_PI, constant) -> fatal error
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("Encoding must be a literal"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  l,
				}
			}
			// Check if multibyte is enabled
			mbVal := c.GetConfig(phpv.ZString("zend.multibyte"), nil)
			mbEnabled := false
			if mbVal != nil {
				mbStr := string(mbVal.AsString(c))
				mbEnabled = mbStr == "1" || strings.EqualFold(mbStr, "on") || strings.EqualFold(mbStr, "true")
			}

			if !mbEnabled {
				// Emit warning about multibyte being off
				c.Warn("declare(encoding=...) ignored because Zend multibyte feature is turned off by settings")
			}

			// For non-string literal values, warn about unsupported encoding
			switch v := lit.v.(type) {
			case phpv.ZString:
				// String encoding value - only "utf-8" is supported (case-insensitive)
				if !strings.EqualFold(string(v), "utf-8") && !strings.EqualFold(string(v), "utf8") {
					c.Warn("Unsupported encoding [%s]", string(v))
				}
			case phpv.ZInt:
				c.Warn("Unsupported encoding [%d]", int64(v))
			case phpv.ZFloat:
				c.Warn("Unsupported encoding [%s]", phpv.FormatFloat(float64(v)))
			}
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

	// Set strict_types on the compile context so attribute constructors can check it
	for _, d := range directives {
		if d.name == "strict_types" {
			if lit, ok := d.val.(*runZVal); ok {
				if lit.v == phpv.ZInt(1) || lit.v == phpv.ZBool(true) {
					if rc, ok := c.(*compileRootCtx); ok {
						rc.strictTypes = true
					}
				}
			}
		}
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
	// For ticks, wrap the body in a ticking handler.
	var ticksVal int64
	hasTicks := false
	for _, d := range directives {
		if d.name == "ticks" {
			hasTicks = true
			if lit, ok := d.val.(*runZVal); ok {
				switch v := lit.v.(type) {
				case phpv.ZInt:
					ticksVal = int64(v)
				case phpv.ZFloat:
					ticksVal = int64(v)
				}
			}
		}
	}

	if hasTicks && body != nil && ticksVal > 0 {
		return &runnableDeclareTicks{body: body, ticks: ticksVal, l: l}
	}

	if body != nil {
		return body
	}

	return nil
}

// runnableDeclareTicks wraps a body and calls tick functions after every N statements
type runnableDeclareTicks struct {
	body  phpv.Runnable
	ticks int64
	l     *phpv.Loc
}

func (r *runnableDeclareTicks) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	if r.body == nil {
		return nil, nil
	}

	// If the body is a Runnables (list of statements), tick after each
	if stmts, ok := r.body.(phpv.Runnables); ok {
		var last *phpv.ZVal
		var count int64
		for _, stmt := range stmts {
			var err error
			last, err = stmt.Run(ctx)
			if err != nil {
				return last, err
			}
			count++
			if count%r.ticks == 0 {
				if err := ctx.Global().CallTickFunctions(ctx); err != nil {
					return nil, err
				}
			}
		}
		return last, nil
	}

	// Single statement body
	result, err := r.body.Run(ctx)
	if err != nil {
		return result, err
	}
	if err := ctx.Global().CallTickFunctions(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *runnableDeclareTicks) Dump(w io.Writer) error {
	_, err := w.Write([]byte("declare(ticks=...)"))
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


