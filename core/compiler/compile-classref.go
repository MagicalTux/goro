package compiler

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phperr"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// when classname::$something is used
type runClassStaticVarRef struct {
	className phpv.Runnable
	varName   phpv.ZString
	l         *phpv.Loc
}

func (r *runClassStaticVarRef) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	className, err := r.className.Run(ctx)
	if err != nil {
		return nil, err
	}

	var class phpv.ZClass

	switch className.GetType() {
	case phpv.ZtObject:
		class = className.AsObject(ctx).GetClass()
	case phpv.ZtString:
		class, err = ctx.Global().GetClass(ctx, className.AsString(ctx), true)
	default:
		return nil, errors.New("invalid method receiver type: " + className.GetName().String())
	}

	if err != nil {
		return nil, err
	}

	// Walk the class hierarchy to find the static property (handles inheritance)
	zc := class.(*phpobj.ZClass)

	// Check visibility before looking up the property value
	if visErr := phpobj.CheckStaticPropVisibility(ctx, zc, r.varName); visErr != "" {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, visErr)
	}

	p, found, err := zc.FindStaticProp(ctx, r.varName)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Access to undeclared static property %s::$%s", class.GetName(), r.varName))
	}

	v := p.GetString(r.varName)
	// Return a detached snapshot so in-place mutations to the hash
	// entry don't retroactively change already-read values (PHP semantics).
	return phpv.NewZVal(v.Value()), nil
}

func (r *runClassStaticVarRef) WriteValue(ctx phpv.Context, value *phpv.ZVal) error {
	className, err := r.className.Run(ctx)
	if err != nil {
		return err
	}

	class, err := ctx.Global().GetClass(ctx, className.AsString(ctx), true)
	if err != nil {
		return err
	}

	// Walk the class hierarchy to find the static property (handles inheritance)
	zc := class.(*phpobj.ZClass)

	// PHP: unset() on static properties is always an error, even if the
	// property doesn't exist. Check for unset before visibility/existence.
	if value == nil {
		return phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Attempt to unset static property %s::$%s", class.GetName(), r.varName))
	}

	// Check visibility before writing
	if visErr := phpobj.CheckStaticPropVisibility(ctx, zc, r.varName); visErr != "" {
		return phpobj.ThrowError(ctx, phpobj.Error, visErr)
	}

	p, found, err := zc.FindStaticProp(ctx, r.varName)
	if err != nil {
		return err
	}
	if !found {
		return phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Access to undeclared static property %s::$%s", class.GetName(), r.varName))
	}

	// Track object references for static properties
	var oldObj interface {
		DecRef(phpv.Context) error
	}
	if old := p.GetString(r.varName); old != nil && old.GetType() == phpv.ZtObject {
		if obj, ok := old.Value().(interface {
			DecRef(phpv.Context) error
		}); ok {
			oldObj = obj
		}
	}
	if value != nil && value.GetType() == phpv.ZtObject {
		if obj, ok := value.Value().(interface{ IncRef() }); ok {
			obj.IncRef()
		}
	}

	err = p.SetString(r.varName, value)
	if err != nil {
		return err
	}
	if oldObj != nil {
		return oldObj.DecRef(ctx)
	}
	return nil
}

func (r *runClassStaticVarRef) Loc() *phpv.Loc {
	return r.l
}

func (r *runClassStaticVarRef) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s::$%s", r.className, r.varName)
	return err
}

// when classname::${expr} is used (dynamic static property)
type runClassStaticDynVarRef struct {
	className phpv.Runnable
	nameExpr  phpv.Runnable
	l         *phpv.Loc

	// PrepareWrite caching
	prepared   bool
	cachedName phpv.ZString
}

func (r *runClassStaticDynVarRef) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	className, err := r.className.Run(ctx)
	if err != nil {
		return nil, err
	}

	var class phpv.ZClass
	switch className.GetType() {
	case phpv.ZtObject:
		class = className.AsObject(ctx).GetClass()
	case phpv.ZtString:
		class, err = ctx.Global().GetClass(ctx, className.AsString(ctx), true)
	default:
		return nil, errors.New("invalid method receiver type: " + className.GetName().String())
	}
	if err != nil {
		return nil, err
	}

	// Use cached name from PrepareWrite if available (e.g. ??= memoization)
	var varName phpv.ZString
	if r.prepared && r.cachedName != "" {
		varName = r.cachedName
		// Don't consume the cache - it may be needed by WriteValue later
	} else {
		nameVal, err := r.nameExpr.Run(ctx)
		if err != nil {
			return nil, err
		}
		varName = phpv.ZString(nameVal.String())
	}

	zc := class.(*phpobj.ZClass)
	p, found, err := zc.FindStaticProp(ctx, varName)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Access to undeclared static property %s::$%s", class.GetName(), varName))
	}

	v := p.GetString(varName)
	return phpv.NewZVal(v.Value()), nil
}

