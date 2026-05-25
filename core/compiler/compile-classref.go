package compiler

import (
	"fmt"
	"io"
	"strings"

	"github.com/KarpelesLab/goro/core/logopt"
	"github.com/KarpelesLab/goro/core/phperr"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
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
	return EvalClassStaticVarRead(ctx, className, r.varName, r.l)
}

// EvalClassStaticVarRead implements `Cls::$prop` reads with the
// pre-evaluated class source. Both AST runner and VM's
// OP_CLASS_STATIC_GET share this. Visibility, hierarchy walk, and the
// "detached snapshot" return live here.
func EvalClassStaticVarRead(ctx phpv.Context, className *phpv.ZVal, varName phpv.ZString, l *phpv.Loc) (*phpv.ZVal, error) {
	var class phpv.ZClass
	var err error
	switch className.GetType() {
	case phpv.ZtObject:
		class = className.AsObject(ctx).GetClass()
	case phpv.ZtString:
		class, err = ctx.Global().GetClass(ctx, className.AsString(ctx), true)
	default:
		phpErr := &phpv.PhpError{
			Err:  fmt.Errorf("Illegal class name"),
			Code: phpv.E_ERROR,
			Loc:  l,
		}
		ctx.Global().LogError(phpErr)
		return nil, phpv.ExitError(255)
	}
	if err != nil {
		return nil, err
	}
	zc := class.(*phpobj.ZClass)
	if visErr := phpobj.CheckStaticPropVisibility(ctx, zc, varName); visErr != "" {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, visErr)
	}
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

func (r *runClassStaticVarRef) WriteValue(ctx phpv.Context, value *phpv.ZVal) error {
	className, err := r.className.Run(ctx)
	if err != nil {
		return err
	}
	return AssignClassStaticProp(ctx, className, r.varName, value)
}

