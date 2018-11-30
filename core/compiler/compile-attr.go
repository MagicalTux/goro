package compiler

import (
	"errors"

	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

func parseZClassAttr(a *phpv.ZClassAttr, c compileCtx) error {
	// parse class attributes (abstract or final)
	for {
		i, err := c.NextItem()
		if err != nil {
			return err
		}

		switch i.Type {
		case tokenizer.T_ABSTRACT:
			if *a&phpv.ZClassAbstract != 0 {
				return errors.New("Multiple abstract modifiers are not allowed")
			}
			*a |= phpv.ZClassAbstract | phpv.ZClassExplicitAbstract
		case tokenizer.T_FINAL:
			if *a&phpv.ZClassFinal != 0 {
				return errors.New("Multiple final modifiers are not allowed")
			}
			*a |= phpv.ZClassFinal
		default:
			c.backup()
			return nil
		}
	}
}

func parseZObjectAttr(a *phpv.ZObjectAttr, c compileCtx) error {
	// parse method attributes (public/protected/private, abstract or final)
	for {
		i, err := c.NextItem()
		if err != nil {
			return err
		}

		switch i.Type {
		case tokenizer.T_STATIC:
			if *a&phpv.ZAttrStatic != 0 {
				return errors.New("Multiple static modifiers are not allowed")
			}
			*a |= phpv.ZAttrStatic
		case tokenizer.T_ABSTRACT:
			if *a&phpv.ZAttrAbstract != 0 {
				return errors.New("Multiple abstract modifiers are not allowed")
			}
			*a |= phpv.ZAttrAbstract
		case tokenizer.T_FINAL:
			if *a&phpv.ZAttrFinal != 0 {
				return errors.New("Multiple final modifiers are not allowed")
			}
			*a |= phpv.ZAttrFinal
		case tokenizer.T_PUBLIC:
			if *a&phpv.ZAttrAccess != 0 {
				return errors.New("Multiple access type modifiers are not allowed")
			}
			*a |= phpv.ZAttrPublic
		case tokenizer.T_PROTECTED:
			if *a&phpv.ZAttrAccess != 0 {
				return errors.New("Multiple access type modifiers are not allowed")
			}
			*a |= phpv.ZAttrProtected
		case tokenizer.T_PRIVATE:
			if *a&phpv.ZAttrAccess != 0 {
				return errors.New("Multiple access type modifiers are not allowed")
			}
			*a |= phpv.ZAttrPrivate
		default:
			c.backup()
			return nil
		}
	}
}