func (r *runClassStaticDynVarRef) PrepareWrite(ctx phpv.Context) error {
	nameVal, err := r.nameExpr.Run(ctx)
	if err != nil {
		return err
	}
	r.prepared = true
	r.cachedName = phpv.ZString(nameVal.String())
	return nil
}

func (r *runClassStaticDynVarRef) WriteValue(ctx phpv.Context, value *phpv.ZVal) error {
	className, err := r.className.Run(ctx)
	if err != nil {
		return err
	}

	class, err := ctx.Global().GetClass(ctx, className.AsString(ctx), true)
	if err != nil {
		return err
	}

	var varName phpv.ZString
	if r.prepared {
		varName = r.cachedName
		r.prepared = false
	} else {
		nameVal, err := r.nameExpr.Run(ctx)
		if err != nil {
			return err
		}
		varName = phpv.ZString(nameVal.String())
	}

	zc := class.(*phpobj.ZClass)
	p, found, err := zc.FindStaticProp(ctx, varName)
	if err != nil {
		return err
	}
	if !found {
		return phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Access to undeclared static property %s::$%s", class.GetName(), varName))
	}

	// Track object references for static properties
	var oldObj interface {
		DecRef(phpv.Context) error
	}
	if old := p.GetString(varName); old != nil && old.GetType() == phpv.ZtObject {
		if obj, ok := old.Value().(interface {
			DecRef(phpv.Context) error
		}); ok {
			oldObj = obj
		}
	}
	if value != nil && value.GetType() == phpv.ZtObject {
		if obj, ok := value.Value().(interface{ IncRef() }); ok {
			obj.IncRef()
		}
	}

	err = p.SetString(varName, value)
	if err != nil {
		return err
	}
	if oldObj != nil {
		return oldObj.DecRef(ctx)
	}
	return nil
}

func (r *runClassStaticDynVarRef) Loc() *phpv.Loc {
	return r.l
}

func (r *runClassStaticDynVarRef) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s::${", r.className)
	if err != nil {
		return err
	}
	err = r.nameExpr.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{'}'})
	return err
}

// when classname::something is used
type runClassStaticObjRef struct {
	className phpv.Runnable
	objName   phpv.ZString
	l         *phpv.Loc
}