// AssignClassStaticProp writes `value` into the static property
// named `varName` on the class identified by `className` (a resolved
// class-source value: a string, "self", "parent", "static", etc.).
// Mirrors runClassStaticVarRef.WriteValue: LSB-aware class resolution
// via ctx.Global().GetClass, both read-side and write-side
// (PHP 8.4 asymmetric) visibility checks, typed-property enforcement
// (strict + weak coercion), and IncRef/DecRef bookkeeping for object
// values. Used by both the AST runner and the VM's
// OP_STATIC_PROP_SET.
func AssignClassStaticProp(ctx phpv.Context, className *phpv.ZVal, varName phpv.ZString, value *phpv.ZVal) error {
	class, err := ctx.Global().GetClass(ctx, className.AsString(ctx), true)
	if err != nil {
		return err
	}
	zc := class.(*phpobj.ZClass)

	// PHP: unset() on static properties is always an error, even if the
	// property doesn't exist. Check for unset before visibility/existence.
	if value == nil {
		return phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Attempt to unset static property %s::$%s", class.GetName(), varName))
	}

	// Check visibility before writing
	if visErr := phpobj.CheckStaticPropVisibility(ctx, zc, varName); visErr != "" {
		return phpobj.ThrowError(ctx, phpobj.Error, visErr)
	}

	// Check asymmetric set visibility for static properties (PHP 8.4)
	if visErr := phpobj.CheckStaticPropSetVisibility(ctx, zc, varName); visErr != "" {
		return phpobj.ThrowError(ctx, phpobj.Error, visErr)
	}

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

	// Enforce typed property type checking for static properties
	if prop := zc.FindDeclaredProp(varName); prop != nil && prop.TypeHint != nil {
		hint := prop.TypeHint
		if value.IsNull() {
			if !hint.IsNullable() {
				return phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("Cannot assign null to property %s::$%s of type %s",
						class.GetName(), varName, hint.String()))
			}
		} else {
			isStrict := ctx.Global().GetStrictTypes()
			if isStrict {
				if !hint.CheckStrict(ctx, value) {
					typeName := phpv.ZValTypeNameDetailed(value)
					return phpobj.ThrowError(ctx, phpobj.TypeError,
						fmt.Sprintf("Cannot assign %s to property %s::$%s of type %s",
							typeName, class.GetName(), varName, hint.String()))
				}
				// int->float widening in strict mode
				if hint.Type() == phpv.ZtFloat && value.GetType() == phpv.ZtInt && len(hint.Union) == 0 && len(hint.Intersection) == 0 {
					if coerced, err2 := value.Value().AsVal(ctx, phpv.ZtFloat); err2 == nil && coerced != nil {
						value = coerced.ZVal()
					}
				}
			} else {
				if !hint.Check(ctx, value) {
					typeName := phpv.ZValTypeNameDetailed(value)
					return phpobj.ThrowError(ctx, phpobj.TypeError,
						fmt.Sprintf("Cannot assign %s to property %s::$%s of type %s",
							typeName, class.GetName(), varName, hint.String()))
				}
				// Coerce scalar types in weak mode
				hintType := hint.Type()
				valType := value.GetType()
				if hintType != phpv.ZtMixed && hintType != phpv.ZtObject && valType != hintType && len(hint.Union) == 0 && len(hint.Intersection) == 0 {
					if hintType == phpv.ZtInt && valType == phpv.ZtFloat {
						v, err2 := phpv.FloatToIntImplicit(ctx, value.Value().(phpv.ZFloat))
						if err2 != nil {
							return err2
						}
						value = v.ZVal()
					} else if coerced, err2 := value.Value().AsVal(ctx, hintType); err2 == nil && coerced != nil {
						value = coerced.ZVal()
					}
				}
			}
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
	// Use cached name from PrepareWrite if available (e.g. ??= memoization)
	var varName phpv.ZString
	if r.prepared && r.cachedName != "" {
		varName = r.cachedName
	} else {
		nameVal, err := r.nameExpr.Run(ctx)
		if err != nil {
			return nil, err
		}
		varName = phpv.ZString(nameVal.String())
	}
	return EvalClassStaticDynVarRead(ctx, className, varName, r.l)
}

// EvalClassStaticDynVarRead implements `Cls::${$name}` reads. No
// visibility check (the dyn-name form bypasses it; matches the
// original runClassStaticDynVarRef.Run).
func EvalClassStaticDynVarRead(ctx phpv.Context, className *phpv.ZVal, varName phpv.ZString, l *phpv.Loc) (*phpv.ZVal, error) {
	var class phpv.ZClass
	var err error
	switch className.GetType() {
	case phpv.ZtObject:
		class = className.AsObject(ctx).GetClass()
	case phpv.ZtString:
		class, err = ctx.Global().GetClass(ctx, className.AsString(ctx), true)
	default:
		phpErr := &phpv.PhpError{
			Err:  fmt.Errorf("Illegal class name"),
			Code: phpv.E_ERROR,
			Loc:  l,
		}
		ctx.Global().LogError(phpErr)
		return nil, phpv.ExitError(255)
	}
	if err != nil {
		return nil, err
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
	return AssignClassStaticDynProp(ctx, className, varName, value)
}

// AssignClassStaticDynProp writes `value` to `Cls::${$varName}` with
// the pre-evaluated class source. Used by both the AST runner (via
// runClassStaticDynVarRef.WriteValue) and the VM's OP_STATIC_PROP_DYN_SET.
//
// Unlike AssignClassStaticProp, the dyn-name form skips visibility,
// asymmetric-visibility, and type-hint checks — matching the original
// AST runClassStaticDynVarRef.WriteValue semantics.
func AssignClassStaticDynProp(ctx phpv.Context, className *phpv.ZVal, varName phpv.ZString, value *phpv.ZVal) error {
	class, err := ctx.Global().GetClass(ctx, className.AsString(ctx), true)
	if err != nil {
		return err
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

	if err := p.SetString(varName, value); err != nil {
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
	return EvalClassStaticObjRef(ctx, className, r.objName, r.l)
}

// EvalClassStaticObjRef implements `Cls::IDENT` literal class-const /
// enum-case fetch with pre-evaluated class source. Includes the full
// machinery from runClassStaticObjRef.Run: trait const blocking,
// visibility, interface/parent walking, attribute deprecation,
// CompileDelayed resolution with [constant expression] frame
// decoration, enum errors, and typed class-const coercion.
func EvalClassStaticObjRef(ctx phpv.Context, className *phpv.ZVal, objName phpv.ZString, l *phpv.Loc) (*phpv.ZVal, error) {
	origClassName := className.AsString(ctx)
	var err error

	var class phpv.ZClass

	switch className.GetType() {
	case phpv.ZtObject:
		class = className.AsObject(ctx).GetClass()
	case phpv.ZtString:
		class, err = ctx.Global().GetClass(ctx, className.AsString(ctx), true)
	default:
		phpErr := &phpv.PhpError{
			Err:  fmt.Errorf("Illegal class name"),
			Code: phpv.E_ERROR,
			Loc:  l,
		}
		ctx.Global().LogError(phpErr)
		return nil, phpv.ExitError(255)
	}

	if err != nil {
		// If the error is a thrown exception (e.g. "Class not found") and we
		// have a compile-time location (l), update the exception's file/line
		// to point to where the class reference appears in source code,
		// not where the constant expression is being evaluated from (GH-7771).
		if l != nil {
			if ex, ok := err.(*phperr.PhpThrow); ok {
				if ex.Loc == nil || (ex.Loc.Filename != l.Filename || ex.Loc.Line != l.Line) {
					ex.Loc = l
					// Also update the exception object's file/line properties
					if ex.Obj != nil {
						ex.Obj.HashTable().SetString("file", phpv.ZString(l.Filename).ZVal())
						ex.Obj.HashTable().SetString("line", phpv.ZInt(l.Line).ZVal())
					}
				}
			}
		}
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
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot access trait constant %s::%s directly", zclass.Name, objName))
		}
	}

	cc, ok := zclass.Const[objName]
	// Track the class that actually declares this constant for deprecation messages.
	deprecClassName := class.GetName()
	if !ok {
		// Check implemented interfaces (for internal classes that don't inherit at registration time)
		for _, intf := range zclass.Implementations {
			if c, found := intf.Const[objName]; found {
				cc = c
				ok = true
				deprecClassName = intf.GetName()
				break
			}
		}
		// Check parent class
		if !ok && zclass.Extends != nil {
			if c, found := zclass.Extends.Const[objName]; found {
				cc = c
				ok = true
				deprecClassName = zclass.Extends.GetName()
			}
		}
	}
	if !ok {
		return nil, phpobj.ThrowErrorAt(ctx, phpobj.Error, fmt.Sprintf("Undefined constant %s::%s", errorClassName, objName), l)
	}

	// Check visibility. compilingClass takes priority (used during attribute
	// argument evaluation where self:: should have full access).
	if cc.Modifiers.IsPrivate() {
		callerClass := ctx.Class()
		if cc := ctx.Global().GetCompilingClass(); cc != nil {
			callerClass = cc
		}
		if callerClass == nil || callerClass.GetName() != class.GetName() {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot access private constant %s::%s", class.GetName(), objName))
		}
	} else if cc.Modifiers.IsProtected() {
		callerClass := ctx.Class()
		if compilingCls := ctx.Global().GetCompilingClass(); compilingCls != nil {
			callerClass = compilingCls
		}
		if callerClass == nil || !callerClass.InstanceOf(class) && !class.InstanceOf(callerClass) {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot access protected constant %s::%s", class.GetName(), objName))
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
					if caseName == objName {
						label = "Enum case"
						break
					}
				}
			}
			name := string(deprecClassName) + "::" + string(objName)
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
			selfRefName := objName
			zc := class.(*phpobj.ZClass)
			for _, name := range zc.ConstOrder {
				if c := zc.Const[name]; c != nil && c.Resolving && name != objName {
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
		// Call cd.V.Run directly (bypassing CompileDelayed.resolving guard)
		// because re-entrant access to this constant is legitimate when
		// autoloading satisfies the dependency (see GH-10709). The
		// cc.Resolving flag above guards against true circular references.
		resolved, err := cd.V.Run(ctx)
		ctx.Global().SetCompilingClass(prevCompiling)
		cc.Resolving = false
		if err != nil {
			// Add a synthetic [constant expression] frame to match PHP
			// behavior. PHP wraps *class*-scope const lookup failures
			// (self::X, ClassName::Y) but leaves bare-name "Undefined
			// constant \"NAME\"" failures alone — so only decorate when
			// the error text mentions a :: qualifier (bug41633_2 vs
			// enum/update-class-constant-failure).
			if ex, ok := err.(*phperr.PhpThrow); ok {
				msg := ""
				if msgVal := ex.Obj.HashTable().GetString("message"); msgVal != nil {
					msg = msgVal.String()
				}
				wrap := !strings.HasPrefix(msg, "Undefined constant") || strings.Contains(msg, "::")
				if wrap {
					hasFrame := false
					if trace, ok2 := ex.Obj.GetOpaque(ex.Obj.GetClass()).([]*phpv.StackTraceEntry); ok2 {
						for _, e := range trace {
							if e.FuncName == "[constant expression]" {
								hasFrame = true
								break
							}
						}
					}
					if !hasFrame {
						phpobj.AddConstantExpressionFrame(ex, ctx)
					}
				}
			}
			return nil, err
		}
		// Coerce and validate value against typed class constant type hint
		if cc.TypeHint != nil {
			coerced, cerr := coerceConstToTypeHint(ctx, resolved, cc.TypeHint)
			if cerr != nil {
				typeName := phpv.ZValTypeName(resolved)
				return nil, phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("Cannot assign %s to class constant %s::%s of type %s",
						typeName, errorClassName, objName, cc.TypeHint.String()))
			}
			resolved = coerced
		}
		cc.Value = resolved.Value()
		return resolved, nil
	}

	// For enum cases, check if the enum has a stored error (e.g. duplicate values).
	// This must be checked AFTER resolution since enum cases are resolved eagerly.
	if zc, ok := class.(*phpobj.ZClass); ok && zc.EnumError != nil {
		// Only throw for enum case constants, not regular constants
		for _, caseName := range zc.EnumCases {
			if caseName == objName {
				return nil, zc.EnumError
			}
		}
	}

	// Validate and coerce value against type hint for already-resolved constants
	result := v.ZVal()
	if cc.TypeHint != nil {
		coerced, cerr := coerceConstToTypeHint(ctx, result, cc.TypeHint)
		if cerr != nil {
			typeName := phpv.ZValTypeName(result)
			return nil, phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Cannot assign %s to class constant %s::%s of type %s",
					typeName, errorClassName, objName, cc.TypeHint.String()))
		}
		result = coerced
	}

	return result, nil
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
	return EvalClassDynConst(ctx, classVal, nameVal)
}

// EvalClassDynConst implements `Cls::CONST` / `$obj::CONST` / `Cls::{$name}`.
// Both AST runner and VM's OP_CLASS_DYN_CONST share this. Visibility,
// interface/parent walking, CompileDelayed resolution and `::class`
// special-casing all live here.
func EvalClassDynConst(ctx phpv.Context, classVal, nameVal *phpv.ZVal) (*phpv.ZVal, error) {
	if nameVal.GetType() != phpv.ZtString {
		return nil, phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Cannot use value of type %s as class constant name", phpv.ZValTypeName(nameVal)))
	}
	constName := nameVal.AsString(ctx)
	var err error

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
		// For anonymous classes, return the full internal name (including null byte),
		// matching PHP behavior where $anon::class includes the null byte separator.
		if zc, ok2 := class.(*phpobj.ZClass); ok2 {
			return phpv.ZString(zc.Name).ZVal(), nil
		}
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
		// Call cd.V.Run directly for consistency with runClassStaticObjRef (GH-10709)
		resolved, err := cd.V.Run(ctx)
		ctx.Global().SetCompilingClass(prevCompiling)
		cc.Resolving = false
		if err != nil {
			// Mirror the [constant expression] decoration from
			// runClassStaticObjRef.Run: wrap every const-eval error except
			// the bare-name "Undefined constant \"NAME\"" kind, which PHP
			// doesn't wrap.
			if ex, ok := err.(*phperr.PhpThrow); ok {
				msg := ""
				if msgVal := ex.Obj.HashTable().GetString("message"); msgVal != nil {
					msg = msgVal.String()
				}
				wrap := !strings.HasPrefix(msg, "Undefined constant") || strings.Contains(msg, "::")
				if wrap {
					hasFrame := false
					if trace, ok2 := ex.Obj.GetOpaque(ex.Obj.GetClass()).([]*phpv.StackTraceEntry); ok2 {
						for _, e := range trace {
							if e.FuncName == "[constant expression]" {
								hasFrame = true
								break
							}
						}
					}
					if !hasFrame {
						phpobj.AddConstantExpressionFrame(ex, ctx)
					}
				}
			}
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
	isLiteral := false
	switch cn := r.className.(type) {
	case *runZVal:
		_ = cn
		isLiteral = true
	case *runParentheses:
		if _, ok := cn.r.(*runZVal); ok {
			isLiteral = true
		}
	}
	return EvalClassNameOf(ctx, v, isLiteral, r.l)
}

// EvalClassNameOf implements the `Cls::class` operator with the
// pre-evaluated class-source value in v. isLiteral indicates whether
// the class-source expression was a compile-time literal — used only
// to pick between two error messages ("Illegal class name" vs
// "Cannot use \"::class\" on …"). Both AST runner and VM share this
// helper.
func EvalClassNameOf(ctx phpv.Context, v *phpv.ZVal, isLiteral bool, l *phpv.Loc) (*phpv.ZVal, error) {
	switch v.GetType() {
	case phpv.ZtObject:
		// For anonymous classes, return the full internal name (including null byte),
		// matching PHP behavior where $anon::class includes the null byte separator.
		objClass := v.AsObject(ctx).GetClass()
		if zc, ok := objClass.(*phpobj.ZClass); ok {
			return phpv.ZString(zc.Name).ZVal(), nil
		}
		return phpv.ZString(objClass.GetName()).ZVal(), nil
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
				return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot use \"self\" in the global scope")
			}
			return phpv.ZString(cls.GetName()).ZVal(), nil
		case "parent":
			cls := ctx.Class()
			if cls == nil {
				if ctx.Func() == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot use \"parent\" in the global scope")
				}
				return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot use \"parent\" in the global scope")
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
				return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot use \"static\" in the global scope")
			}
			return phpv.ZString(cls.GetName()).ZVal(), nil
		}
		// ClassName::class resolves to the fully-qualified class name
		return v, nil
	default:
		typeName := v.GetType().TypeName()
		if typeName == "bool" {
			if v.AsBool(ctx) {
				typeName = "true"
			} else {
				typeName = "false"
			}
		}
		if typeName == "null" {
			// null::class throws TypeError (catchable)
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("Cannot use \"::class\" on %s", typeName))
		}
		// Other non-object/non-string types produce a fatal error.
		// If the expression is a literal (compile-time constant), use "Illegal class name"
		// as PHP does at compile time. Otherwise, use the runtime error format.
		var errMsg string
		if isLiteral {
			errMsg = "Illegal class name"
		} else {
			errMsg = fmt.Sprintf("Cannot use \"::class\" on %s", typeName)
		}
		phpErr := &phpv.PhpError{
			Err:  fmt.Errorf("%s", errMsg),
			Code: phpv.E_ERROR,
			Loc:  l,
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

// coerceConstToTypeHint coerces a resolved class constant value to match its type hint.
// For example, int(3) with a float type hint becomes float(3).
// It also validates that the value matches the type hint and returns an error if not.
func coerceConstToTypeHint(ctx phpv.Context, val *phpv.ZVal, th *phpv.TypeHint) (*phpv.ZVal, error) {
	if th == nil || val == nil {
		return val, nil
	}
	v := val.Value()
	vt := v.GetType()

	// Direct float type: coerce int to float
	if th.Type() == phpv.ZtFloat && len(th.Union) == 0 && len(th.Intersection) == 0 {
		if vt == phpv.ZtInt {
			f, _ := v.AsVal(ctx, phpv.ZtFloat)
			return f.ZVal(), nil
		}
		if vt == phpv.ZtFloat {
			return val, nil
		}
		// Type mismatch
		return nil, fmt.Errorf("type mismatch for float constant: got %s", vt.TypeName())
	}

	// Union types: check if any member is float and value is int
	if len(th.Union) > 0 {
		for _, u := range th.Union {
			if u.Type() == phpv.ZtFloat && vt == phpv.ZtInt {
				// Only coerce if int is not also in the union
				hasInt := false
				for _, u2 := range th.Union {
					if u2.Type() == phpv.ZtInt {
						hasInt = true
						break
					}
				}
				if !hasInt {
					f, _ := v.AsVal(ctx, phpv.ZtFloat)
					return f.ZVal(), nil
				}
			}
		}
	}

	// Validate that the value matches the type hint
	if !constValueMatchesTypeHint(ctx, val, th) {
		typeName := phpv.ZValTypeName(val)
		return nil, fmt.Errorf("type mismatch: got %s", typeName)
	}

	return val, nil
}

// constValueMatchesTypeHint checks if a constant value matches a type hint (strict check).
func constValueMatchesTypeHint(ctx phpv.Context, val *phpv.ZVal, th *phpv.TypeHint) bool {
	if th == nil {
		return true
	}

	// Nullable check
	if th.IsNullable() && val.IsNull() {
		return true
	}

	// Union type: any member must match
	if len(th.Union) > 0 {
		for _, u := range th.Union {
			if constValueMatchesTypeHint(ctx, val, u) {
				return true
			}
		}
		return false
	}

	// Intersection type: all must match
	if len(th.Intersection) > 0 {
		for _, part := range th.Intersection {
			if !constValueMatchesTypeHint(ctx, val, part) {
				return false
			}
		}
		return true
	}

	vt := val.GetType()

	// mixed accepts anything
	if th.Type() == phpv.ZtMixed {
		return true
	}

	// null type
	if th.Type() == phpv.ZtNull {
		return val.IsNull()
	}

	// Handle false/true standalone types
	if th.Type() == phpv.ZtBool && th.ClassName() == "false" {
		return vt == phpv.ZtBool && !bool(val.Value().(phpv.ZBool))
	}
	if th.Type() == phpv.ZtBool && th.ClassName() == "true" {
		return vt == phpv.ZtBool && bool(val.Value().(phpv.ZBool))
	}

	// Object type: check class match
	if th.Type() == phpv.ZtObject {
		if th.ClassName() == "" {
			return vt == phpv.ZtObject
		}
		if vt != phpv.ZtObject {
			return false
		}
		// Check class name match
		if obj, ok := val.Value().(phpv.ZObject); ok {
			return phpv.ClassNameMatch(obj.GetClass(), th.ClassName(), ctx)
		}
		return false
	}

	// Scalar types: strict matching (no coercion for constants)
	switch th.Type() {
	case phpv.ZtInt:
		return vt == phpv.ZtInt
	case phpv.ZtFloat:
		return vt == phpv.ZtFloat || vt == phpv.ZtInt // int→float is allowed
	case phpv.ZtString:
		return vt == phpv.ZtString
	case phpv.ZtBool:
		return vt == phpv.ZtBool
	case phpv.ZtArray:
		return vt == phpv.ZtArray
	}

	return vt == th.Type()
}
