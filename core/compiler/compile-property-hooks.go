package compiler

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

// compilePropertyHooks parses the { get { ... } set($value) { ... } } block
// after a property declaration (PHP 8.4 property hooks).
func compilePropertyHooks(prop *phpv.ZClassProp, class *phpobj.ZClass, c compileCtx) error {
	prop.HasHooks = true
	for {
		i, err := c.NextItem()
		if err != nil {
			return err
		}

		if i.IsSingle('}') {
			return nil
		}

		// Skip optional modifiers before hook name
		for i.Type == tokenizer.T_FINAL || i.Type == tokenizer.T_ABSTRACT {
			i, err = c.NextItem()
			if err != nil {
				return err
			}
		}

		if i.Type != tokenizer.T_STRING {
			return i.Unexpected()
		}

		switch i.Data {
		case "get":
			i, err = c.NextItem()
			if err != nil {
				return err
			}
			if i.IsSingle(';') {
				continue // abstract get
			}
			if i.IsSingle('{') {
				body, err := compileBase(i, c)
				if err != nil {
					return err
				}
				prop.GetHook = body
			} else if i.Type == tokenizer.T_DOUBLE_ARROW {
				expr, err := compileExpr(nil, c)
				if err != nil {
					return err
				}
				prop.GetHook = &runReturn{v: expr, l: i.Loc()}
				i, err = c.NextItem()
				if err != nil {
					return err
				}
				if !i.IsSingle(';') {
					return i.Unexpected()
				}
			} else {
				return i.Unexpected()
			}

		case "set":
			prop.SetParam = "value"
			i, err = c.NextItem()
			if err != nil {
				return err
			}
			if i.IsSingle('(') {
				// Parse set parameter - skip type hints, get variable name
				for {
					i, err = c.NextItem()
					if err != nil {
						return err
					}
					if i.Type == tokenizer.T_VARIABLE {
						prop.SetParam = phpv.ZString(i.Data[1:])
						i, err = c.NextItem()
						if err != nil {
							return err
						}
						break
					}
					if i.IsSingle(')') {
						break
					}
				}
				if !i.IsSingle(')') {
					return i.Unexpected()
				}
				i, err = c.NextItem()
				if err != nil {
					return err
				}
			}
			if i.IsSingle(';') {
				continue // abstract set
			}
			if i.IsSingle('{') {
				body, err := compileBase(i, c)
				if err != nil {
					return err
				}
				prop.SetHook = body
			} else if i.Type == tokenizer.T_DOUBLE_ARROW {
				expr, err := compileExpr(nil, c)
				if err != nil {
					return err
				}
				prop.SetHook = expr
				i, err = c.NextItem()
				if err != nil {
					return err
				}
				if !i.IsSingle(';') {
					return i.Unexpected()
				}
			} else {
				return i.Unexpected()
			}

		default:
			return &phpv.PhpError{
				Err:  fmt.Errorf("Unknown property hook '%s'", i.Data),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
		}
	}
}