func (r *runClassStaticObjRef) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	className, err := r.className.Run(ctx)
	if err != nil {
		return nil, err
	}

	// Preserve the original class reference string for error messages
	// (e.g., "self" should stay "self" in "Undefined constant self::B")
	origClassName := className.AsString(ctx)

	var class phpv.ZClass

	switch className.GetType() {
	case phpv.ZtObject:
		class = className.AsObject(ctx).GetClass()
	case phpv.ZtString:
		class, err = ctx.Global().GetClass(ctx, className.AsString(ctx), true)
	default:
		return nil, errors.New("invalid method receiver type: " + className.GetName().String())
	}

	if err != nil {
		return nil, err
	}

	// For error messages: use the original reference (self/parent/static) when applicable,
	// otherwise use the resolved class name.
	errorClassName := class.GetName()
	switch origClassName.ToLower() {
	case "self", "parent", "static":
		errorClassName = origClassName
	}

	// Cannot access trait constants directly (PHP 8.2+)
	zclass := class.(*phpobj.ZClass)
	if zclass.Type == phpv.ZClassTypeTrait {
		// Only block if accessed via the trait name directly (not via a using class)
		if origClassName.ToLower() == zclass.Name.ToLower() {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot access trait constant %s::%s directly", zclass.Name, r.objName))
		}
	}

	cc, ok := zclass.Const[r.objName]
	if !ok {
		// Check implemented interfaces (for internal classes that don't inherit at registration time)
		for _, intf := range zclass.Implementations {
			if c, found := intf.Const[r.objName]; found {
				cc = c
				ok = true
				break
			}
		}
		// Check parent class
		if !ok && zclass.Extends != nil {
			if c, found := zclass.Extends.Const[r.objName]; found {
				cc = c
				ok = true
			}
		}
	}
	if !ok {
		return nil, phpobj.ThrowErrorAt(ctx, phpobj.Error, fmt.Sprintf("Undefined constant %s::%s", errorClassName, r.objName), r.l)
	}

	// Check visibility. compilingClass takes priority (used during attribute
	// argument evaluation where self:: should have full access).
	if cc.Modifiers.IsPrivate() {
		callerClass := ctx.Class()
		if cc := ctx.Global().GetCompilingClass(); cc != nil {
			callerClass = cc
		}
		if callerClass == nil || callerClass.GetName() != class.GetName() {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot access private constant %s::%s", class.GetName(), r.objName))
		}
	} else if cc.Modifiers.IsProtected() {
		callerClass := ctx.Class()
		if compilingCls := ctx.Global().GetCompilingClass(); compilingCls != nil {
			callerClass = compilingCls
		}
		if callerClass == nil || !callerClass.InstanceOf(class) && !class.InstanceOf(callerClass) {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot access protected constant %s::%s", class.GetName(), r.objName))
		}
	}

	// Check #[\Deprecated] attribute on the class constant
	for _, attr := range cc.Attributes {
		if attr.ClassName == "Deprecated" {
			// Skip if args are currently being resolved (prevents infinite recursion)
			if attr.Resolving {
				break
			}
			// Set compiling class context so self:: resolves correctly during attribute arg resolution
			prevCompiling := ctx.Global().GetCompilingClass()
			if zc, ok := class.(*phpobj.ZClass); ok {
				ctx.Global().SetCompilingClass(zc)
			}
			// Resolve lazy argument expressions (e.g., self::TEST)
			if err := ResolveAttrArgs(ctx, attr); err != nil {
				ctx.Global().SetCompilingClass(prevCompiling)
				return nil, err
			}
			ctx.Global().SetCompilingClass(prevCompiling)
			// Determine label: "Enum case" for enum cases, "Constant" otherwise
			label := "Constant"
			if zc, ok := class.(*phpobj.ZClass); ok && zc.Type == phpv.ZClassTypeEnum {
				// Check if this is an enum case (present in EnumCases list)
				for _, caseName := range zc.EnumCases {
					if caseName == r.objName {
						label = "Enum case"
						break
					}
				}
			}
			name := string(class.GetName()) + "::" + string(r.objName)
			msg := FormatDeprecatedMsg(label, name, attr)
			if err := ctx.UserDeprecated("%s", msg, logopt.NoFuncName(true)); err != nil {
				return nil, err
			}
			break
		}
	}

	v := cc.Value

	// Resolve CompileDelayed values (e.g., constants referencing other constants)
	if cd, isCD := v.(*phpv.CompileDelayed); isCD {
		// Detect circular references: if this constant is already being resolved,
		// we have a cycle. Find which constant triggered the cycle by looking for
		// the other constant(s) that are also in Resolving state.
		if cc.Resolving {
			// The self-referencing constant is the one that's currently being resolved
			// and depends (directly or indirectly) on itself. Find it by scanning
			// all resolving constants - the last-started one (not the one we're
			// looking up) is the self-referencing one.
			selfRefName := r.objName
			zc := class.(*phpobj.ZClass)
			for _, name := range zc.ConstOrder {
				if c := zc.Const[name]; c != nil && c.Resolving && name != r.objName {
					selfRefName = name
				}
			}
			return nil, phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Cannot declare self-referencing constant %s::%s", errorClassName, selfRefName))
		}
		cc.Resolving = true
		// Set compiling class so self:: works during constant resolution.
		// Save and restore previous value to avoid disrupting outer scope.
		prevCompiling := ctx.Global().GetCompilingClass()
		ctx.Global().SetCompilingClass(class.(*phpobj.ZClass))
		resolved, err := cd.Run(ctx)
		ctx.Global().SetCompilingClass(prevCompiling)
		cc.Resolving = false
		if err != nil {
			// Add a synthetic [constant expression] frame to match PHP behavior,
			// but only if one hasn't been added yet (avoid duplicates in circular refs).
			if ex, ok := err.(*phperr.PhpThrow); ok {
				hasFrame := false
				if opaque := ex.Obj.GetOpaque(ex.Obj.GetClass()); opaque != nil {
					if trace, ok2 := opaque.([]*phpv.StackTraceEntry); ok2 {
						for _, e := range trace {
							if e.FuncName == "[constant expression]" {
								hasFrame = true
								break
							}
						}
					}
				}
				if !hasFrame {
					phpobj.AddConstantExpressionFrame(ex, ctx)
				}
			}
			return nil, err
		}
		cc.Value = resolved.Value()
		return resolved, nil
	}

	// For enum cases, check if the enum has a stored error (e.g. duplicate values).
	// This must be checked AFTER resolution since enum cases are resolved eagerly.
	if zc, ok := class.(*phpobj.ZClass); ok && zc.EnumError != nil {
		// Only throw for enum case constants, not regular constants
		for _, caseName := range zc.EnumCases {
			if caseName == r.objName {
				return nil, zc.EnumError
			}
		}
	}

	return v.ZVal(), nil
}

