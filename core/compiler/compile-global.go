package compiler

import (
	"fmt"
	"io"

	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/tokenizer"
)

type globalVar struct {
	static  phpv.ZString  // for static variable names like $foo
	dynamic phpv.Runnable // for variable-variables like $$foo
}

type runGlobal struct {
	vars []globalVar
	l    *phpv.Loc
}

func (g *runGlobal) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	if err := ctx.Tick(ctx, g.l); err != nil {
		return nil, err
	}
	for _, gv := range g.vars {
		var k phpv.ZString
		if gv.dynamic != nil {
			z, err := gv.dynamic.Run(ctx)
			if err != nil {
				return nil, err
			}
			k = phpv.ZString(z.String())
		} else {
			k = gv.static
		}
		if err := EvalGlobalBinding(ctx, k); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// EvalGlobalBinding implements one `global $name` binding: link the
// local FuncContext entry for $name to the corresponding global slot,
// auto-creating it as NULL if the global hasn't been set yet. Shared
// between AST and VM (OP_GLOBAL_BIND).
func EvalGlobalBinding(ctx phpv.Context, name phpv.ZString) error {
	glob := ctx.Global()
	var v *phpv.ZVal
	if ok, _ := glob.OffsetExists(ctx, name); !ok {
		v = phpv.ZNull{}.ZVal()
		if err := glob.OffsetSet(ctx, name, v); err != nil {
			return err
		}
	} else {
		var err error
		v, err = glob.OffsetGet(ctx, name)
		if err != nil {
			return err
		}
	}
	return ctx.OffsetSet(ctx, name, v)
}

func (g *runGlobal) Dump(w io.Writer) error {
	// PHP's AST dump emits each variable as a separate "global $x" statement.
	// We emit "global $a;\nglobal $b" so that DumpStatements appends the final ";\n".
	for i, gv := range g.vars {
		if i > 0 {
			// Separate previous statement with ";\n" (indentWriter will add indentation)
			if _, err := w.Write([]byte(";\nglobal ")); err != nil {
				return err
			}
		} else {
			if _, err := w.Write([]byte("global ")); err != nil {
				return err
			}
		}
		var err error
		if gv.dynamic != nil {
			// Variable-variable: global $$foo or global ${expr}
			// Dump as "$" + inner expression (runVariableRef handles $$foo as ${$foo},
			// but we want $$foo for simple cases, and ${expr} for complex ones).
			if rv, ok := gv.dynamic.(*runVariableRef); ok {
				// Check if inner is a simple variable ($$b → "$$b")
				if innerVar, ok2 := rv.v.(*runVariable); ok2 {
					_, err = fmt.Fprintf(w, "$$%s", innerVar.VarName())
				} else {
					// Complex inner expr: use $${expr} format... actually use ${expr}
					if _, err = w.Write([]byte("$${")); err != nil {
						return err
					}
					if err = rv.v.Dump(w); err != nil {
						return err
					}
					_, err = w.Write([]byte{'}'})
				}
			} else {
				// Fallback: use $ + dynamic dump
				if _, err = w.Write([]byte{'$'}); err != nil {
					return err
				}
				err = gv.dynamic.Dump(w)
			}
		} else {
			if _, err = w.Write([]byte{'$'}); err != nil {
				return err
			}
			_, err = w.Write([]byte(gv.static))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func compileGlobal(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// global $var, $var, $var, ...
	var err error

	// TODO check we are in a function/etc?

	g := &runGlobal{l: i.Loc()}

	// parse passed arguments
	for {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.Type == tokenizer.T_VARIABLE {
			varName := phpv.ZString(i.Data[1:])
			if varName == "this" {
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("Cannot use $this as global variable"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  i.Loc(),
				}
			}
			g.vars = append(g.vars, globalVar{static: varName})
		} else if i.IsSingle('$') {
			// variable-variable: global $$k or global ${expr}
			expr, err := compileRunVariableRef(nil, c, i.Loc())
			if err != nil {
				return nil, err
			}
			g.vars = append(g.vars, globalVar{dynamic: expr})
		} else {
			return nil, i.Unexpected()
		}

		i, err = c.NextItem()

		if i.IsSingle(',') {
			continue
		}

		if i.IsSingle(';') {
			c.backup()
			return g, nil
		}

		return nil, i.Unexpected()
	}
}
