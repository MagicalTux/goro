package compiler

import (
	"errors"
	"fmt"

	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

// parseAttributes parses one or more #[...] attribute groups.
// Called when T_ATTRIBUTE has been seen. The T_ATTRIBUTE token represents "#[".
// Format: #[AttrName(args...), AttrName2(args...)]
// Multiple attribute groups are allowed: #[A] #[B]
func parseAttributes(c compileCtx) ([]*phpv.ZAttribute, error) {
	var attrs []*phpv.ZAttribute

	for {
		// We just consumed T_ATTRIBUTE (#[), now parse comma-separated attributes until ]
		for {
			// Parse attribute class name (possibly namespaced)
			i, err := c.NextItem()
			if err != nil {
				return nil, err
			}

			if i.IsSingle(']') {
				// Empty attribute group #[] - break out
				break
			}

			// Build fully qualified class name
			var className phpv.ZString

			// Handle leading namespace separator
			if i.Type == tokenizer.T_NS_SEPARATOR {
				className = "\\"
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
			}

			if i.Type != tokenizer.T_STRING {
				return nil, i.Unexpected()
			}

			className += phpv.ZString(i.Data)

			// Consume additional namespace parts
			for {
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
				if i.Type == tokenizer.T_NS_SEPARATOR {
					next, err := c.NextItem()
					if err != nil {
						return nil, err
					}
					if next.Type != tokenizer.T_STRING {
						return nil, next.Unexpected()
					}
					className += "\\" + phpv.ZString(next.Data)
				} else {
					break
				}
			}

			// Resolve class name through use map
			className = c.resolveClassName(className)

			attr := &phpv.ZAttribute{
				ClassName: className,
			}

			// Check for arguments: (
			if i.IsSingle('(') {
				// Parse arguments as constant expressions
				args, err := parseAttributeArgs(c)
				if err != nil {
					return nil, err
				}
				attr.Args = args

				// Read next token after closing )
				i, err = c.NextItem()
				if err != nil {
					return nil, err
				}
			}

			attrs = append(attrs, attr)

			// Check for comma (more attributes in same group) or ]
			if i.IsSingle(',') {
				continue
			}
			if i.IsSingle(']') {
				break
			}

			return nil, i.Unexpected()
		}

		// Check if there's another attribute group following: #[...]
		i, err := c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.Type == tokenizer.T_ATTRIBUTE {
			// Another attribute group
			continue
		}
		// Not another attribute group, back up
		c.backup()
		break
	}

	return attrs, nil
}

// parseAttributeArgs parses the arguments inside #[Attr(...)].
// Called after the opening '(' has been consumed.
// Returns the parsed argument values.
func parseAttributeArgs(c compileCtx) ([]*phpv.ZVal, error) {
	var args []*phpv.ZVal

	// Check for empty args: ()
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}
	if i.IsSingle(')') {
		return args, nil
	}
	c.backup()

	for {
		// Parse each argument as a constant expression
		expr, err := compileExpr(nil, c)
		if err != nil {
			return nil, err
		}

		// Try to evaluate the expression at compile time
		val, err := expr.Run(c)
		if err != nil {
			// If we can't evaluate at compile time, store as nil
			// This handles forward references etc.
			args = append(args, phpv.ZNULL.ZVal())
		} else {
			args = append(args, val)
		}

		i, err := c.NextItem()
		if err != nil {
			return nil, err
		}
		if i.IsSingle(')') {
			return args, nil
		}
		if i.IsSingle(',') {
			// Check for trailing comma before )
			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			if i.IsSingle(')') {
				return args, nil
			}
			c.backup()
			continue
		}

		return nil, i.Unexpected()
	}
}

func parseZClassAttr(a *phpv.ZClassAttr, c compileCtx) error {
	// parse class attributes (abstract or final)
	for {
		i, err := c.NextItem()
		if err != nil {
			return err
		}

		switch i.Type {
		case tokenizer.T_ATTRIBUTE:
			// Skip attributes in class attr context - they are handled separately
			// Parse and discard (the caller will handle them)
			c.backup()
			return nil
		case tokenizer.T_ABSTRACT:
			if *a&phpv.ZClassAbstract != 0 {
				return errors.New("Multiple abstract modifiers are not allowed")
			}
			if *a&phpv.ZClassFinal != 0 {
				return errors.New("Cannot use the final modifier on an abstract class")
			}
			*a |= phpv.ZClassAbstract | phpv.ZClassExplicitAbstract
		case tokenizer.T_FINAL:
			if *a&phpv.ZClassFinal != 0 {
				return errors.New("Multiple final modifiers are not allowed")
			}
			if *a&phpv.ZClassAbstract != 0 {
				return errors.New("Cannot use the final modifier on an abstract class")
			}
			*a |= phpv.ZClassFinal
		case tokenizer.T_READONLY:
			if *a&phpv.ZClassReadonly != 0 {
				return errors.New("Multiple readonly modifiers are not allowed")
			}
			*a |= phpv.ZClassReadonly
		default:
			c.backup()
			return nil
		}
	}
}

func parseZObjectAttr(a *phpv.ZObjectAttr, c compileCtx) error {
	return parseZObjectAttrFull(a, nil, nil, c)
}

func parseZObjectAttrWithAttrs(a *phpv.ZObjectAttr, attrs *[]*phpv.ZAttribute, c compileCtx) error {
	return parseZObjectAttrFull(a, nil, attrs, c)
}