func (r *runClassStaticObjRef) Call(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	className, err := r.className.Run(ctx)
	if err != nil {
		return nil, err
	}

	ctx = ctx.Parent(1) // go back one level
	// first, fetch class object
	class, err := ctx.Global().GetClass(ctx, className.AsString(ctx), true)
	if err != nil {
		return nil, err
	}

	method, ok := class.GetMethod(r.objName.ToLower())
	if !ok {
		// When called from an instance context ($this is available), prefer __call
		// over __callStatic, matching PHP behavior for self::, static::, ClassName::
		// calls within instance methods.
		if ctx.This() != nil {
			method, ok = class.GetMethod("__call")
			if ok {
				a := phpv.NewZArray()
				callArgs := []*phpv.ZVal{r.objName.ZVal(), a.ZVal()}
				for _, sub := range args {
					a.OffsetSet(ctx, nil, sub)
				}
				return ctx.CallZVal(ctx, method.Method, callArgs, ctx.This())
			}
		}
		method, ok = class.GetMethod("__callStatic")
		if ok {
			a := phpv.NewZArray()
			callArgs := []*phpv.ZVal{r.objName.ZVal(), a.ZVal()}

			for _, sub := range args {
				a.OffsetSet(ctx, nil, sub)
			}

			return ctx.CallZVal(ctx, method.Method, callArgs, ctx.This())
		}
		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Call to undefined method %s::%s()", r.className, r.objName))
	}

	return ctx.CallZVal(ctx, method.Method, args, ctx.This())
}

func (r *runClassStaticObjRef) Loc() *phpv.Loc {
	return r.l
}

func (r *runClassStaticObjRef) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s::%s", r.className, r.objName)
	return err
}

// runClassDynConst implements C::{expr} — dynamic class constant fetch (PHP 8.3+).
// The expression is evaluated at runtime to produce the constant name.
type runClassDynConst struct {
	className phpv.Runnable
	nameExpr  phpv.Runnable
	l         *phpv.Loc
}

func (r *runClassDynConst) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	classVal, err := r.className.Run(ctx)
	if err != nil {
		return nil, err
	}

	nameVal, err := r.nameExpr.Run(ctx)
	if err != nil {
		return nil, err
	}

	// The constant name must be a string
	if nameVal.GetType() != phpv.ZtString {
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Cannot use value of type %s as class constant name", phpv.ZValTypeName(nameVal)))
	}

	constName := nameVal.AsString(ctx)

	var class phpv.ZClass
	switch classVal.GetType() {
	case phpv.ZtObject:
		class = classVal.AsObject(ctx).GetClass()
	case phpv.ZtString:
		class, err = ctx.Global().GetClass(ctx, classVal.AsString(ctx), true)
		if err != nil {
			return nil, err
		}
	default:
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("Cannot use %s as class name", phpv.ZValTypeName(classVal)))
	}

	// Handle "class" as special keyword
	if strings.EqualFold(string(constName), "class") {
		return phpv.ZString(class.GetName()).ZVal(), nil
	}

	// Look up the constant on the class
	zclass, ok := class.(*phpobj.ZClass)
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Undefined constant %s::%s", class.GetName(), constName))
	}

	cc, found := zclass.Const[constName]
	if !found {
		// Check interfaces
		for _, intf := range zclass.Implementations {
			if c, f := intf.Const[constName]; f {
				cc = c
				found = true
				break
			}
		}
		// Check parent
		if !found && zclass.Extends != nil {
			if c, f := zclass.Extends.Const[constName]; f {
				cc = c
				found = true
			}
		}
	}
	if !found {
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Undefined constant %s::%s", class.GetName(), constName))
	}

	// Check visibility
	if cc.Modifiers.IsPrivate() {
		callerClass := ctx.Class()
		if callerClass == nil || callerClass.GetName() != class.GetName() {
			return nil, phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Cannot access private constant %s::%s", class.GetName(), constName))
		}
	} else if cc.Modifiers.IsProtected() {
		callerClass := ctx.Class()
		if callerClass == nil || (!callerClass.InstanceOf(class) && !class.InstanceOf(callerClass)) {
			return nil, phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Cannot access protected constant %s::%s", class.GetName(), constName))
		}
	}

	v := cc.Value
	// Resolve CompileDelayed
	if cd, isCD := v.(*phpv.CompileDelayed); isCD {
		if cc.Resolving {
			return nil, phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Cannot declare self-referencing constant %s::%s", class.GetName(), constName))
		}
		cc.Resolving = true
		prevCompiling := ctx.Global().GetCompilingClass()
		ctx.Global().SetCompilingClass(zclass)
		resolved, err := cd.Run(ctx)
		ctx.Global().SetCompilingClass(prevCompiling)
		cc.Resolving = false
		if err != nil {
			return nil, err
		}
		cc.Value = resolved.Value()
		return resolved, nil
	}

	return v.ZVal(), nil
}

