package core

import (
	"errors"

	"git.atonline.com/tristantech/gophp/core/tokenizer"
)

type ZClass struct {
	Name       ZString
	Extends    ZString
	Implements []ZString
	l          *Loc
}

func compileClass(i *tokenizer.Item, c *compileCtx) (Runnable, error) {
	class := &ZClass{l: MakeLoc(i.Loc())}

	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.Type != tokenizer.T_STRING {
		return nil, i.Unexpected()
	}

	class.Name = ZString(i.Data)

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	if i.Type == tokenizer.T_EXTENDS {
		// can only extend one class
		class.Extends, err = compileReadClassIdentifier(c)
		if err != nil {
			return nil, err
		}

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}
	if i.Type == tokenizer.T_IMPLEMENTS {
		// can implement many classes
		for {
			impl, err := compileReadClassIdentifier(c)
			if err != nil {
				return nil, err
			}

			class.Implements = append(class.Implements, impl)

			// read next
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}

			if i.IsSingle(',') {
				// there's more
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}

				continue
			}
			break
		}
	}

	if !i.IsSingle('{') {
		return nil, i.Unexpected()
	}

	return nil, errors.New("class todo")
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
