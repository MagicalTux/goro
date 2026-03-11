package compiler

import (
	"slices"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type zclassCompileCtx struct {
	compileCtx
	class *phpobj.ZClass
}

func (z *zclassCompileCtx) getClass() *phpobj.ZClass {
	return z.class
}

func compileClass(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	var attr phpv.ZClassAttr
	var err error

	// If called from a modifier token (abstract/final), back it up so
	// parseZClassAttr can consume it, then read the actual class token.
	if i.Type == tokenizer.T_ABSTRACT || i.Type == tokenizer.T_FINAL {
		c.backup()
		err = parseZClassAttr(&attr, c)
		if err != nil {
			return nil, err
		}
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	} else {
		err = parseZClassAttr(&attr, c)
		if err != nil {
			return nil, err
		}
	}

	class := &phpobj.ZClass{
		L:       i.Loc(),
		Attr:    attr,
		Methods: make(map[phpv.ZString]*phpv.ZClassMethod),
		Const:   make(map[phpv.ZString]phpv.Val),
		H:       &phpv.ZClassHandlers{},
	}

	switch i.Type {
	case tokenizer.T_CLASS:
	case tokenizer.T_INTERFACE:
		class.Type = phpv.ZClassTypeInterface
	default:
		return nil, i.Unexpected()
	}

	c = &zclassCompileCtx{c, class}

	err = parseClassLine(class, c)
	if err != nil {
		return nil, err
	}

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	if !i.IsSingle('{') {
		return nil, i.Unexpected()
	}

	for {
		// we just read this item to grab location and check for '}'
		i, err := c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle('}') {
			// end of class
			break
		}
		l := i.Loc()
		c.backup()

		// parse attrs if any
		var attr phpv.ZObjectAttr
		parseZObjectAttr(&attr, c)

		// read whatever comes next
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		switch i.Type {
		case tokenizer.T_VAR:
			// class variable, with possible default value
			i, err := c.NextItem()
			if err != nil {
				return nil, err
			}
			if i.Type != tokenizer.T_VARIABLE {
				return nil, i.Unexpected()
			}
			fallthrough
		case tokenizer.T_VARIABLE:
			for {
				prop := &phpv.ZClassProp{Modifiers: attr}
				prop.VarName = phpv.ZString(i.Data[1:])

				// check for default value
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}

				if i.IsSingle('=') {
					r, err := compileExpr(nil, c)
					if err != nil {
						return nil, err
					}
					// parse default value for class variable
					prop.Default = &phpv.CompileDelayed{r}

					i, err = c.NextItem()
					if err != nil {
						return nil, err
					}
				}

				class.Props = append(class.Props, prop)
				if i.IsSingle(';') {
					break
				}
				if i.IsSingle(',') {
					i, err = c.NextItem()
					if err != nil {
						return nil, err
					}

					if i.Type != tokenizer.T_VARIABLE {
						return nil, i.Unexpected()
					}
					continue
				}

				return nil, i.Unexpected()
			}
			// sort props, show public first, then protected, then private
			slices.SortStableFunc(class.Props, func(a, b *phpv.ZClassProp) int {
				visibility := phpv.ZAttrPublic | phpv.ZAttrProtected | phpv.ZAttrPrivate
				attrA := a.Modifiers & (phpv.ZObjectAttr(visibility))
				attrB := b.Modifiers & (phpv.ZObjectAttr(visibility))
				return int(attrA - attrB)
			})
		case tokenizer.T_CONST:
			// const K = V [, K2 = V2 ...];
			for {
				// get const name
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
				if i.Type != tokenizer.T_STRING {
					return nil, i.Unexpected()
				}
				constName := i.Data

				// =
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
				if !i.IsSingle('=') {
					return nil, i.Unexpected()
				}

				var v phpv.Runnable
				v, err = compileExpr(nil, c)
				if err != nil {
					return nil, err
				}

				class.Const[phpv.ZString(constName)] = &phpv.CompileDelayed{v}

				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
				if i.IsSingle(';') {
					break
				}
				if !i.IsSingle(',') {
					return nil, i.Unexpected()
				}
			}
		case tokenizer.T_FUNCTION:
			// next must be a string (method name)
			i, err := c.NextItem()
			if err != nil {
				return nil, err
			}

			rref := false
			if i.IsSingle('&') {
				rref = true
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
			}

			if i.Type != tokenizer.T_STRING {
				return nil, i.Unexpected()
			}

			var f phpv.Callable

			optionalBody := class.Type == phpv.ZClassTypeInterface || attr&phpv.ZAttrAbstract != 0
			f, err = compileFunctionWithName(phpv.ZString(i.Data), c, l, rref, optionalBody)
			if err != nil {
				return nil, err
			}
			f.(*ZClosure).class = class

			// an interface method with a body is not a parse error,
			// so delay returning an error when code is ran
			_, emptyBody := f.(*ZClosure).code.(phpv.RunNull)

			// register method
			method := &phpv.ZClassMethod{
				Name:      phpv.ZString(i.Data),
				Modifiers: attr,
				Method:    f,
				Class:     class,
				Empty:     emptyBody,
			}

			if x := method.Name.ToLower(); x == class.BaseName().ToLower() || x == "__construct" {
				//if class.Constructor != nil {
				class.Handlers().Constructor = method
			}
			class.Methods[method.Name.ToLower()] = method
		default:
			return nil, i.Unexpected()
		}
	}

	return class, nil
}