func (r *runClassDynConst) Dump(w io.Writer) error {
	if err := r.className.Dump(w); err != nil {
		return err
	}
	if _, err := w.Write([]byte("::")); err != nil {
		return err
	}
	if _, err := w.Write([]byte{'{'}); err != nil {
		return err
	}
	if err := r.nameExpr.Dump(w); err != nil {
		return err
	}
	_, err := w.Write([]byte{'}'})
	return err
}

// runClassNameOf implements $var::class and ClassName::class
type runClassNameOf struct {
	className phpv.Runnable
	l         *phpv.Loc
}

func (r *runClassNameOf) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	v, err := r.className.Run(ctx)
	if err != nil {
		return nil, err
	}

	switch v.GetType() {
	case phpv.ZtObject:
		return phpv.ZString(v.AsObject(ctx).GetClass().GetName()).ZVal(), nil
	case phpv.ZtString:
		// self::class, parent::class, static::class must resolve at runtime
		name := v.AsString(ctx)
		switch strings.ToLower(string(name)) {
		case "self":
			// Check compilingClass first (set during attribute argument evaluation)
			if cc := ctx.Global().GetCompilingClass(); cc != nil {
				return phpv.ZString(cc.GetName()).ZVal(), nil
			}
			cls := ctx.Class()
			if cls == nil {
				// Differentiate: top-level (no function context) says "in the global scope",
				// inside a function it says "when no class scope is active"
				if ctx.Func() == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot use \"self\" in the global scope")
				}
				return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot use \"self\" when no class scope is active")
			}
			return phpv.ZString(cls.GetName()).ZVal(), nil
		case "parent":
			cls := ctx.Class()
			if cls == nil {
				if ctx.Func() == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot use \"parent\" in the global scope")
				}
				return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot use \"parent\" when no class scope is active")
			}
			parent := cls.GetParent()
			if parent == nil {
				return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot use \"parent\" when current class scope has no parent")
			}
			return phpv.ZString(parent.GetName()).ZVal(), nil
		case "static":
			// Late static binding: resolve to the actual called class.
			if this := ctx.This(); this != nil {
				if uw, ok := this.(interface{ Unwrap() phpv.ZObject }); ok {
					return phpv.ZString(uw.Unwrap().GetClass().GetName()).ZVal(), nil
				}
				return phpv.ZString(this.GetClass().GetName()).ZVal(), nil
			}
			// Check for CalledClass (late static binding in static context)
			if fc := ctx.Func(); fc != nil {
				if cc, ok := fc.(interface{ CalledClass() phpv.ZClass }); ok {
					if called := cc.CalledClass(); called != nil {
						return phpv.ZString(called.GetName()).ZVal(), nil
					}
				}
			}
			cls := ctx.Class()
			if cls == nil {
				if ctx.Func() == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot use \"static\" in the global scope")
				}
				return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot use \"static\" when no class scope is active")
			}
			return phpv.ZString(cls.GetName()).ZVal(), nil
		}
		// ClassName::class resolves to the fully-qualified class name
		return v, nil
	default:
		typeName := v.GetType().TypeName()
		if typeName == "null" {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "Cannot use \"::class\" on null")
		}
		// For non-string, non-object types, PHP raises a fatal error.
		// PHP uses "Illegal class name" for simple literal values, but
		// "Cannot use ::class on TYPE" for computed expressions.
		// We use the latter as it's the more descriptive form.
		phpErr := &phpv.PhpError{
			Err:  fmt.Errorf("Illegal class name"),
			Code: phpv.E_ERROR,
			Loc:  r.l,
		}
		ctx.Global().LogError(phpErr)
		return nil, phpv.ExitError(255)
	}
}

func (r *runClassNameOf) Loc() *phpv.Loc {
	return r.l
}

func (r *runClassNameOf) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s::class", r.className)
	return err
}
