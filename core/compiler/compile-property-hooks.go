package compiler

import (
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

// runParentPropHookCall implements parent::$prop::get() and parent::$prop::set()
// in property hooks. At runtime, it resolves the parent class, finds the property,
// and calls the parent's hook (or accesses the backing value for plain properties).
type runParentPropHookCall struct {
	propName phpv.ZString    // property name (without $)
	hookType string          // "get" or "set"
	argExprs phpv.Runnables  // argument expressions
	l        *phpv.Loc
}

func (r *runParentPropHookCall) Dump(w io.Writer) error {
	if r.hookType == "set" {
		_, err := fmt.Fprintf(w, "parent::$%s::set(...)", r.propName)
		return err
	}
	_, err := fmt.Fprintf(w, "parent::$%s::get()", r.propName)
	return err
}

func (r *runParentPropHookCall) Loc() *phpv.Loc {
	return r.l
}

func (r *runParentPropHookCall) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	// Resolve parent class
	cls := ctx.Class()
	if cls == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Cannot use \"parent\" when no class scope is active"))
	}
	parentCls := cls.GetParent()
	if parentCls == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			"Cannot use \"parent\" when current class scope has no parent")
	}

	parentZC, ok := parentCls.(*phpobj.ZClass)
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			"Cannot use \"parent\" when current class scope has no parent")
	}

	// Find the property in the parent class hierarchy
	var prop *phpv.ZClassProp
	var declClass *phpobj.ZClass
	for cur := parentZC; cur != nil; cur = cur.Extends {
		for _, p := range cur.Props {
			if p.VarName == r.propName {
				prop = p
				declClass = cur
				break
			}
		}
		if prop != nil {
			break
		}
	}

	if prop == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Undefined property %s::$%s", parentZC.GetName(), r.propName))
	}

	// Evaluate argument expressions
	args := make([]*phpv.ZVal, len(r.argExprs))
	for i, expr := range r.argExprs {
		v, err := expr.Run(ctx)
		if err != nil {
			return nil, err
		}
		args[i] = v
	}

	// Get $this
	this := ctx.This()
	if this == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Using $this when not in object context")
	}

	obj, ok := this.(*phpobj.ZObject)
	if !ok {
		if uw, ok2 := this.(interface{ Unwrap() phpv.ZObject }); ok2 {
			obj, ok = uw.Unwrap().(*phpobj.ZObject)
		}
	}
	if obj == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, "Using $this when not in object context")
	}

	if r.hookType == "get" {
		// parent::$prop::get()
		if prop.HasHooks && prop.GetHook != nil {
			// Parent has a get hook — call it
			if len(args) > 0 {
				return nil, phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("%s::$%s::get() expects exactly 0 arguments, %d given",
						declClass.GetName(), r.propName, len(args)))
			}
			return obj.RunParentGetHook(ctx, r.propName, prop.GetHook, declClass)
		}
		// Parent has no get hook (plain property or hooked without get) — read backing value
		if len(args) > 0 {
			return nil, phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("%s::$%s::get() expects exactly 0 arguments, %d given",
					declClass.GetName(), r.propName, len(args)))
		}
		return obj.ReadParentBacking(ctx, r.propName, declClass)
	}

	// parent::$prop::set($value)
	if prop.HasHooks && prop.SetHook != nil {
		// Parent has a set hook — call it
		// For user-defined hooks, extra args are tolerated (they are user functions)
		if len(args) == 0 {
			return nil, phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("%s::$%s::set() expects exactly 1 argument, 0 given",
					declClass.GetName(), r.propName))
		}
		err := obj.RunParentSetHook(ctx, r.propName, prop, args[0], declClass)
		if err != nil {
			return nil, err
		}
		// parent::$prop::set() returns the value that was set
		return args[0], nil
	}
	// Parent has no set hook (plain property) — write backing value directly
	if len(args) != 1 {
		expected := 1
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("%s::$%s::set() expects exactly %d argument, %d given",
				declClass.GetName(), r.propName, expected, len(args)))
	}
	obj.WriteParentBacking(ctx, r.propName, args[0], declClass)
	return args[0], nil
}

// IsFuncCallExpression marks this as a function call expression so that
// ++parent::$prop::get() correctly produces the "Can't use method return value in write context" error.
func (r *runParentPropHookCall) IsFuncCallExpression() {}


// hookReferencesBacking checks if a compiled Runnable references $this->propName,
// which means the property has a backing store and is not virtual.
// This recursively walks the compiled AST tree.
func hookReferencesBacking(r phpv.Runnable, propName phpv.ZString) bool {
	if r == nil {
		return false
	}
	// Check if this is a $this->prop access (read or write)
	if ov, ok := r.(*runObjectVar); ok {
		if rv, ok2 := ov.ref.(*runVariable); ok2 && rv.v == "this" && ov.varName == propName {
			return true
		}
	}
	// Recursively check all children using the existing GetChildren helper
	for _, child := range GetChildren(r) {
		if hookReferencesBacking(child, propName) {
			return true
		}
	}
	return false
}