func parseClassLine(class *phpobj.ZClass, c compileCtx) error {
	i, err := c.NextItem()
	if err != nil {
		return err
	}

	if i.Type != tokenizer.T_STRING {
		return i.Unexpected()
	}

	class.Name = phpv.ZString(i.Data)

	i, err = c.NextItem()
	if err != nil {
		return err
	}

	if i.Type == tokenizer.T_EXTENDS {
		// For interfaces, extends can have multiple comma-separated parents
		class.ExtendsStr, err = compileReadClassIdentifier(c)
		if err != nil {
			return err
		}

		i, err = c.NextItem()
		if err != nil {
			return err
		}

		// Check for comma-separated extends (interfaces only)
		if i.IsSingle(',') && class.Type == phpv.ZClassTypeInterface {
			// first extends goes to ImplementsStr for interfaces
			class.ImplementsStr = append(class.ImplementsStr, class.ExtendsStr)
			for {
				impl, err := compileReadClassIdentifier(c)
				if err != nil {
					return err
				}
				class.ImplementsStr = append(class.ImplementsStr, impl)
				i, err = c.NextItem()
				if err != nil {
					return err
				}
				if !i.IsSingle(',') {
					break
				}
			}
			// Move first extends back
			class.ExtendsStr = class.ImplementsStr[0]
			class.ImplementsStr = class.ImplementsStr[1:]
		}
	}
	if i.Type == tokenizer.T_IMPLEMENTS {
		// can implement many classes
		for {
			impl, err := compileReadClassIdentifier(c)
			if err != nil {
				return err
			}

			if impl != "" {
				class.ImplementsStr = append(class.ImplementsStr, impl)
			}

			// read next
			i, err = c.NextItem()
			if err != nil {
				return err
			}

			if i.IsSingle(',') {
				// there's more
				i, err = c.NextItem()
				if err != nil {
					return err
				}

				continue
			}
			break
		}
	}

	c.backup()

	return nil
}

func compileReadClassIdentifier(c compileCtx) (phpv.ZString, error) {
	var res phpv.ZString

	for {
		i, err := c.NextItem()
		if err != nil {
			return res, err
		}

		// T_NS_SEPARATOR
		if i.Type == tokenizer.T_NS_SEPARATOR {
			if res != "" {
				res += "\\"
			}
			i, err := c.NextItem()
			if err != nil {
				return res, err
			}
			if i.Type != tokenizer.T_STRING {
				return res, i.Unexpected()
			}
			res += phpv.ZString(i.Data)
			continue
		}
		if i.Type == tokenizer.T_STRING {
			res += phpv.ZString(i.Data)
			continue
		}

		c.backup()
		return res, nil
	}
}
