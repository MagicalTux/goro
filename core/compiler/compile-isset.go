package compiler

import (
	"fmt"
	"io"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phperr"
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
				// If HandleIssetDim + HandleReadDim are both available (e.g. SplFixedArray),
				// use internal data access to avoid PHP-level offsetExists/offsetGet calls.
				if h := phpobj.FindIssetDimHandler(obj.GetClass()); h != nil {
					if rh := phpobj.FindReadDimHandler(obj.GetClass()); rh != nil {
						result, err := h(ctx, obj, key)
						if err != nil {
							return true, nil
						}
						if !result {
							return true, nil
						}
						val, err := rh(ctx, obj, key)
						if err != nil {
							return true, nil
						}
						return isValueEmpty(ctx, val), nil
					}
				}
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
			// For string access in empty() context:
			// PHP converts key to integer index, with specific rules per type.
			// Array/object/resource keys → empty (true) without warnings.
			// Float keys → deprecation, then truncate to int.
			// Bool/null keys → silent coercion to int.
			// Non-numeric string keys → empty (true).
			var idx int
			switch key.GetType() {
			case phpv.ZtInt:
				idx = int(key.Value().(phpv.ZInt))
			case phpv.ZtBool:
				if key.Value().(phpv.ZBool) {
					idx = 1
				} else {
					idx = 0
				}
			case phpv.ZtNull:
				idx = 0
			case phpv.ZtFloat:
				fval := float64(key.Value().(phpv.ZFloat))
				if err := ctx.Deprecated("Implicit conversion from float %v to int loses precision", fval, logopt.NoFuncName(true)); err != nil {
					return false, err
				}
				idx = int(key.AsInt(ctx))
			case phpv.ZtString:
				s := key.AsString(ctx)
				if !s.IsNumeric() {
					return true, nil
				}
				// Check if it's a float-like numeric string (has '.' or 'e/E')
				sStr := string(s)
				isFloat := false
				for _, ch := range sStr {
					if ch == '.' || ch == 'e' || ch == 'E' {
						isFloat = true
						break
					}
				}
				if isFloat {
					return true, nil
				}
				idx = int(key.AsInt(ctx))
			default:
				// Array, object, resource → empty without warnings
				return true, nil
			}
			// Check bounds without warnings
			str := value.AsString(ctx)
			strLen := len(str)
			if idx < 0 {
				idx = strLen + idx
			}
			if idx < 0 || idx >= strLen {
				return true, nil
			}
			ch := phpv.ZString(string(str[idx]))
			return ch == "" || ch == "0", nil
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
		// Use checkExistenceAndGet to get the container value without double evaluation
		containerExists, containerVal, containerErr := checkExistenceAndGet(ctx, t.value, true)
		if !containerExists || containerErr != nil {
			return containerExists, containerErr
		}
		var value *phpv.ZVal
		var err error
		if containerVal != nil {
			// Reuse cached value (avoids double offsetGet for nested ArrayAccess)
			value = containerVal
		} else {
			value, err = t.value.Run(ctx)
			if err != nil {
				return false, err
			}
		}

		if t.offset == nil {
			return false, &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use [] for reading"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  t.l,
			}
		}
		// Use cached offset from PrepareWrite if available (e.g. ??= memoization)
		var key *phpv.ZVal
		if t.prepared && t.cachedOffset != nil {
			key = t.cachedOffset
			// Don't consume the cache - it will be needed by WriteValue later
		} else {
			key, err = t.offset.Run(ctx)
			if err != nil {
				// Exceptions (PhpThrow) must propagate; non-exception errors return false.
				if _, isThrow := phpv.UnwrapError(err).(*phperr.PhpThrow); isThrow {
					return false, err
				}
				return false, nil
			}
		}

		// PHP 8.1: Deprecation warning for null array offsets
		if key.GetType() == phpv.ZtNull {
			if err := ctx.Deprecated("Using null as an array offset is deprecated, use an empty string instead", logopt.NoFuncName(true)); err != nil {
				return false, err
			}
			key = phpv.ZString("").ZVal()
		}

		// PHP 8.0+: array/object as array offset in isset/empty throws TypeError
		// (only when the container is an actual PHP array or string, not an object with ArrayAccess)
		if value.GetType() != phpv.ZtObject {
			switch key.GetType() {
			case phpv.ZtArray:
				return false, phpobj.ThrowError(ctx, phpobj.TypeError, "Cannot access offset of type array in isset or empty")
			case phpv.ZtObject:
				typeName := "object"
				if obj, ok := key.Value().(phpv.ZObject); ok {
					typeName = string(obj.GetClass().GetName())
				}
				return false, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("Cannot access offset of type %s in isset or empty", typeName))
			}
		}

		var arr phpv.ZArrayAccess
		if value.GetType() == phpv.ZtString {
			// PHP isset on string offsets accepts various key types:
			// int -> direct offset
			// string -> only integer numeric strings (not "1.5", not "abc")
			// bool -> true=1, false=0
			// float -> truncated to int (with deprecation in real PHP)
			// null -> treated as 0
			// array, object, resource -> false
			switch key.GetType() {
			case phpv.ZtInt:
				// good
			case phpv.ZtString:
				// Check if it's a pure integer numeric string (not float)
				s := key.AsString(ctx)
				if !s.IsNumeric() {
					return false, nil
				}
				// Must be integer-like (no dots, no 'e' notation)
				for _, c := range string(s) {
					if c == '.' || c == 'e' || c == 'E' {
						return false, nil
					}
				}
				key = key.AsInt(ctx).ZVal()
			case phpv.ZtBool:
				if key.Value().(phpv.ZBool) {
					key = phpv.ZInt(1).ZVal()
				} else {
					key = phpv.ZInt(0).ZVal()
				}
			case phpv.ZtFloat:
				key = key.AsInt(ctx).ZVal()
			case phpv.ZtNull:
				key = phpv.ZInt(0).ZVal()
			default:
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

		// For objects with HandleIssetDim (like ArrayObject), use the special handler.
		if value.GetType() == phpv.ZtObject {
			if obj, ok := value.Value().(*phpobj.ZObject); ok {
				if h := phpobj.FindIssetDimHandler(obj.GetClass()); h != nil {
					return h(ctx, obj, key)
				}
				// For ArrayAccess objects without a special handler, PHP's isset() ONLY
				// calls offsetExists() — not offsetGet() for a null check.
				// The contract is that offsetExists() returns false if value is null.
				return obj.OffsetExists(ctx, key)
			}
		}
		// For regular PHP arrays (ZArrayAccess, not objects), isset checks both
		// key existence AND that the value is not null.
		exists, existsErr := arr.OffsetExists(ctx, key)
		if !exists || existsErr != nil {
			return false, existsErr
		}
		val, valErr := arr.OffsetGet(ctx, key)
		if valErr != nil {
			return false, nil
		}
		return val != nil && !phpv.IsNull(val), nil

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
		// Use cached name from PrepareWrite if available (e.g. ??= memoization)
		var name *phpv.ZVal
		if t.prepared && t.cachedName != nil {
			name = t.cachedName
		} else {
			name, err = t.nameExpr.Run(ctx)
			if err != nil {
				return false, nil
			}
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

	case *runClassStaticDynVarRef:
		className, err := t.className.Run(ctx)
		if err != nil {
			return false, nil
		}
		class, err := ctx.Global().GetClass(ctx, className.AsString(ctx), true)
		if err != nil {
			return false, nil
		}
		// Use cached name from PrepareWrite if available
		var varName phpv.ZString
		if t.prepared && t.cachedName != "" {
			varName = t.cachedName
		} else {
			nameVal, nameErr := t.nameExpr.Run(ctx)
			if nameErr != nil {
				return false, nil
			}
			varName = phpv.ZString(nameVal.String())
		}
		zc := class.(*phpobj.ZClass)
		p, found, err := zc.FindStaticProp(ctx, varName)
		if err != nil || !found {
			return false, nil
		}
		val := p.GetString(varName)
		return val != nil && !phpv.IsNull(val), nil

	default:
		if !subExpr {
			return false, ctx.Errorf(`Cannot use isset() on the result of an expression (you can use "null !== expression" instead)`)
		}
		return true, nil
	}
}

// checkExistenceAndGet is like checkExistence but also returns the fetched value
// when it has to read it (e.g., to navigate into a nested structure or check for null).
// This avoids double offsetGet calls in the ?? and ??= operators.
// Returns (exists bool, val *phpv.ZVal, err error).
// When exists=false, val is nil. When exists=true, val may be nil if not fetched.
func checkExistenceAndGet(ctx phpv.Context, v phpv.Runnable, subExpr bool) (bool, *phpv.ZVal, error) {
	switch t := v.(type) {
	case *runVariable:
		exists, err := ctx.OffsetExists(ctx, t.v.ZVal())
		if !exists || err != nil {
			return false, nil, err
		}
		val, err := ctx.OffsetGet(ctx, t.v.ZVal())
		if err != nil {
			return false, nil, err
		}
		if val == nil || phpv.IsNull(val) {
			return false, nil, nil
		}
		return true, val, nil

	case *runVariableRef:
		vv, err := t.v.Run(ctx)
		if err != nil {
			return false, nil, err
		}
		name := phpv.ZString(vv.String())
		exists, err := ctx.OffsetExists(ctx, name.ZVal())
		if !exists || err != nil {
			return false, nil, err
		}
		val, err := ctx.OffsetGet(ctx, name.ZVal())
		if err != nil {
			return false, nil, err
		}
		if val == nil || phpv.IsNull(val) {
			return false, nil, nil
		}
		return true, val, nil

	case *runArrayAccess:
		var value *phpv.ZVal
		var err error
		// Try to get the container value from the recursive existence check,
		// to avoid double offsetGet on ArrayAccess objects.
		containerExists, containerVal, containerErr := checkExistenceAndGet(ctx, t.value, true)
		if !containerExists || containerErr != nil {
			return containerExists, nil, containerErr
		}
		if containerVal != nil {
			// Reuse the value already fetched during existence check
			value = containerVal
		} else {
			// Container exists but value wasn't cached (e.g., plain variable) — fetch it
			value, err = t.value.Run(ctx)
			if err != nil {
				return false, nil, err
			}
		}

		if t.offset == nil {
			return false, nil, &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use [] for reading"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  t.l,
			}
		}
		var key *phpv.ZVal
		if t.prepared && t.cachedOffset != nil {
			key = t.cachedOffset
		} else {
			key, err = t.offset.Run(ctx)
			if err != nil {
				return false, nil, nil
			}
		}

		if key.GetType() == phpv.ZtNull {
			if err := ctx.Deprecated("Using null as an array offset is deprecated, use an empty string instead", logopt.NoFuncName(true)); err != nil {
				return false, nil, err
			}
			key = phpv.ZString("").ZVal()
		}

		if value.GetType() == phpv.ZtObject {
			if obj, ok := value.Value().(*phpobj.ZObject); ok {
				if obj.GetClass().Implements(phpobj.ArrayAccess) {
					if h := phpobj.FindIssetDimHandler(obj.GetClass()); h != nil {
						// Has special isset handler — use it for existence check
						existsVal, hErr := h(ctx, obj, key)
						if hErr != nil || !existsVal {
							return false, nil, hErr
						}
						// Now get the value (for null check and for reuse)
						if rh := phpobj.FindReadDimHandler(obj.GetClass()); rh != nil {
							val, rErr := rh(ctx, obj, key)
							if rErr != nil {
								return false, nil, nil
							}
							if val == nil || phpv.IsNull(val) {
								return false, nil, nil
							}
							return true, val, nil
						}
						// Fall back to offsetGet
						val, oErr := obj.OffsetGet(ctx, key)
						if oErr != nil {
							return false, nil, nil
						}
						if val == nil || phpv.IsNull(val) {
							return false, nil, nil
						}
						return true, val, nil
					}
					// No special handler: use offsetExists + offsetGet
					existsVal, eErr := obj.OffsetExists(ctx, key)
					if eErr != nil || !existsVal {
						return false, nil, eErr
					}
					val, oErr := obj.OffsetGet(ctx, key)
					if oErr != nil {
						return false, nil, nil
					}
					if val == nil || phpv.IsNull(val) {
						return false, nil, nil
					}
					return true, val, nil
				}
			}
		}

		var arr phpv.ZArrayAccess
		if value.GetType() == phpv.ZtString {
			switch key.GetType() {
			case phpv.ZtInt:
			case phpv.ZtString:
				s := key.AsString(ctx)
				if !s.IsNumeric() {
					return false, nil, nil
				}
				for _, c := range string(s) {
					if c == '.' || c == 'e' || c == 'E' {
						return false, nil, nil
					}
				}
				key = key.AsInt(ctx).ZVal()
			case phpv.ZtBool:
				if key.Value().(phpv.ZBool) {
					key = phpv.ZInt(1).ZVal()
				} else {
					key = phpv.ZInt(0).ZVal()
				}
			case phpv.ZtFloat:
				key = key.AsInt(ctx).ZVal()
			case phpv.ZtNull:
				key = phpv.ZInt(0).ZVal()
			default:
				return false, nil, nil
			}
			str := value.AsString(ctx)
			arr = phpv.ZStringArray{ZString: &str}
		} else {
			var ok bool
			arr, ok = value.Value().(phpv.ZArrayAccess)
			if !ok {
				return false, nil, nil
			}
		}
		existsVal, existsErr := arr.OffsetExists(ctx, key)
		if !existsVal || existsErr != nil {
			return false, nil, existsErr
		}
		val, valErr := arr.OffsetGet(ctx, key)
		if valErr != nil {
			return false, nil, nil
		}
		if val == nil || phpv.IsNull(val) {
			return false, nil, nil
		}
		return true, val, nil

	default:
		// For everything else, fall back to checkExistence (no value caching)
		exists, err := checkExistence(ctx, v, subExpr)
		return exists, nil, err
	}
}
