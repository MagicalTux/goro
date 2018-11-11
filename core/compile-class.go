package core

import (
	"errors"

	"git.atonline.com/tristantech/gophp/core/tokenizer"
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
	err := attr.parse(c)
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
			i, err := c.NextItem()
			if err != nil {
				return nil, err
			}
			if i.Type != tokenizer.T_VARIABLE {
				return nil, i.Unexpected()
			}
			return nil, i.Unexpected()
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
