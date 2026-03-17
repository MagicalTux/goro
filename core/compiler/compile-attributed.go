package compiler

import (
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

// compileAttributed handles top-level #[...] attribute groups followed by
// a class, function, or enum declaration.
func compileAttributed(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// i is the T_ATTRIBUTE token (#[)
	// Parse the attributes first
	attrs, err := parseAttributes(c)
	if err != nil {
		return nil, err
	}

	// Now read what follows - should be class, function, enum, or modifiers
	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	// Handle additional attribute groups: #[A] #[B] class Foo {}
	for i.Type == tokenizer.T_ATTRIBUTE {
		moreAttrs, err := parseAttributes(c)
		if err != nil {
			return nil, err
		}
		attrs = append(attrs, moreAttrs...)
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}
	}

	switch i.Type {
	case tokenizer.T_CLASS, tokenizer.T_INTERFACE, tokenizer.T_TRAIT,
		tokenizer.T_ABSTRACT, tokenizer.T_FINAL, tokenizer.T_READONLY:
		r, err := compileClass(i, c)
		if err != nil {
			return nil, err
		}
		if zc, ok := r.(*phpobj.ZClass); ok {
			zc.Attributes = append(attrs, zc.Attributes...)

			// #[\Deprecated] is valid on traits but NOT on classes/interfaces
			if zc.Type != phpv.ZClassTypeTrait {
				for _, attr := range zc.Attributes {
					if attr.ClassName == "Deprecated" || attr.ClassName == "\\Deprecated" {
						kind := "class"
						if zc.Type == phpv.ZClassTypeInterface {
							kind = "interface"
						}
						phpErr := &phpv.PhpError{
							Err:  fmt.Errorf("Cannot apply #[\\Deprecated] to %s %s", kind, zc.Name),
							Code: phpv.E_ERROR,
							Loc:  zc.L,
						}
						c.Global().LogError(phpErr)
						return nil, phpv.ExitError(255)
					}
				}
			}
		}
		// For the wrapper type, also set attributes
		if w, ok := r.(*runClassWithTraitDeprecationCheck); ok { // wrapper from compileClass
			w.class.Attributes = append(attrs, w.class.Attributes...)
			if w.class.Type != phpv.ZClassTypeTrait {
				for _, attr := range w.class.Attributes {
					if attr.ClassName == "Deprecated" || attr.ClassName == "\\Deprecated" {
						kind := "class"
						if w.class.Type == phpv.ZClassTypeInterface {
							kind = "interface"
						}
						phpErr := &phpv.PhpError{
							Err:  fmt.Errorf("Cannot apply #[\\Deprecated] to %s %s", kind, w.class.Name),
							Code: phpv.E_ERROR,
							Loc:  w.class.L,
						}
						c.Global().LogError(phpErr)
						return nil, phpv.ExitError(255)
					}
				}
			}
		}
		return r, nil

	case tokenizer.T_ENUM:
		r, err := compileEnum(i, c)
		if err != nil {
			return nil, err
		}
		var enumClass *phpobj.ZClass
		if zc, ok := r.(*phpobj.ZClass); ok {
			zc.Attributes = append(attrs, zc.Attributes...)
			enumClass = zc
		} else if er, ok := r.(*runEnumRegister); ok {
			er.class.Attributes = append(attrs, er.class.Attributes...)
			enumClass = er.class
		}
		if enumClass != nil {
			for _, attr := range enumClass.Attributes {
				if attr.ClassName == "Deprecated" || attr.ClassName == "\\Deprecated" {
					phpErr := &phpv.PhpError{
						Err:  fmt.Errorf("Cannot apply #[\\Deprecated] to enum %s", enumClass.Name),
						Code: phpv.E_ERROR,
						Loc:  enumClass.L,
					}
					c.Global().LogError(phpErr)
					return nil, phpv.ExitError(255)
				}
			}
		}
		return r, nil

	case tokenizer.T_FUNCTION:
		// Function declaration with attributes
		r, err := compileFunction(i, c)
		if err != nil {
			return nil, err
		}
		// Store attributes on the closure
		if zc, ok := r.(*ZClosure); ok {
			zc.attributes = attrs
			// For named functions, wrap with attribute validation
			if zc.name != "" {
				return &runAttributeValidatedFunc{inner: r, attrs: attrs, target: phpobj.AttributeTARGET_FUNCTION}, nil
			}
		}
		return r, nil

	case tokenizer.T_FN:
		// Arrow function with attributes
		r, err := compileArrowFunction(i, c)
		if err != nil {
			return nil, err
		}
		if zc, ok := r.(*ZClosure); ok {
			zc.attributes = attrs
		}
		return r, nil

	case tokenizer.T_CONST:
		// Top-level constant with attributes: #[Attr] const FOO = expr;
		r, err := compileTopLevelConst(i, c)
		if err != nil {
			return nil, err
		}
		// Store attributes on the constant(s)
		if rtlc, ok := r.(*runTopLevelConst); ok {
			rtlc.attrs = attrs
		} else if runnables, ok := r.(phpv.Runnables); ok {
			// Multiple constants: const A = 1, B = 2;
			// Only the first constant gets the attributes
			for _, sub := range runnables {
				if rtlc, ok := sub.(*runTopLevelConst); ok {
					rtlc.attrs = attrs
					break
				}
			}
		}
		return r, nil

	default:
		return nil, i.Unexpected()
	}
}

