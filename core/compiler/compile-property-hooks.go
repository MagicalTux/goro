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

		// Parse optional attributes before hook name
		var hookAttrs []*phpv.ZAttribute
		for i.Type == tokenizer.T_ATTRIBUTE {
			// Consume the attribute and its arguments
			parsed, err := parseAttributes(c)
			if err != nil {
				return err
			}
			hookAttrs = append(hookAttrs, parsed...)
			i, err = c.NextItem()
			if err != nil {
				return err
			}
		}

		// Check for #[\NoDiscard] on property hooks (not allowed)
		for _, attr := range hookAttrs {
			if attr.ClassName == "NoDiscard" || attr.ClassName == "\\NoDiscard" {
				// Check if DelayedTargetValidation is also present
				hasDelayed := false
				for _, a := range hookAttrs {
					if a.ClassName == "DelayedTargetValidation" || a.ClassName == "\\DelayedTargetValidation" {
						hasDelayed = true
						break
					}
				}
				if !hasDelayed {
					return &phpv.PhpError{
						Err:  fmt.Errorf("#[\\NoDiscard] is not supported for property hooks"),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  i.Loc(),
					}
				}
			}
		}

		// Skip optional modifiers before hook name
		for i.Type == tokenizer.T_FINAL || i.Type == tokenizer.T_ABSTRACT {
			i, err = c.NextItem()
			if err != nil {
				return err
			}
		}

		// Skip optional '&' for by-ref hooks
		if i.IsSingle('&') {
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
			// Wrap compilation in a function context so __FUNCTION__ resolves to $prop::get
			hookClosure := &ZClosure{name: phpv.ZString(fmt.Sprintf("$%s::get", prop.VarName))}
			hookCtx := &zclosureCompileCtx{c, hookClosure}
			if i.IsSingle('{') {
				body, err := compileBase(i, hookCtx)
				if err != nil {
					return err
				}
				prop.GetHook = body
			} else if i.Type == tokenizer.T_DOUBLE_ARROW {
				expr, err := compileExpr(nil, hookCtx)
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
			// Wrap compilation in a function context so __FUNCTION__ resolves to $prop::set
			setHookClosure := &ZClosure{name: phpv.ZString(fmt.Sprintf("$%s::set", prop.VarName))}
			setHookCtx := &zclosureCompileCtx{c, setHookClosure}
			if i.IsSingle('{') {
				body, err := compileBase(i, setHookCtx)
				if err != nil {
					return err
				}
				prop.SetHook = body
			} else if i.Type == tokenizer.T_DOUBLE_ARROW {
				expr, err := compileExpr(nil, setHookCtx)
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
