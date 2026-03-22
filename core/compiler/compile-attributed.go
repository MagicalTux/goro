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
			// Skip this check if #[DelayedTargetValidation] is present
			if zc.Type != phpv.ZClassTypeTrait && !hasDelayedTargetValidationAttr(zc.Attributes) {
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
			if w.class.Type != phpv.ZClassTypeTrait && !hasDelayedTargetValidationAttr(w.class.Attributes) {
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
		if enumClass != nil && !hasDelayedTargetValidationAttr(enumClass.Attributes) {
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
		// const with attributes inside a function body is a parse error
		if c.getFunc() != nil {
			return nil, i.Unexpected()
		}
		// Top-level constant with attributes: #[Attr] const FOO = expr;
		r, err := compileTopLevelConst(i, c)
		if err != nil {
			return nil, err
		}
		// Check if multiple constants are declared with attributes (not allowed)
		if _, ok := r.(phpv.Runnables); ok {
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Cannot apply attributes to multiple constants at once"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
		}
		// Store attributes on the constant
		if rtlc, ok := r.(*runTopLevelConst); ok {
			rtlc.attrs = attrs
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

// noDiscardAliasName stores an alias method name for NoDiscard warnings.
// When __call() or __callStatic() is invoked, the caller sets this to
// the original method name (e.g., "test") so the warning says
// "method Clazz::test()" instead of "method Clazz::__call()".
var noDiscardAliasName string

// noDiscardAlreadyEmitted is set to true when the NoDiscard warning was
// already emitted before the magic method call, so runNoDiscardStatement
// doesn't emit it again.
var noDiscardAlreadyEmitted bool

// inNoDiscardContext is true when the current execution is inside a
// runNoDiscardStatement (i.e., the call result is being discarded).
var inNoDiscardContext bool

// SetNoDiscardAlias sets the alias name for the next NoDiscard check.
func SetNoDiscardAlias(name string) {
	noDiscardAliasName = name
}

// EmitNoDiscardForMagicCall checks if a magic method (__call/__callStatic) has
// #[\NoDiscard] and if so, emits the warning using the virtual method name.
// This is called BEFORE the magic method body executes so the warning appears
// before any output from the method body.
func EmitNoDiscardForMagicCall(ctx phpv.Context, method phpv.Callable, className phpv.ZString, virtualMethodName string) error {
	if !inNoDiscardContext {
		return nil // Only emit when the result is being discarded
	}
	if ag, ok := method.(phpv.AttributeGetter); ok {
		for _, attr := range ag.GetAttributes() {
			if attr.ClassName == "NoDiscard" || attr.ClassName == "\\NoDiscard" {
				if err := ResolveAttrArgs(ctx, attr); err != nil {
					return err
				}
				if err := ValidateNoDiscardArgs(ctx, attr); err != nil {
					return err
				}
				funcName := string(className) + "::" + virtualMethodName
				msg := fmt.Sprintf("The return value of method %s() should either be used or intentionally ignored by casting it as (void)", funcName)
				if len(attr.Args) > 0 && attr.Args[0] != nil && attr.Args[0].GetType() != phpv.ZtNull {
					customMsg := attr.Args[0].String()
					if customMsg != "" {
						msg += ", " + customMsg
					}
				}
				noDiscardAlreadyEmitted = true
				return ctx.Warn("%s", msg, logopt.NoFuncName(true), logopt.ErrType(phpv.E_USER_WARNING))
			}
		}
	}
	// Also check if callable is a BoundedCallable wrapping the method
	if bc, ok := method.(*phpv.BoundedCallable); ok {
		return EmitNoDiscardForMagicCall(ctx, bc.Callable, className, virtualMethodName)
	}
	return nil
}

type runNoDiscardStatement struct {
	inner phpv.Runnable
}
func (r *runNoDiscardStatement) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	ctx.Global().ClearLastCallable()
	noDiscardAlreadyEmitted = false
	prevInCtx := inNoDiscardContext
	inNoDiscardContext = true
	result, err := r.inner.Run(ctx)
	inNoDiscardContext = prevInCtx
	if err != nil { return result, err }
	// If the NoDiscard warning was already emitted before the call (e.g., for __call/__callStatic),
	// don't emit it again.
	if noDiscardAlreadyEmitted {
		noDiscardAlreadyEmitted = false
		return result, nil
	}
	callable := ctx.Global().LastCallable()
	if callable == nil { return result, nil }
	attrs, funcName, label := getNoDiscardInfo(callable)
	if attrs == nil { return result, nil }
	for _, attr := range attrs {
		if attr.ClassName == "NoDiscard" || attr.ClassName == "\\NoDiscard" {
			if err := ResolveAttrArgs(ctx, attr); err != nil { return nil, err }
			// Validate NoDiscard constructor arg types
			if err := ValidateNoDiscardArgs(ctx, attr); err != nil { return nil, err }
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
	// Consume any alias name
	alias := noDiscardAliasName
	noDiscardAliasName = ""

	if bc, ok := c.(*phpv.BoundedCallable); ok {
		innerAttrs, _, _ := getNoDiscardInfo(bc.Callable)
		if innerAttrs != nil {
			name := bc.Callable.Name()
			// Use alias name for __call/__callStatic
			if alias != "" && (name == "__call" || name == "__callStatic" || name == "__callstatic") {
				name = alias
			}
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
		if innerAttrs != nil {
			name := mc.Callable.Name()
			if alias != "" && (name == "__call" || name == "__callStatic" || name == "__callstatic") {
				name = alias
			}
			return innerAttrs, string(mc.Class.GetName()) + "::" + name, "method"
		}
		return nil, "", ""
	}
	if ag, ok := c.(phpv.AttributeGetter); ok {
		attrs := ag.GetAttributes()
		name := c.Name()
		if alias != "" && (name == "__call" || name == "__callStatic" || name == "__callstatic") {
			name = alias
		}
		lbl := "function"
		if zc, ok := c.(phpv.ZClosure); ok && zc.GetClass() != nil {
			lbl = "method"
			name = string(zc.GetClass().GetName()) + "::" + name
		}
		return attrs, name, lbl
	}
	return nil, "", ""
}

// hasDelayedTargetValidationAttr checks if an attribute list contains #[DelayedTargetValidation].
func hasDelayedTargetValidationAttr(attrs []*phpv.ZAttribute) bool {
	for _, attr := range attrs {
		if attr.ClassName == "DelayedTargetValidation" || attr.ClassName == "\\DelayedTargetValidation" {
			return true
		}
	}
	return false
}
