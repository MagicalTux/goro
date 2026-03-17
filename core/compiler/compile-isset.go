package compiler

import (
	"errors"
	"io"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

type runnableIsset struct {
	args phpv.Runnables
	l    *phpv.Loc
}

func (r *runnableIsset) Dump(w io.Writer) error {
	_, err := w.Write([]byte("isset("))
	if err != nil {
		return err
	}
	err = r.args.DumpWith(w, []byte{','})
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{')'})
	return err
}

func (r *runnableIsset) Run(ctx phpv.Context) (l *phpv.ZVal, err error) {
	for _, v := range r.args {
		exists, err := checkExistence(ctx, v, false)
		if !exists || err != nil {
			return phpv.ZBool(false).ZVal(), err
		}
	}
	return phpv.ZBool(true).ZVal(), nil
}

func compileIsset(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	var err error
	is := &runnableIsset{l: i.Loc()}
	is.args, err = compileFuncPassedArgs(c)
	return is, err
}

type runnableEmpty struct {
	arg phpv.Runnable
	l   *phpv.Loc
}

func (r *runnableEmpty) Dump(w io.Writer) error {
	_, err := w.Write([]byte("empty("))
	if err != nil {
		return err
	}
	err = r.arg.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{')'})
	return err
}

func (r *runnableEmpty) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	isEmpty, err := checkEmpty(ctx, r.arg)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(isEmpty).ZVal(), nil
}

func compileEmpty(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	// empty() takes exactly one argument
	args, err := compileFuncPassedArgs(c)
	if err != nil {
		return nil, err
	}
	if len(args) != 1 {
		return nil, i.Unexpected()
	}
	return &runnableEmpty{arg: args[0], l: i.Loc()}, nil
}

func isValueEmpty(ctx phpv.Context, v *phpv.ZVal) bool {
	if v == nil {
		return true
	}
	switch v.GetType() {
	case phpv.ZtNull:
		return true
	case phpv.ZtBool:
		return !bool(v.Value().(phpv.ZBool))
	case phpv.ZtInt:
		return v.Value().(phpv.ZInt) == 0
	case phpv.ZtFloat:
		return v.Value().(phpv.ZFloat) == 0
	case phpv.ZtString:
		s := v.Value().(phpv.ZString)
		return s == "" || s == "0"
	case phpv.ZtArray:
		return v.Value().(*phpv.ZArray).Count(ctx) == 0
	case phpv.ZtObject:
		return false // objects are never empty
	}
	return false
}

func checkEmpty(ctx phpv.Context, v phpv.Runnable) (bool, error) {
	switch t := v.(type) {
	case *runVariable:
		exists, _ := ctx.OffsetExists(ctx, t.v.ZVal())
		if !exists {
			return true, nil
		}
		val, err := v.Run(ctx)
		if err != nil {
			return true, nil
		}
		return isValueEmpty(ctx, val), nil

	case *runArrayAccess:
		// Check if the container exists first (suppress warnings for undefined props)
		exists, _ := checkExistence(ctx, t.value, true)
		if !exists {
			return true, nil
		}

		// Now evaluate the container
		value, err := t.value.Run(ctx)
		if err != nil {
			return true, nil
		}
		if value == nil {
			return true, nil
		}

		if t.offset == nil {
			return true, nil
		}
		key, err := t.offset.Run(ctx)
		if err != nil {
			return true, nil
		}

		// For ArrayAccess objects, call offsetExists first
		if value.GetType() == phpv.ZtObject {
			obj, ok := value.Value().(*phpobj.ZObject)
			if ok && obj.GetClass().Implements(phpobj.ArrayAccess) {
				exists, err := obj.OffsetExists(ctx, key)
				if err != nil {
					return true, nil
				}
				if !exists {
					return true, nil
				}
				val, err := obj.OffsetGet(ctx, key)
				if err != nil {
					return true, nil
				}
				return isValueEmpty(ctx, val), nil
			}
		}

		// For arrays and strings, check existence then value
		var arr phpv.ZArrayAccess
		if value.GetType() == phpv.ZtString {
			str := value.AsString(ctx)
			arr = phpv.ZStringArray{ZString: &str}
		} else {
			var ok bool
			arr, ok = value.Value().(phpv.ZArrayAccess)
			if !ok {
				return true, nil
			}
		}
		exists, err = arr.OffsetExists(ctx, key)
		if err != nil || !exists {
			return true, nil
		}
		val, err := arr.OffsetGet(ctx, key)
		if err != nil {
			return true, nil
		}
		return isValueEmpty(ctx, val), nil

	case *runObjectVar:
		// For object property access
		value, err := t.ref.Run(ctx)
		if err != nil {
			return true, nil
		}
		if value.GetType() != phpv.ZtObject {
			return true, nil
		}
		obj := value.AsObject(ctx).(*phpobj.ZObject)
		// Resolve variable property name (e.g. $this->$name)
		propName := t.varName
		if len(propName) > 0 && propName[0] == '$' {
			propVal, err := ctx.OffsetGet(ctx, propName[1:].ZVal())
			if err != nil {
				return true, nil
			}
			propName = propVal.AsString(ctx)
		}
		exists, err := obj.HasProp(ctx, propName)
		if err != nil || !exists {
			return true, nil
		}
		val, err := obj.ObjectGet(ctx, propName)
		if err != nil {
			return true, nil
		}
		return isValueEmpty(ctx, val), nil

	case *runObjectDynVar:
		// For dynamic object property access: $obj->{expr}
		value, err := t.ref.Run(ctx)
		if err != nil {
			return true, nil
		}
		if value.GetType() != phpv.ZtObject {
			return true, nil
		}
		obj := value.AsObject(ctx).(*phpobj.ZObject)
		name, err := t.nameExpr.Run(ctx)
		if err != nil {
			return true, nil
		}
		propName := phpv.ZString(name.String())
		exists, err := obj.HasProp(ctx, propName)
		if err != nil || !exists {
			return true, nil
		}
		val, err := obj.ObjectGet(ctx, propName)
		if err != nil {
			return true, nil
		}
		return isValueEmpty(ctx, val), nil

	default:
		// For any other expression, just evaluate and check
		val, err := v.Run(ctx)
		if err != nil {
			return true, nil
		}
		return isValueEmpty(ctx, val), nil
	}
}

