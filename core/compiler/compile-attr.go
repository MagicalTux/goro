package compiler

import (
	"errors"

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
	return parseZObjectAttrWithAttrs(a, nil, c)
}

func parseZObjectAttrWithAttrs(a *phpv.ZObjectAttr, attrs *[]*phpv.ZAttribute, c compileCtx) error {
	// parse method attributes (public/protected/private, abstract or final)
	// and PHP 8.0 #[...] attributes
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