// compilePropertyHooks parses the { get { ... } set($value) { ... } } block
// after a property declaration (PHP 8.4 property hooks).
func compilePropertyHooks(prop *phpv.ZClassProp, class *phpobj.ZClass, c compileCtx) error {
	prop.HasHooks = true
	hasGet := false
	hasSet := false
	hookCount := 0
	for {
		i, err := c.NextItem()
		if err != nil {
			return err
		}

		if i.IsSingle('}') {
			// Empty hook list is not allowed
			if hookCount == 0 {
				return &phpv.PhpError{
					Err:  fmt.Errorf("Property hook list must not be empty"),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  i.Loc(),
				}
			}

			// After parsing all hooks, determine if the property is backed.
			// A property is backed if:
			// - Any hook references $this->propName (backing store access), OR
			// - The set hook is an arrow expression (set => expr), which implicitly
			//   writes the result to the backing store.
			if hookReferencesBacking(prop.GetHook, prop.VarName) ||
				hookReferencesBacking(prop.SetHook, prop.VarName) {
				prop.IsBacked = true
			}
			// Arrow set hooks (not Runnables/block) always write to backing store
			if prop.SetHook != nil {
				if _, isBlock := prop.SetHook.(phpv.Runnables); !isBlock {
					prop.IsBacked = true
				}
			}
			return nil
		}

		hookCount++

		// Parse optional attributes before hook name
		var hookAttrs []*phpv.ZAttribute
		for i.Type == tokenizer.T_ATTRIBUTE {
			// Consume the attribute and its arguments
			parsed, err := parseAttributes(c)
			if err != nil {
				return err
			}
			hookAttrs = append(hookAttrs, parsed...)
			i, err = c.NextItem()
			if err != nil {
				return err
			}
		}

		// Check for #[\NoDiscard] on property hooks (not allowed)
		for _, attr := range hookAttrs {
			if attr.ClassName == "NoDiscard" || attr.ClassName == "\\NoDiscard" {
				// Check if DelayedTargetValidation is also present
				hasDelayed := false
				for _, a := range hookAttrs {
					if a.ClassName == "DelayedTargetValidation" || a.ClassName == "\\DelayedTargetValidation" {
						hasDelayed = true
						break
					}
				}
				if !hasDelayed {
					return &phpv.PhpError{
						Err:  fmt.Errorf("#[\\NoDiscard] is not supported for property hooks"),
						Code: phpv.E_COMPILE_ERROR,
						Loc:  i.Loc(),
					}
				}
			}
		}

		// Check for visibility modifiers (not allowed on hooks)
		if i.Type == tokenizer.T_PUBLIC || i.Type == tokenizer.T_PROTECTED || i.Type == tokenizer.T_PRIVATE {
			modName := "public"
			if i.Type == tokenizer.T_PROTECTED {
				modName = "protected"
			} else if i.Type == tokenizer.T_PRIVATE {
				modName = "private"
			}
			return &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use the %s modifier on a property hook", modName),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
		}

		// Check for static modifier (not allowed on hooks)
		if i.Type == tokenizer.T_STATIC {
			return &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use the static modifier on a property hook"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
		}

		// Parse optional modifiers before hook name (final, abstract)
		hookIsFinal := false
		hookIsAbstract := false
		for i.Type == tokenizer.T_FINAL || i.Type == tokenizer.T_ABSTRACT {
			if i.Type == tokenizer.T_FINAL {
				hookIsFinal = true
			}
			if i.Type == tokenizer.T_ABSTRACT {
				hookIsAbstract = true
			}
			i, err = c.NextItem()
			if err != nil {
				return err
			}
		}

		// Hook inherits abstract from property modifier
		effectiveAbstract := hookIsAbstract || prop.Modifiers.Has(phpv.ZAttrAbstract)

		// Validate modifier combinations
		if effectiveAbstract && hookIsFinal {
			return &phpv.PhpError{
				Err:  fmt.Errorf("Property hook cannot be both abstract and final"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
		}
		if effectiveAbstract && prop.Modifiers.IsPrivate() {
			return &phpv.PhpError{
				Err:  fmt.Errorf("Property hook cannot be both abstract and private"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
		}
		if hookIsFinal && prop.Modifiers.IsPrivate() {
			return &phpv.PhpError{
				Err:  fmt.Errorf("Property hook cannot be both final and private"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
		}

		// Skip optional '&' for by-ref hooks
		if i.IsSingle('&') {
			i, err = c.NextItem()
			if err != nil {
				return err
			}
		}

		if i.Type != tokenizer.T_STRING {
			return i.Unexpected()
		}

		switch i.Data {
		case "get":
			// Check for duplicate get hook
			if hasGet {
				return &phpv.PhpError{
					Err:  fmt.Errorf("Cannot redeclare property hook \"get\""),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  i.Loc(),
				}
			}
			hasGet = true
			prop.HasGetDeclared = true

			i, err = c.NextItem()
			if err != nil {
				return err
			}

			// get hook must not have a parameter list
			if i.IsSingle('(') {
				return &phpv.PhpError{
					Err:  fmt.Errorf("get hook of property %s::$%s must not have a parameter list", class.Name, prop.VarName),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  i.Loc(),
				}
			}

			if i.IsSingle(';') {
				prop.GetIsAbstract = true
				continue // abstract get
			}
			// If hook is declared abstract but has a body, the "abstract property must have abstract hook" error
			// will be caught later
			// Wrap compilation in a function context so __FUNCTION__ resolves to $prop::get
			hookClosure := &ZClosure{name: phpv.ZString(fmt.Sprintf("$%s::get", prop.VarName))}
			hookCtx := &zclosureCompileCtx{c, hookClosure}
			if i.IsSingle('{') {
				body, err := compileBase(i, hookCtx)
				if err != nil {
					return err
				}
				prop.GetHook = body
			} else if i.Type == tokenizer.T_DOUBLE_ARROW {
				expr, err := compileExpr(nil, hookCtx)
				if err != nil {
					return err
				}
				prop.GetHook = &runReturn{v: expr, l: i.Loc()}
				i, err = c.NextItem()
				if err != nil {
					return err
				}
				if !i.IsSingle(';') {
					return i.Unexpected()
				}
			} else {
				return i.Unexpected()
			}

		case "set":
			// Check for duplicate set hook
			if hasSet {
				return &phpv.PhpError{
					Err:  fmt.Errorf("Cannot redeclare property hook \"set\""),
					Code: phpv.E_COMPILE_ERROR,
					Loc:  i.Loc(),
				}
			}
			hasSet = true
			prop.HasSetDeclared = true

			prop.SetParam = "value"
			i, err = c.NextItem()
			if err != nil {
				return err
			}
			if i.IsSingle('(') {
				// Parse set parameter with validation
				for {
					i, err = c.NextItem()
					if err != nil {
						return err
					}

					// Check for variadic parameter (not allowed)
					if i.Type == tokenizer.T_ELLIPSIS {
						return &phpv.PhpError{
							Err:  fmt.Errorf("Parameter $%s of set hook %s::$%s must not be variadic", prop.SetParam, class.Name, prop.VarName),
							Code: phpv.E_COMPILE_ERROR,
							Loc:  i.Loc(),
						}
					}

					// Check for by-reference parameter (not allowed)
					if i.IsSingle('&') {
						// Peek at next to get the variable name
						return &phpv.PhpError{
							Err:  fmt.Errorf("Parameter $%s of set hook %s::$%s must not be pass-by-reference", prop.SetParam, class.Name, prop.VarName),
							Code: phpv.E_COMPILE_ERROR,
							Loc:  i.Loc(),
						}
					}

					if i.Type == tokenizer.T_VARIABLE {
						prop.SetParam = phpv.ZString(i.Data[1:])
						i, err = c.NextItem()
						if err != nil {
							return err
						}

						// Check for default value (not allowed)
						if i.IsSingle('=') {
							return &phpv.PhpError{
								Err:  fmt.Errorf("Parameter $%s of set hook %s::$%s must not have a default value", prop.SetParam, class.Name, prop.VarName),
								Code: phpv.E_COMPILE_ERROR,
								Loc:  i.Loc(),
							}
						}

						break
					}
					if i.IsSingle(')') {
						break
					}
				}
				if !i.IsSingle(')') {
					return i.Unexpected()
				}
				i, err = c.NextItem()
				if err != nil {
					return err
				}
			}
			if i.IsSingle(';') {
				prop.SetIsAbstract = true
				continue // abstract set
			}
			// Wrap compilation in a function context so __FUNCTION__ resolves to $prop::set
			setHookClosure := &ZClosure{name: phpv.ZString(fmt.Sprintf("$%s::set", prop.VarName))}
			setHookCtx := &zclosureCompileCtx{c, setHookClosure}
			if i.IsSingle('{') {
				body, err := compileBase(i, setHookCtx)
				if err != nil {
					return err
				}
				prop.SetHook = body
			} else if i.Type == tokenizer.T_DOUBLE_ARROW {
				expr, err := compileExpr(nil, setHookCtx)
				if err != nil {
					return err
				}
				prop.SetHook = expr
				i, err = c.NextItem()
				if err != nil {
					return err
				}
				if !i.IsSingle(';') {
					return i.Unexpected()
				}
			} else {
				return i.Unexpected()
			}

		default:
			return &phpv.PhpError{
				Err:  fmt.Errorf("Unknown hook \"%s\" for property %s::$%s, expected \"get\" or \"set\"", i.Data, class.Name, prop.VarName),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
		}
	}
}