func checkExistence(ctx phpv.Context, v phpv.Runnable, subExpr bool) (bool, error) {
	// isset should only evaluate the sub-expressions:
	// - isset(foo()) // foo is not evaluated
	// - isset((foo())['x']) // foo is evaluated
	// - isset(($x[foo()]) // foo is evaluated
	// - isset($x->foo()) // foo is not evaluated
	switch t := v.(type) {
	case *runVariable:
		exists, err := ctx.OffsetExists(ctx, t.v.ZVal())
		if !exists || err != nil {
			return false, err
		}
		// isset() returns false for NULL values
		val, err := ctx.OffsetGet(ctx, t.v.ZVal())
		if err != nil {
			return false, err
		}
		return val != nil && !phpv.IsNull(val), nil

	case *runVariableRef:
		v, err := t.v.Run(ctx)
		if err != nil {
			return false, err
		}
		name := phpv.ZString(v.String())
		exists, err := ctx.OffsetExists(ctx, name.ZVal())
		if !exists || err != nil {
			return false, err
		}
		val, err := ctx.OffsetGet(ctx, name.ZVal())
		if err != nil {
			return false, err
		}
		return val != nil && !phpv.IsNull(val), nil

	case *runArrayAccess:
		exists, err := checkExistence(ctx, t.value, true)
		if !exists || err != nil {
			return exists, err
		}
		value, err := t.value.Run(ctx)
		if err != nil {
			return false, err
		}

		if t.offset == nil {
			return false, errors.New("Cannot use [] for reading")
		}
		key, err := t.offset.Run(ctx)
		if err != nil {
			return false, nil
		}

		// PHP 8.1: Deprecation warning for null array offsets
		if key.GetType() == phpv.ZtNull {
			if err := ctx.Deprecated("Using null as an array offset is deprecated, use an empty string instead", logopt.NoFuncName(true)); err != nil {
				return false, err
			}
			key = phpv.ZString("").ZVal()
		}

		var arr phpv.ZArrayAccess
		if value.GetType() == phpv.ZtString {
			if key.GetType() != phpv.ZtInt {
				return false, nil
			}
			str := value.AsString(ctx)
			arr = phpv.ZStringArray{ZString: &str}
		} else {
			var ok bool
			arr, ok = value.Value().(phpv.ZArrayAccess)
			if !ok {
				return false, nil
			}
		}

		return arr.OffsetExists(ctx, key)

	case *runObjectVar:
		exists, err := checkExistence(ctx, t.ref, true)
		if !exists || err != nil {
			return exists, err
		}
		value, err := t.ref.Run(ctx)
		if err != nil {
			return false, err
		}
		if value.GetType() != phpv.ZtObject {
			return false, nil
		}
		obj := value.AsObject(ctx).(*phpobj.ZObject)
		// Resolve variable property name (e.g. $this->$name)
		propName := t.varName
		if len(propName) > 0 && propName[0] == '$' {
			propVal, err := ctx.OffsetGet(ctx, propName[1:].ZVal())
			if err != nil {
				return false, nil
			}
			propName = propVal.AsString(ctx)
		}
		return obj.HasProp(ctx, propName)

	case *runObjectDynVar:
		exists, err := checkExistence(ctx, t.ref, true)
		if !exists || err != nil {
			return exists, err
		}
		value, err := t.ref.Run(ctx)
		if err != nil {
			return false, err
		}
		if value.GetType() != phpv.ZtObject {
			return false, nil
		}
		obj := value.AsObject(ctx).(*phpobj.ZObject)
		name, err := t.nameExpr.Run(ctx)
		if err != nil {
			return false, nil
		}
		propName := phpv.ZString(name.String())
		return obj.HasProp(ctx, propName)

	case *runClassStaticVarRef:
		// Check if the static property exists and is accessible
		className, err := t.className.Run(ctx)
		if err != nil {
			return false, nil
		}
		class, err := ctx.Global().GetClass(ctx, className.AsString(ctx), true)
		if err != nil {
			return false, nil
		}
		zc := class.(*phpobj.ZClass)
		// Check visibility: private/protected static properties are not
		// accessible from outside their declared scope.
		if !phpobj.IsStaticPropAccessible(ctx, zc, t.varName) {
			return false, nil
		}
		p, found, err := zc.FindStaticProp(ctx, t.varName)
		if err != nil || !found {
			return false, nil
		}
		val := p.GetString(t.varName)
		return val != nil && !phpv.IsNull(val), nil

	default:
		if !subExpr {
			return false, ctx.Errorf(`Cannot use isset() on the result of an expression (you can use "null !== expression" instead)`)
		}
		return true, nil
	}
}
