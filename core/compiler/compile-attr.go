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

			// Propagate strict_types from the compile context
			if rc, ok := c.(*compileRootCtx); ok && rc.strictTypes {
				attr.StrictTypes = true
			}

			// Check for arguments: (
			if i.IsSingle('(') {
				// Parse arguments as constant expressions (supports named args)
				args, argExprs, namedArgs, err := parseAttributeArgs(c)
				if err != nil {
					return nil, err
				}
				resolved, argNames := resolveAttributeNamedArgsWithNames(className, args, namedArgs)
				attr.Args = resolved
				attr.ArgExprs = argExprs
				attr.ArgNames = argNames

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

// orderedNamedArg is a name-value pair for named arguments, preserving insertion order.
type orderedNamedArg struct {
	Name phpv.ZString
	Val  *phpv.ZVal
}

// parseAttributeArgs parses the arguments inside #[Attr(...)].
// Called after the opening '(' has been consumed.
// Returns the parsed argument values, expression runnables for lazy evaluation,
// and named arguments. Named arguments (e.g., message: "foo") are collected
// and returned separately via namedArgs (in insertion order).
func parseAttributeArgs(c compileCtx) (args []*phpv.ZVal, argExprs []phpv.Runnable, namedArgs []orderedNamedArg, err error) {
	// Check for empty args: ()
	i, err := c.NextItem()
	if err != nil {
		return nil, nil, nil, err
	}
	if i.IsSingle(')') {
		return args, nil, nil, nil
	}
	c.backup()

	hasLazy := false

	for {
		// Check for unpacking (...) which is not allowed in attribute arguments
		i, err = c.NextItem()
		if err != nil {
			return nil, nil, nil, err
		}
		if i.Type == tokenizer.T_ELLIPSIS {
			return nil, nil, nil, &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use unpacking in attribute argument list"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
		}

		// Check for named argument: identifier followed by ':'
		// We must read the label and then the next token to check for ':'.
		// We cannot use peekType() here because if the condition fails,
		// backup() would overwrite the peeked token stored in c.next,
		// losing the token that peekType() read from the tokenizer.
		isNamedArg := false
		if i.IsLabel() {
			// Read the next token to see if it's a single ':' (named argument)
			next, err2 := c.NextItem()
			if err2 != nil {
				return nil, nil, nil, err2
			}
			if next.IsSingle(':') {
				isNamedArg = true
			} else {
				// Not a named argument - back up the non-':' token,
				// then pass the label as the first token to compileExpr.
				c.backup()
			}
		}

		if isNamedArg {
			// Named argument: name: expr (the ':' has already been consumed)
			argName := phpv.ZString(i.Data)

			// Parse the value expression
			expr, err := compileExpr(nil, c)
			if err != nil {
				return nil, nil, nil, err
			}

			val, err := expr.Run(c)
			if err != nil {
				val = phpv.ZNULL.ZVal()
			}

			namedArgs = append(namedArgs, orderedNamedArg{Name: argName, Val: val})
		} else {
			// Positional argument - pass i as the already-read first token
			// if it was a label (to avoid needing double backup), or back up
			// and let compileExpr read from scratch.
			var expr phpv.Runnable
			if i.IsLabel() {
				// We already backed up the token after the label, so
				// pass the label item directly to compileExpr.
				expr, err = compileExpr(i, c)
			} else {
				c.backup()
				expr, err = compileExpr(nil, c)
			}
			if err != nil {
				return nil, nil, nil, err
			}

			// Check for dynamic class names in class constant references
			if containsDynamicClassName(expr) || containsAttrDynamicClassName(expr) {
				return nil, nil, nil, &phpv.PhpError{
					Err:  fmt.Errorf("Dynamic class names are not allowed in compile-time class constant references"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  i.Loc(),
				}
			}

			// Check for invalid operations in constant expressions (function calls, variables)
			if containsRuntimeOps(expr) || containsAttrRuntimeOps(expr) {
				return nil, nil, nil, &phpv.PhpError{
					Err:  fmt.Errorf("Constant expression contains invalid operations"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  i.Loc(),
				}
			}

			// Try to evaluate the expression at compile time
			val, err := expr.Run(c)
			if err != nil {
				// If we can't evaluate at compile time, store the expression
				// for lazy evaluation at runtime.
				args = append(args, phpv.ZNULL.ZVal())
				argExprs = append(argExprs, expr)
				hasLazy = true
			} else {
				args = append(args, val)
				argExprs = append(argExprs, nil)
			}
		}

		i, err = c.NextItem()
		if err != nil {
			return nil, nil, nil, err
		}
		if i.IsSingle(')') {
			if !hasLazy {
				argExprs = nil // no lazy args, don't store expressions
			}
			return args, argExprs, namedArgs, nil
		}
		if i.IsSingle(',') {
			// Check for trailing comma before )
			i, err = c.NextItem()
			if err != nil {
				return nil, nil, nil, err
			}
			if i.IsSingle(')') {
				if !hasLazy {
					argExprs = nil
				}
				return args, argExprs, namedArgs, nil
			}
			c.backup()
			continue
		}

		return nil, nil, nil, i.Unexpected()
	}
}

// resolveAttributeNamedArgs resolves named arguments into positional arguments
// for known built-in attribute classes. Unknown attributes have their named
// args appended after positional args.
func resolveAttributeNamedArgs(className phpv.ZString, args []*phpv.ZVal, namedArgs []orderedNamedArg) []*phpv.ZVal {
	if len(namedArgs) == 0 {
		return args
	}

	// Parameter name -> position mapping for known attribute classes
	var paramMap map[phpv.ZString]int

	switch className {
	case "Deprecated", "\\Deprecated":
		paramMap = map[phpv.ZString]int{"message": 0, "since": 1}
	case "Attribute", "\\Attribute":
		paramMap = map[phpv.ZString]int{"flags": 0}
	case "NoDiscard", "\\NoDiscard":
		paramMap = map[phpv.ZString]int{"message": 0}
	default:
		// For unknown attribute classes, append named args after positional args
		// (in the order they were specified)
		for _, na := range namedArgs {
			args = append(args, na.Val)
		}
		return args
	}

	// Find max position needed
	maxPos := len(args) - 1
	for _, na := range namedArgs {
		if pos, ok := paramMap[na.Name]; ok && pos > maxPos {
			maxPos = pos
		}
	}

	// Extend args slice to accommodate all positions
	for len(args) <= maxPos {
		args = append(args, phpv.ZString("").ZVal())
	}

	// Place named args at their correct positions
	for _, na := range namedArgs {
		if pos, ok := paramMap[na.Name]; ok {
			args[pos] = na.Val
		}
	}

	return args
}

// resolveAttributeNamedArgsWithNames is like resolveAttributeNamedArgs but also
// returns the argument names (for reflection). Empty string = positional.
func resolveAttributeNamedArgsWithNames(className phpv.ZString, args []*phpv.ZVal, namedArgs []orderedNamedArg) ([]*phpv.ZVal, []phpv.ZString) {
	if len(namedArgs) == 0 {
		return args, nil
	}

	// Parameter name -> position mapping for known attribute classes
	var paramMap map[phpv.ZString]int

	switch className {
	case "Deprecated", "\\Deprecated":
		paramMap = map[phpv.ZString]int{"message": 0, "since": 1}
	case "Attribute", "\\Attribute":
		paramMap = map[phpv.ZString]int{"flags": 0}
	case "NoDiscard", "\\NoDiscard":
		paramMap = map[phpv.ZString]int{"message": 0}
	default:
		// For unknown attribute classes, append named args after positional args
		// Build names list: empty for positional, name for named (in insertion order)
		names := make([]phpv.ZString, len(args))
		for _, na := range namedArgs {
			args = append(args, na.Val)
			names = append(names, na.Name)
		}
		return args, names
	}

	// Find max position needed
	maxPos := len(args) - 1
	for _, na := range namedArgs {
		if pos, ok := paramMap[na.Name]; ok && pos > maxPos {
			maxPos = pos
		}
	}

	// Extend args slice to accommodate all positions
	names := make([]phpv.ZString, len(args))
	for len(args) <= maxPos {
		args = append(args, phpv.ZString("").ZVal())
		names = append(names, "")
	}

	// Place named args at their correct positions
	for _, na := range namedArgs {
		if pos, ok := paramMap[na.Name]; ok {
			args[pos] = na.Val
			for len(names) <= pos {
				names = append(names, "")
			}
			names[pos] = na.Name
		}
	}

	return args, names
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
// parenConsumedByAsymmetric is set to true when tryParseAsymmetricSet consumed
// '(' but found it was not "(set)". The token after '(' has been backed up.
// Callers should check this flag to handle DNF type parsing.
var parenConsumedByAsymmetric bool

func tryParseAsymmetricSet(setAccess phpv.ZObjectAttr, c compileCtx) (phpv.ZObjectAttr, bool, error) {
	parenConsumedByAsymmetric = false
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
		// Not "(set)" - could be a DNF type like public (X&Y)|null $prop
		// Back up the non-"set" token. The '(' is consumed and lost.
		// Signal via parenConsumedByAsymmetric so the caller can handle DNF.
		c.backup()
		parenConsumedByAsymmetric = true
		return 0, false, nil
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
			*a |= phpv.ZAttrAbstract
		case tokenizer.T_FINAL:
			if *a&phpv.ZAttrFinal != 0 {
				return errors.New("Multiple final modifiers are not allowed")
			}
			*a |= phpv.ZAttrFinal
		case tokenizer.T_PUBLIC, tokenizer.T_PROTECTED, tokenizer.T_PRIVATE:
			var thisAccess phpv.ZObjectAttr
			switch i.Type {
			case tokenizer.T_PUBLIC:
				thisAccess = phpv.ZAttrPublic
			case tokenizer.T_PROTECTED:
				thisAccess = phpv.ZAttrProtected
			case tokenizer.T_PRIVATE:
				thisAccess = phpv.ZAttrPrivate
			}

			// First, try to parse as asymmetric visibility: modifier(set)
			setAccess, isAsymmetric, err := tryParseAsymmetricSet(thisAccess, c)
			if err != nil {
				return err
			}

			if parenConsumedByAsymmetric {
				// '(' was consumed but it wasn't "(set)" - this is a DNF type.
				// Set the access modifier and return. The caller will handle DNF parsing.
				if *a&phpv.ZAttrAccess != 0 {
					return errors.New("Multiple access type modifiers are not allowed")
				}
				*a |= thisAccess
				return nil
			}

			if isAsymmetric {
				// Check for duplicate set modifier
				if setModifiers != nil && *setModifiers != 0 {
					return errors.New("Multiple access type modifiers are not allowed")
				}

				// This is a (set) modifier — it specifies write visibility only
				if *a&phpv.ZAttrAccess == 0 {
					// No read modifier yet — implicit public
					*a |= phpv.ZAttrPublic
				}

				// Defer detailed validation to compile-class.go where class/property
				// names are available for proper error messages. Only reject
				// combinations that are syntactically invalid here.

				if setModifiers != nil {
					*setModifiers = setAccess
				}
			} else {
				// Normal (non-asymmetric) access modifier
				if *a&phpv.ZAttrAccess != 0 {
					return errors.New("Multiple access type modifiers are not allowed")
				}
				*a |= thisAccess
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
