package compiler

import (
	"strings"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

// compileNamespace handles:
//   namespace Foo\Bar;           (statement form)
//   namespace Foo\Bar { ... }    (block form)
//   namespace { ... }            (global block form)
//   namespace\Name               (relative name expression — delegates to compileExpr)
func compileNamespace(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// Get the root context to set namespace
	root := getRootCtx(c)
	if root == nil {
		return nil, i.Unexpected()
	}

	// Read the namespace name or check for '{' or '\'
	next, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	// If followed by T_NS_SEPARATOR, this is a namespace\Name expression (e.g. namespace\func())
	if next.Type == tokenizer.T_NS_SEPARATOR {
		c.backup() // backs up NS_SEPARATOR so compileExpr's T_NAMESPACE handler sees it
		// Delegate to expression compiler — pass the original T_NAMESPACE token
		r, err := compileExpr(i, c)
		if err != nil {
			return nil, err
		}
		// compileNamespace is called with skip=true so we need to consume the semicolon
		semi, err := c.NextItem()
		if err != nil {
			return r, err
		}
		if !semi.IsExpressionEnd() {
			return nil, semi.Unexpected()
		}
		return r, nil
	}

	var nsName phpv.ZString

	if next.IsSingle('{') {
		// namespace { ... } — global namespace block
		nsName = ""
		oldNs := root.namespace
		oldUseMap := root.useMap
		oldUseFuncMap := root.useFuncMap
		oldUseConstMap := root.useConstMap
		root.namespace = nsName
		root.useMap = make(map[phpv.ZString]phpv.ZString)
		root.useFuncMap = make(map[phpv.ZString]phpv.ZString)
		root.useConstMap = make(map[phpv.ZString]phpv.ZString)

		body, err := compileBase(nil, c)

		root.namespace = oldNs
		root.useMap = oldUseMap
		root.useFuncMap = oldUseFuncMap
		root.useConstMap = oldUseConstMap

		if err != nil {
			return nil, err
		}
		return body, nil
	}

	// Read namespace name: T_STRING parts joined by T_NS_SEPARATOR
	if next.Type != tokenizer.T_STRING {
		return nil, next.Unexpected()
	}
	nsName = phpv.ZString(next.Data)

	for {
		next, err = c.NextItem()
		if err != nil {
			return nil, err
		}
		if next.Type == tokenizer.T_NS_SEPARATOR {
			part, err := c.NextItem()
			if err != nil {
				return nil, err
			}
			if part.Type != tokenizer.T_STRING {
				return nil, part.Unexpected()
			}
			nsName = nsName + "\\" + phpv.ZString(part.Data)
		} else {
			break
		}
	}

	if next.IsSingle(';') {
		// Statement form: namespace Foo\Bar;
		// Set namespace on the root context and reset use maps
		root.namespace = nsName
		root.useMap = make(map[phpv.ZString]phpv.ZString)
		root.useFuncMap = make(map[phpv.ZString]phpv.ZString)
		root.useConstMap = make(map[phpv.ZString]phpv.ZString)
		return nil, nil
	}

	if next.IsSingle('{') {
		// Block form: namespace Foo\Bar { ... }
		oldNs := root.namespace
		oldUseMap := root.useMap
		oldUseFuncMap := root.useFuncMap
		oldUseConstMap := root.useConstMap
		root.namespace = nsName
		root.useMap = make(map[phpv.ZString]phpv.ZString)
		root.useFuncMap = make(map[phpv.ZString]phpv.ZString)
		root.useConstMap = make(map[phpv.ZString]phpv.ZString)

		body, err := compileBase(nil, c)

		root.namespace = oldNs
		root.useMap = oldUseMap
		root.useFuncMap = oldUseFuncMap
		root.useConstMap = oldUseConstMap

		if err != nil {
			return nil, err
		}
		return body, nil
	}

	return nil, next.Unexpected()
}

// compileUse handles:
//   use Foo\Bar;
//   use Foo\Bar as Baz;
//   use Foo\{Bar, Baz};
//   use function Foo\bar;
//   use const Foo\BAR;
func compileUse(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	root := getRootCtx(c)
	if root == nil {
		return nil, i.Unexpected()
	}

	next, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	// Check for "use function" or "use const"
	useType := "class" // default
	if next.Type == tokenizer.T_FUNCTION {
		useType = "function"
		next, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	} else if next.Type == tokenizer.T_CONST {
		useType = "const"
		next, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	// Parse one or more use declarations separated by commas
	for {
		// Read the full name
		var fullName phpv.ZString
		hasLeadingBackslash := false

		if next.Type == tokenizer.T_NS_SEPARATOR {
			hasLeadingBackslash = true
			next, err = c.NextItem()
			if err != nil {
				return nil, err
			}
		}

		if next.Type != tokenizer.T_STRING {
			return nil, next.Unexpected()
		}

		fullName = phpv.ZString(next.Data)

		for {
			next, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			if next.Type == tokenizer.T_NS_SEPARATOR {
				peek, err := c.NextItem()
				if err != nil {
					return nil, err
				}
				if peek.IsSingle('{') {
					// Group use: use Foo\{Bar, Baz}
					err = parseGroupUse(root, fullName, useType, c)
					if err != nil {
						return nil, err
					}
					// After group use, expect ; or ,
					next, err = c.NextItem()
					if err != nil {
						return nil, err
					}
					if next.IsSingle(';') {
						return nil, nil
					}
					if next.IsSingle(',') {
						next, err = c.NextItem()
						if err != nil {
							return nil, err
						}
						continue
					}
					return nil, next.Unexpected()
				}
				if peek.Type != tokenizer.T_STRING {
					return nil, peek.Unexpected()
				}
				fullName = fullName + "\\" + phpv.ZString(peek.Data)
			} else {
				break
			}
		}

		_ = hasLeadingBackslash // leading backslash is optional in use statements

		// Check for "as Alias"
		alias := lastPart(fullName)
		if next.Type == tokenizer.T_AS {
			aliasItem, err := c.NextItem()
			if err != nil {
				return nil, err
			}
			if aliasItem.Type != tokenizer.T_STRING {
				return nil, aliasItem.Unexpected()
			}
			alias = phpv.ZString(aliasItem.Data)
			next, err = c.NextItem()
			if err != nil {
				return nil, err
			}
		}

		// PHP warning: non-compound names in use statements have no effect
		if !strings.Contains(string(fullName), "\\") {
			ctx := c.(phpv.Context)
			ctx.Warn("The use statement with non-compound name '%s' has no effect", fullName, logopt.NoFuncName(true))
		}

		// Register the alias
		switch useType {
		case "class":
			root.useMap[alias] = fullName
		case "function":
			root.useFuncMap[alias] = fullName
		case "const":
			root.useConstMap[alias] = fullName
		}

		if next.IsSingle(';') {
			return nil, nil
		}
		if next.IsSingle(',') {
			next, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			continue
		}
		return nil, next.Unexpected()
	}
}

// parseGroupUse handles: use Prefix\{Name1, Name2 as Alias2, ...};
func parseGroupUse(root *compileRootCtx, prefix phpv.ZString, useType string, c compileCtx) error {
	for {
		next, err := c.NextItem()
		if err != nil {
			return err
		}

		if next.IsSingle('}') {
			return nil
		}

		// Check for per-item type override: use Foo\{function bar, const BAZ}
		itemType := useType
		if next.Type == tokenizer.T_FUNCTION {
			itemType = "function"
			next, err = c.NextItem()
			if err != nil {
				return err
			}
		} else if next.Type == tokenizer.T_CONST {
			itemType = "const"
			next, err = c.NextItem()
			if err != nil {
				return err
			}
		}

		if next.Type != tokenizer.T_STRING {
			return next.Unexpected()
		}

		name := phpv.ZString(next.Data)
		for {
			next, err = c.NextItem()
			if err != nil {
				return err
			}
			if next.Type == tokenizer.T_NS_SEPARATOR {
				part, err := c.NextItem()
				if err != nil {
					return err
				}
				if part.Type != tokenizer.T_STRING {
					return part.Unexpected()
				}
				name = name + "\\" + phpv.ZString(part.Data)
			} else {
				break
			}
		}

		fullName := prefix + "\\" + name
		alias := lastPart(name)

		if next.Type == tokenizer.T_AS {
			aliasItem, err := c.NextItem()
			if err != nil {
				return err
			}
			if aliasItem.Type != tokenizer.T_STRING {
				return aliasItem.Unexpected()
			}
			alias = phpv.ZString(aliasItem.Data)
			next, err = c.NextItem()
			if err != nil {
				return err
			}
		}

		switch itemType {
		case "class":
			root.useMap[alias] = fullName
		case "function":
			root.useFuncMap[alias] = fullName
		case "const":
			root.useConstMap[alias] = fullName
		}

		if next.IsSingle('}') {
			return nil
		}
		if !next.IsSingle(',') {
			return next.Unexpected()
		}
	}
}

// lastPart returns the last component of a backslash-separated name.
// e.g. "Foo\Bar\Baz" → "Baz", "Simple" → "Simple"
func lastPart(name phpv.ZString) phpv.ZString {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '\\' {
			return name[i+1:]
		}
	}
	return name
}

// getRootCtx walks up the compile context chain to find the compileRootCtx.
func getRootCtx(c compileCtx) *compileRootCtx {
	switch v := c.(type) {
	case *compileRootCtx:
		return v
	case *zclosureCompileCtx:
		return getRootCtx(v.compileCtx)
	case *zclassCompileCtx:
		return getRootCtx(v.compileCtx)
	default:
		return nil
	}
}
