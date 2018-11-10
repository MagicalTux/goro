package core

import (
	"errors"

	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

type ZClassAttr int

const (
	// would use 1 << iota but those values come from php, so making them constants is more appropriate
	ZClassStatic           ZClassAttr = 0x001
	ZClassAbstract                    = 0x002
	ZClassImplAbstract                = 0x008 // an abstract method which has been implemented
	ZClassImplicitAbstract            = 0x010 // for classes
	ZClassExplicitAbstract            = 0x020 // for classes
	ZClassFinal                       = 0x040 // class attribute (not method)

	ZAttrStatic         = ZClassStatic
	ZAttrAbstract       = ZClassAbstract
	ZAttrFinal          = 0x004 // final method, not the same value as ZClassFinal
	ZAttrPublic         = 0x100
	ZAttrProtected      = 0x200
	ZAttrPrivate        = 0x400
	ZAttrAccess         = ZAttrPublic | ZAttrProtected | ZAttrPrivate
	ZAttrImplicitPublic = 0x1000 // method without flag
	ZAttrCtor           = 0x2000
	ZAttrDtor           = 0x4000
	ZAttrUserArgInfo    = 0x80    // method flag used by Closure::__invoke()
	ZAttrAllowStatic    = 0x10000 // method flag (bc only), any method that has this flag can be used statically and non statically.
	ZAttrShadow         = 0x20000 // shadow of parent's private method/property
	ZAttrDeprecated     = 0x40000 // deprecation flag
	ZAttrClosure        = 0x100000
	ZAttrFakeClosure    = 0x40
	ZAttrGenerator      = 0x800000
	ZAttrViaTrampoline  = 0x200000           // call through user function trampoline. e.g. __call, __callstatic
	ZAttrViaHandler     = ZAttrViaTrampoline // call through internal function handler. e.g. Closure::invoke()
	ZAttrVariadic       = 0x1000000
	ZAttrReturnRef      = 0x4000000
	ZAttrUseGuard       = 0x1000000  // class has magic methods __get/__set/__unset/__isset that use guards
	ZAttrHasTypeHints   = 0x10000000 // function has typed arguments
	ZAttrHasReturnType  = 0x40000000 // Function has a return type (or class has such non-private function)
)

type ZClassProp struct {
	VarName ZString
	Default *ZVal
}

type ZClass struct {
	Name ZString
	l    *Loc
	attr ZClassAttr

	Extends     ZString
	Implements  []ZString
	Const       map[ZString]ZString
	Props       []*ZClassProp
	StaticProps *ZHashTable
}

func compileClass(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	var attr ZClassAttr
	err := attr.parseClass(c)
	if err != nil {
		return nil, err
	}

	class := &ZClass{
		l:           MakeLoc(i.Loc()),
		StaticProps: NewHashTable(),
		attr:        attr,
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

		attr = 0
		attr.parseMethod(c)

		switch i.Type {
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
			// TODO
			_ = f
		}
		return nil, i.Unexpected()
	}

	return nil, errors.New("class todo")
}

func (a *ZClassAttr) parseClass(c *compileCtx) error {
	// parse class attributes (abstract or final)
	for {
		i, err := c.NextItem()
		if err != nil {
			return err
		}

		switch i.Type {
		case tokenizer.T_ABSTRACT:
			if *a&ZClassAbstract != 0 {
				return errors.New("Multiple abstract modifiers are not allowed")
			}
			*a |= ZClassAbstract | ZClassExplicitAbstract
		case tokenizer.T_FINAL:
			if *a&ZClassFinal != 0 {
				return errors.New("Multiple final modifiers are not allowed")
			}
			*a |= ZClassFinal
		default:
			c.backup()
			return nil
		}
	}
}

func (a *ZClassAttr) parseMethod(c *compileCtx) error {
	// parse method attributes (public/protected/private, abstract or final)
	for {
		i, err := c.NextItem()
		if err != nil {
			return err
		}

		switch i.Type {
		case tokenizer.T_STATIC:
			if *a&ZAttrStatic != 0 {
				return errors.New("Multiple static modifiers are not allowed")
			}
			*a |= ZAttrStatic
		case tokenizer.T_ABSTRACT:
			if *a&ZAttrAbstract != 0 {
				return errors.New("Multiple abstract modifiers are not allowed")
			}
			*a |= ZAttrAbstract
		case tokenizer.T_FINAL:
			if *a&ZAttrFinal != 0 {
				return errors.New("Multiple final modifiers are not allowed")
			}
			*a |= ZAttrFinal
		case tokenizer.T_PUBLIC:
			if *a&ZAttrAccess != 0 {
				return errors.New("Multiple access type modifiers are not allowed")
			}
			*a |= ZAttrPublic
		case tokenizer.T_PROTECTED:
			if *a&ZAttrAccess != 0 {
				return errors.New("Multiple access type modifiers are not allowed")
			}
			*a |= ZAttrProtected
		case tokenizer.T_PRIVATE:
			if *a&ZAttrAccess != 0 {
				return errors.New("Multiple access type modifiers are not allowed")
			}
			*a |= ZAttrPrivate
		default:
			c.backup()
			return nil
		}
	}
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