// tryParseAsymmetricSet checks if the current position has "(set)" following an access modifier.
// If it does, it consumes the "(set)" tokens and returns the set-visibility and true.
// If not, it backs up and returns 0 and false.
func tryParseAsymmetricSet(setAccess phpv.ZObjectAttr, c compileCtx) (phpv.ZObjectAttr, bool, error) {
	// We've already consumed the access modifier token (e.g., T_PRIVATE).
	// Now check if next token is '('
	i, err := c.NextItem()
	if err != nil {
		return 0, false, err
	}
	if !i.IsSingle('(') {
		c.backup()
		return 0, false, nil
	}

	// Check for "set" keyword
	i, err = c.NextItem()
	if err != nil {
		return 0, false, err
	}
	if i.Type != tokenizer.T_STRING || i.Data != "set" {
		// Not "(set)", back up both tokens
		c.backup() // back up the non-"set" token
		// We can't back up twice with a single backup(), so we need to
		// handle this differently. Since backup only saves one token,
		// this means the '(' is lost. We need to restructure.
		// Actually, if it's not "set", this is a syntax error in the
		// asymmetric visibility context. PHP would also error here.
		return 0, false, i.Unexpected()
	}

	// Expect ')'
	i, err = c.NextItem()
	if err != nil {
		return 0, false, err
	}
	if !i.IsSingle(')') {
		return 0, false, i.Unexpected()
	}

	return setAccess, true, nil
}

func parseZObjectAttrFull(a *phpv.ZObjectAttr, setModifiers *phpv.ZObjectAttr, attrs *[]*phpv.ZAttribute, c compileCtx) error {
	// parse method attributes (public/protected/private, abstract or final)
	// and PHP 8.0 #[...] attributes
	// Also handles PHP 8.4 asymmetric visibility: public private(set)
	for {
		i, err := c.NextItem()
		if err != nil {
			return err
		}

		switch i.Type {
		case tokenizer.T_DOC_COMMENT, tokenizer.T_COMMENT:
			// skip comments between modifiers
			continue
		case tokenizer.T_ATTRIBUTE:
			// Parse #[...] attributes
			parsed, err := parseAttributes(c)
			if err != nil {
				return err
			}
			if attrs != nil {
				*attrs = append(*attrs, parsed...)
			}
			// Continue looking for more modifiers or attributes
			continue
		case tokenizer.T_STATIC:
			if *a&phpv.ZAttrStatic != 0 {
				return errors.New("Multiple static modifiers are not allowed")
			}
			*a |= phpv.ZAttrStatic
		case tokenizer.T_ABSTRACT:
			if *a&phpv.ZAttrAbstract != 0 {
				return errors.New("Multiple abstract modifiers are not allowed")
			}
			if *a&phpv.ZAttrFinal != 0 {
				return errors.New("Cannot use the final modifier on an abstract method")
			}
			*a |= phpv.ZAttrAbstract
		case tokenizer.T_FINAL:
			if *a&phpv.ZAttrFinal != 0 {
				return errors.New("Multiple final modifiers are not allowed")
			}
			if *a&phpv.ZAttrAbstract != 0 {
				return errors.New("Cannot use the final modifier on an abstract method")
			}
			*a |= phpv.ZAttrFinal
		case tokenizer.T_PUBLIC:
			if *a&phpv.ZAttrAccess != 0 {
				// Already have an access modifier — check for asymmetric visibility: public(set)
				_, ok, err := tryParseAsymmetricSet(phpv.ZAttrPublic, c)
				if err != nil {
					return err
				}
				if !ok {
					return errors.New("Multiple access type modifiers are not allowed")
				}
				// Validate: public(set) is not valid — set visibility must be stricter than read
				return fmt.Errorf("Visibility of property must not be weaker than set visibility")
			}
			*a |= phpv.ZAttrPublic
		case tokenizer.T_PROTECTED:
			if *a&phpv.ZAttrAccess != 0 {
				// Already have an access modifier — check for asymmetric visibility: protected(set)
				setAccess, ok, err := tryParseAsymmetricSet(phpv.ZAttrProtected, c)
				if err != nil {
					return err
				}
				if !ok {
					return errors.New("Multiple access type modifiers are not allowed")
				}
				// Validate: set visibility must be <= read visibility
				readAccess := *a & phpv.ZAttrAccess
				if readAccess == phpv.ZAttrPrivate {
					return fmt.Errorf("Visibility of property must not be weaker than set visibility")
				}
				if setModifiers != nil {
					*setModifiers = setAccess
				}
			} else {
				*a |= phpv.ZAttrProtected
			}
		case tokenizer.T_PRIVATE:
			if *a&phpv.ZAttrAccess != 0 {
				// Already have an access modifier — check for asymmetric visibility: private(set)
				setAccess, ok, err := tryParseAsymmetricSet(phpv.ZAttrPrivate, c)
				if err != nil {
					return err
				}
				if !ok {
					return errors.New("Multiple access type modifiers are not allowed")
				}
				// private(set) is valid with public or protected read visibility
				readAccess := *a & phpv.ZAttrAccess
				_ = readAccess // private(set) is always stricter, so always valid
				if setModifiers != nil {
					*setModifiers = setAccess
				}
			} else {
				*a |= phpv.ZAttrPrivate
			}
		case tokenizer.T_READONLY:
			if *a&phpv.ZAttrReadonly != 0 {
				return errors.New("Multiple readonly modifiers are not allowed")
			}
			*a |= phpv.ZAttrReadonly
		default:
			c.backup()
			return nil
		}
	}
}
