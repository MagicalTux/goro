package compiler

import (
	"errors"
	"fmt"
	"io"

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

	nameVal, err := r.nameExpr.Run(ctx)
	if err != nil {
		return nil, err
	}
	varName := phpv.ZString(nameVal.String())

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

	cc, ok := class.(*phpobj.ZClass).Const[r.objName]
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Undefined constant %s::%s", class.GetName(), r.objName))
	}

	// Check visibility
	if cc.Modifiers.IsPrivate() {
		callerClass := ctx.Class()
		if callerClass == nil || callerClass.GetName() != class.GetName() {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot access private constant %s::%s", class.GetName(), r.objName))
		}
	} else if cc.Modifiers.IsProtected() {
		callerClass := ctx.Class()
		if callerClass == nil || !callerClass.InstanceOf(class) && !class.InstanceOf(callerClass) {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot access protected constant %s::%s", class.GetName(), r.objName))
		}
	}

	v := cc.Value

	// Resolve CompileDelayed values (e.g., constants referencing other constants)
	if cd, isCD := v.(*phpv.CompileDelayed); isCD {
		resolved, err := cd.Run(ctx)
		if err != nil {
			return nil, err
		}
		cc.Value = resolved.Value()
		return resolved, nil
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
		method, ok = class.GetMethod("__callStatic")
		if ok {
			// found __call method
			a := phpv.NewZArray()
			callArgs := []*phpv.ZVal{r.objName.ZVal(), a.ZVal()}

			for _, sub := range args {
				a.OffsetSet(ctx, nil, sub)
			}

			return ctx.CallZVal(ctx, method.Method, callArgs, ctx.This())
		}
		return nil, ctx.Errorf("Call to undefined method %s::%s()", r.className, r.objName)
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
		// ClassName::class resolves to the fully-qualified class name
		return v, nil
	default:
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("Cannot use ::class on value of type %s", v.GetType().TypeName()))
	}
}

func (r *runClassNameOf) Loc() *phpv.Loc {
	return r.l
}

func (r *runClassNameOf) Dump(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%s::class", r.className)
	return err
}
