package core

import "github.com/MagicalTux/gophp/core/tokenizer"

type ZClassProp struct {
	VarName   ZString
	Default   Runnable
	Modifiers ZObjectAttr
}

type ZClassMethod struct {
	Name      ZString
	Modifiers ZObjectAttr
	Method    Callable
}

type ZClass struct {
	Name ZString
	l    *Loc
	attr ZClassAttr

	Extends     ZString
	Implements  []ZString
	Const       map[ZString]ZString
	Props       []*ZClassProp
	Methods     map[ZString]*ZClassMethod
	StaticProps *ZHashTable
}

func compileClass(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	var attr ZClassAttr
	err := attr.parse(c)
	if err != nil {
		return nil, err
	}

	class := &ZClass{
		l:           MakeLoc(i.Loc()),
		StaticProps: NewHashTable(),
		attr:        attr,
		Methods:     make(map[ZString]*ZClassMethod),
	}

	err = class.parseClassLine(c)
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
		l := MakeLoc(i.Loc())
		c.backup()

		// parse attrs if any
		var attr ZObjectAttr
		attr.parse(c)

		// read whatever comes next
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		switch i.Type {
		case tokenizer.T_VAR:
			// class variable, with possible default value
			prop := &ZClassProp{Modifiers: attr}
			i, err := c.NextItem()
			if err != nil {
				return nil, err
			}
			if i.Type != tokenizer.T_VARIABLE {
				return nil, i.Unexpected()
			}
			prop.VarName = ZString(i.Data[1:])

			// check for default value
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}

			if i.IsSingle('=') {
				// parse default value for class variable
				prop.Default, err = compileExpr(nil, c)
				if err != nil {
					return nil, err
				}

				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
			}

			if !i.IsSingle(';') {
				return nil, i.Unexpected()
			}

			class.Props = append(class.Props, prop)
		case tokenizer.T_FUNCTION:
			// next must be a string (method name)
			i, err := c.NextItem()
			if err != nil {
				return nil, err
			}

			if i.Type != tokenizer.T_STRING {
				return nil, i.Unexpected()
			}

			f, err := compileFunctionWithName(ZString(i.Data), c, l)

			// register method
			method := &ZClassMethod{Name: ZString(i.Data), Modifiers: attr, Method: f}
			class.Methods[method.Name.ToLower()] = method
		default:
			return nil, i.Unexpected()
		}
	}

	return class, nil
}

func (class *ZClass) parseClassLine(c *compileCtx) error {
	i, err := c.NextItem()
	if err != nil {
		return err
	}

	if i.Type != tokenizer.T_STRING {
		return i.Unexpected()
	}

	class.Name = ZString(i.Data)

	i, err = c.NextItem()
	if err != nil {
		return err
	}

	if i.Type == tokenizer.T_EXTENDS {
		// can only extend one class
		class.Extends, err = compileReadClassIdentifier(c)
		if err != nil {
			return err
		}

		i, err = c.NextItem()
		if err != nil {
			return err
		}
	}
	if i.Type == tokenizer.T_IMPLEMENTS {
		// can implement many classes
		for {
			impl, err := compileReadClassIdentifier(c)
			if err != nil {
				return err
			}

			class.Implements = append(class.Implements, impl)

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

func compileReadClassIdentifier(c *compileCtx) (ZString, error) {
	var res ZString

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
			res += ZString(i.Data)
			continue
		}
		if i.Type == tokenizer.T_STRING {
			res += ZString(i.Data)
			continue
		}

		c.backup()
		return res, nil
	}
}

func (c *ZClass) Run(ctx Context) (*ZVal, error) {
	return nil, ctx.GetGlobal().RegisterClass(c.Name, c)
}

func (c *ZClass) Loc() *Loc {
	return c.l
}
