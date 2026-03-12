package compiler

import (
	"fmt"

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
		Const:   make(map[phpv.ZString]*phpv.ZClassConst),
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

	// Set the compiling class so that deprecation messages can include the class name
	c.Global().SetCompilingClass(class)
	defer c.Global().SetCompilingClass(nil)

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

		// Skip doc comments
		for i.Type == tokenizer.T_DOC_COMMENT || i.Type == tokenizer.T_COMMENT {
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
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
		case tokenizer.T_CONST:
			// Check for invalid modifiers on constants
			if attr.IsStatic() {
				phpErr := &phpv.PhpError{
					Err:  fmt.Errorf("Cannot use the static modifier on a class constant"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  i.Loc(),
				}
				c.Global().LogError(phpErr)
				return nil, phpv.ExitError(255)
			}
			if attr.Has(phpv.ZAttrAbstract) {
				phpErr := &phpv.PhpError{
					Err:  fmt.Errorf("Cannot use the abstract modifier on a class constant"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  i.Loc(),
				}
				c.Global().LogError(phpErr)
				return nil, phpv.ExitError(255)
			}
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

				// Check for duplicate constant
				if _, exists := class.Const[phpv.ZString(constName)]; exists {
					phpErr := &phpv.PhpError{
						Err:  fmt.Errorf("Cannot redefine class constant %s::%s", class.Name, constName),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  i.Loc(),
					}
					c.Global().LogError(phpErr)
					return nil, phpv.ExitError(255)
				}

				// Interface constants must be public
				if class.Type == phpv.ZClassTypeInterface && (attr.IsPrivate() || attr.IsProtected()) {
					phpErr := &phpv.PhpError{
						Err:  fmt.Errorf("Access type for interface constant %s::%s must be public", class.Name, constName),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  i.Loc(),
					}
					c.Global().LogError(phpErr)
					return nil, phpv.ExitError(255)
				}

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

				class.Const[phpv.ZString(constName)] = &phpv.ZClassConst{
					Value:     &phpv.CompileDelayed{v},
					Modifiers: attr,
				}

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
				Loc:       l,
			}

			if x := method.Name.ToLower(); x == class.BaseName().ToLower() || x == "__construct" {
				//if class.Constructor != nil {
				class.Handlers().Constructor = method

				// Handle constructor property promotion (PHP 8.0+)
				if fga, ok := f.(phpv.FuncGetArgs); ok {
					for _, arg := range fga.GetArgs() {
						if arg.Promotion != 0 {
							prop := &phpv.ZClassProp{
								VarName:   arg.VarName,
								Modifiers: arg.Promotion,
							}
							class.Props = append(class.Props, prop)
						}
					}
				}
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