// runAttributeValidatedFunc wraps a function/closure declaration to validate
// its attributes at runtime (when the function is registered).
type runAttributeValidatedFunc struct {
	inner  phpv.Runnable
	attrs  []*phpv.ZAttribute
	target int
}

func (r *runAttributeValidatedFunc) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	// Validate internal attributes before registering the function
	if msg := phpobj.ValidateInternalAttributeList(ctx, r.attrs, r.target); msg != "" {
		// Determine location from the inner runnable
		var loc *phpv.Loc
		if zc, ok := r.inner.(*ZClosure); ok {
			loc = zc.start
		}
		phpErr := &phpv.PhpError{
			Err:  fmt.Errorf("%s", msg),
			Code: phpv.E_ERROR,
			Loc:  loc,
		}
		ctx.Global().LogError(phpErr)
		return nil, phpv.ExitError(255)
	}
	return r.inner.Run(ctx)
}

func (r *runAttributeValidatedFunc) Dump(w io.Writer) error {
	return r.inner.Dump(w)
}

type runNoDiscardStatement struct {
	inner phpv.Runnable
}
func (r *runNoDiscardStatement) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	ctx.Global().ClearLastCallable()
	result, err := r.inner.Run(ctx)
	if err != nil { return result, err }
	callable := ctx.Global().LastCallable()
	if callable == nil { return result, nil }
	attrs, funcName, label := getNoDiscardInfo(callable)
	if attrs == nil { return result, nil }
	for _, attr := range attrs {
		if attr.ClassName == "NoDiscard" || attr.ClassName == "\\NoDiscard" {
			if err := ResolveAttrArgs(ctx, attr); err != nil { return nil, err }
			msg := fmt.Sprintf("The return value of %s %s() should either be used or intentionally ignored by casting it as (void)", label, funcName)
			if len(attr.Args) > 0 && attr.Args[0] != nil && attr.Args[0].GetType() != phpv.ZtNull {
				customMsg := attr.Args[0].String()
				if customMsg != "" { msg += ", " + customMsg }
			}
			if warnErr := ctx.Warn("%s", msg, logopt.NoFuncName(true), logopt.ErrType(phpv.E_USER_WARNING)); warnErr != nil { return nil, warnErr }
			break
		}
	}
	return result, nil
}
func (r *runNoDiscardStatement) Dump(w io.Writer) error { return r.inner.Dump(w) }
func getNoDiscardInfo(c phpv.Callable) ([]*phpv.ZAttribute, string, string) {
	if bc, ok := c.(*phpv.BoundedCallable); ok {
		innerAttrs, _, _ := getNoDiscardInfo(bc.Callable)
		if innerAttrs != nil {
			name := bc.Callable.Name()
			lbl := "function"
			if bc.This != nil {
				lbl = "method"
				name = string(bc.This.GetClass().GetName()) + "::" + name
			}
			return innerAttrs, name, lbl
		}
		return nil, "", ""
	}
	if mc, ok := c.(*phpv.MethodCallable); ok {
		innerAttrs, _, _ := getNoDiscardInfo(mc.Callable)
		if innerAttrs != nil { return innerAttrs, string(mc.Class.GetName()) + "::" + mc.Callable.Name(), "method" }
		return nil, "", ""
	}
	if ag, ok := c.(phpv.AttributeGetter); ok {
		attrs := ag.GetAttributes()
		name := c.Name()
		lbl := "function"
		if zc, ok := c.(phpv.ZClosure); ok && zc.GetClass() != nil { lbl = "method"; name = string(zc.GetClass().GetName()) + "::" + name }
		return attrs, name, lbl
	}
	return nil, "", ""
}
