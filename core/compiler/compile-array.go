package compiler

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"github.com/MagicalTux/goro/core/tokenizer"
)

// isLeadingNumeric checks if a string starts with a digit or +-digit.
// Used to distinguish "foo" (TypeError) from "0foo" (warning) for string offsets.
func isLeadingNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	i := 0
	if s[i] == '+' || s[i] == '-' {
		i++
	}
	if i >= len(s) {
		return false
	}
	return s[i] >= '0' && s[i] <= '9'
}

// isNumericString checks if a string is a valid numeric string (integer or float).
func isNumericString(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return false
	}
	i := 0
	if s[i] == '+' || s[i] == '-' {
		i++
	}
	if i >= len(s) {
		return false
	}
	hasDigit := false
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		hasDigit = true
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			hasDigit = true
			i++
		}
	}
	if !hasDigit {
		return false
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		if i >= len(s) || s[i] < '0' || s[i] > '9' {
			return false
		}
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
	}
	return i == len(s)
}

type arrayEntry struct {
	k, v   phpv.Runnable
	spread bool // ...$expr spread syntax
}

type runArray struct {
	e []*arrayEntry
	l *phpv.Loc
}

func (a runArray) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	var err error
	array := phpv.NewZArray()

	for _, e := range a.e {
		if e.spread {
			// ...$expr - unpack iterable into the array
			v, err := e.v.Run(ctx)
			if err != nil {
				return nil, err
			}
			if v.GetType() == phpv.ZtArray {
				src := v.AsArray(ctx)
				for k, v := range src.Iterate(ctx) {
					// PHP 8.1+: string keys are preserved; int keys are re-indexed
					if k.GetType() == phpv.ZtString {
						array.OffsetSet(ctx, k, v.Dup())
					} else {
						err := array.OffsetSet(ctx, nil, v.Dup())
						if err != nil {
							return nil, phpobj.ThrowError(ctx, phpobj.Error, err.Error())
						}
					}
				}
			} else if v.GetType() == phpv.ZtObject {
				// Handle Traversable objects (Iterator, IteratorAggregate, Generator)
				obj, ok := v.Value().(*phpobj.ZObject)
				if !ok {
					typeName := "object"
					return nil, phpobj.ThrowError(ctx, phpobj.Error,
						fmt.Sprintf("Only arrays and Traversables can be unpacked, %s given", typeName))
				}
				if obj.GetClass().Implements(phpobj.IteratorAggregate) {
					iterResult, iterErr := obj.CallMethod(ctx, "getIterator")
					if iterErr == nil && iterResult != nil && iterResult.GetType() == phpv.ZtObject {
						if iterObj, ok := iterResult.Value().(*phpobj.ZObject); ok && iterObj.GetClass().Implements(phpobj.Iterator) {
							obj = iterObj
						}
					}
				}
				if obj.GetClass().Implements(phpobj.Iterator) {
					obj.CallMethod(ctx, "rewind")
					for {
						valid, verr := obj.CallMethod(ctx, "valid")
						if verr != nil || !valid.AsBool(ctx) {
							break
						}
						key, kerr := obj.CallMethod(ctx, "key")
						if kerr != nil {
							break
						}
						// Validate key type: must be int or string
						if key.GetType() != phpv.ZtInt && key.GetType() != phpv.ZtString {
							return nil, phpobj.ThrowError(ctx, phpobj.Error,
								"Keys must be of type int|string during array unpacking")
						}
						value, verr := obj.CallMethod(ctx, "current")
						if verr != nil {
							break
						}
						// PHP 8.1+: string keys are preserved; numeric string keys
						// that look like integers are converted to integers and re-indexed
						if key.GetType() == phpv.ZtString {
							keyStr := string(key.AsString(ctx))
							if isNumericString(keyStr) {
								// Numeric string key from iterator → re-index as integer
								array.OffsetSet(ctx, nil, value.Dup())
							} else {
								array.OffsetSet(ctx, key, value.Dup())
							}
						} else {
							array.OffsetSet(ctx, nil, value.Dup())
						}
						obj.CallMethod(ctx, "next")
					}
				} else {
					typeName := string(obj.GetClass().GetName())
					return nil, phpobj.ThrowError(ctx, phpobj.Error,
						fmt.Sprintf("Only arrays and Traversables can be unpacked, %s given", typeName))
				}
			} else {
				typeName := v.GetType().TypeName()
				return nil, phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("Only arrays and Traversables can be unpacked, %s given", typeName))
			}
			continue
		}

		var k, v *phpv.ZVal

		if e.k != nil {
			k, err = e.k.Run(ctx)
			if err != nil {
				return nil, err
			}
		}
		v, err = e.v.Run(ctx)
		if err != nil {
			return nil, err
		}

		array.OffsetSet(ctx, k, v.ZVal())
	}

	return array.ZVal(), nil
}

func (a *runArray) Loc() *phpv.Loc {
	return a.l
}

func (a runArray) Dump(w io.Writer) error {
	_, err := w.Write([]byte{'['})
	if err != nil {
		return err
	}
	for _, s := range a.e {
		if s.k != nil {
			err = s.k.Dump(w)
			if err != nil {
				return err
			}
			_, err = w.Write([]byte("=>"))
			if err != nil {
				return err
			}
		}
		err = s.v.Dump(w)
		if err != nil {
			return err
		}
	}
	_, err = w.Write([]byte{']'})
	return err
}

type runArrayAccess struct {
	runnableChild
	value        phpv.Runnable
	offset       phpv.Runnable
	l            *phpv.Loc
	writeContext bool // set when reading as part of a write chain (suppress undefined key warnings)

	// Set by Run() to indicate the container was an ArrayAccess object.
	// Used by runOperator to emit "Indirect modification" notices for compound ops.
	lastContainerIsOverloaded bool
	lastContainerClassName    string

	// PrepareWrite caching
	prepared     bool
	cachedOffset *phpv.ZVal

	// Compound assignment caching: during .= += etc., cache the container
	// from the read phase so WriteValue doesn't re-evaluate the chain.
	compoundCache    bool      // set by runOperator to enable caching
	cachedContainer  *phpv.ZVal // cached result of ac.value.Run(ctx) from Run()
}

func (r *runArrayAccess) Dump(w io.Writer) error {
	err := r.value.Dump(w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte{'['})
	if err != nil {
		return err
	}
	if r.offset != nil {
		err = r.offset.Dump(w)
		if err != nil {
			return err
		}
	}
	_, err = w.Write([]byte{']'})
	return err
}

// IsOverloaded checks if the container of this array access is an object with ArrayAccess.
// This is used to detect illegal operations like assigning by reference.
// Also sets lastContainerClassName for error messages.
func (ac *runArrayAccess) IsOverloaded(ctx phpv.Context) bool {
	v, err := ac.value.Run(ctx)
	if err != nil {
		return false
	}
	if v.GetType() == phpv.ZtObject {
		obj := v.AsObject(ctx)
		if obj != nil && obj.GetClass().Implements(phpobj.ArrayAccess) {
			ac.lastContainerIsOverloaded = true
			ac.lastContainerClassName = string(obj.GetClass().GetName())
			return true
		}
	}
	return false
}

func (ac *runArrayAccess) Run(ctx phpv.Context) (*phpv.ZVal, error) {
	// Reset overloaded container tracking
	ac.lastContainerIsOverloaded = false
	ac.lastContainerClassName = ""

	// Propagate writeContext down the chain to suppress warnings during auto-vivification
	if ac.writeContext {
		if inner, ok := ac.value.(*runArrayAccess); ok {
			inner.writeContext = true
			defer func() { inner.writeContext = false }()
		}
		if inner, ok := ac.value.(*runObjectVar); ok {
			inner.writeContext = true
			defer func() { inner.writeContext = false }()
		}
	}
	v, err := ac.value.Run(ctx)
	if err != nil {
		return nil, err
	}

	// Cache container for compound assignment write-back (avoids re-evaluating the chain)
	if ac.compoundCache {
		ac.cachedContainer = v
		ac.compoundCache = false
	}

	// Check for [] (empty offset) in read context — must be caught early
	// before type-specific handling returns null for undefined containers
	if ac.offset == nil && !ac.writeContext {
		write := false
		switch t := ac.Parent.(type) {
		case *runOperator:
			write = t.opD != nil && t.opD.write
		case *runArrayAccess, *runnableForeach, *runDestructure:
			write = true
		}
		if !write {
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use [] for reading"),
				Code: phpv.E_ERROR,
				Loc:  ac.l,
			}
		}
	}

	switch v.GetType() {
	case phpv.ZtString:
	case phpv.ZtArray:
	case phpv.ZtObject:
		// Track if container is an ArrayAccess object (not a plain array)
		ac.lastContainerIsOverloaded = true
		ac.lastContainerClassName = string(v.AsObject(ctx).GetClass().GetName())
	case phpv.ZtNull:
		// Check if this is a compound write context (e.g. $a[$b] += 1)
		isCompoundWrite := false
		if !ac.writeContext {
			if op, ok := ac.Parent.(*runOperator); ok && op.opD != nil && op.opD.write && op.opD.op != nil {
				isCompoundWrite = true
			}
		}
		if isCompoundWrite {
			// Compound assignment: auto-vivify null to array (like writeContext).
			// Emit "Undefined variable" warning if applicable, then proceed
			// with the offset evaluation so that undefined offsets also get warned about.
			if uc, ok := ac.value.(phpv.UndefinedChecker); ok {
				if uc.IsUnDefined(ctx) {
					if err := ctx.Warn("Undefined variable $%s",
						uc.VarName(), logopt.NoFuncName(true)); err != nil {
						return nil, err
					}
				}
			}
			// Auto-vivify: cast null to empty array and write back
			err = v.CastTo(ctx, phpv.ZtArray)
			if err != nil {
				return nil, err
			}
			if wr, ok := ac.value.(phpv.Writable); ok {
				wr.WriteValue(ctx, v)
			}
		} else if !ac.writeContext {
			// Check if the inner expression is an undefined variable and emit warning
			if uc, ok := ac.value.(phpv.UndefinedChecker); ok {
				if uc.IsUnDefined(ctx) {
					if err := ctx.Warn("Undefined variable $%s",
						uc.VarName(), logopt.NoFuncName(true)); err != nil {
						return nil, err
					}
				}
			}
			if err := ctx.Warn("Trying to access array offset on null", logopt.NoFuncName(true)); err != nil {
				return nil, err
			}
			return phpv.ZNULL.ZVal(), nil
		}
		if !isCompoundWrite {
			return phpv.ZNULL.ZVal(), nil
		}
	case phpv.ZtBool:
		if !ac.writeContext {
			boolName := "true"
			if !bool(v.AsBool(ctx)) {
				boolName = "false"
			}
			if err := ctx.Warn("Trying to access array offset on %s", boolName, logopt.NoFuncName(true)); err != nil {
				return nil, err
			}
			return phpv.ZNULL.ZVal(), nil
		}
		v, err = v.As(ctx, phpv.ZtArray)
		if err != nil {
			return nil, err
		}
	default:
		// PHP 8: accessing a scalar with array syntax in write context throws Error
		isWriteOp := ac.writeContext
		if !isWriteOp {
			if op, ok := ac.Parent.(*runOperator); ok && op.opD != nil && op.opD.write {
				isWriteOp = true
			}
		}
		if isWriteOp {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot use a scalar value as an array")
		}
		// PHP 8: reading array offset on non-array scalar (int, float) warns and returns null
		typeName := v.GetType().TypeName()
		if err := ctx.Warn("Trying to access array offset on %s", typeName, logopt.NoFuncName(true)); err != nil {
			return nil, err
		}
		return phpv.ZNULL.ZVal(), nil
	}

	if ac.offset == nil {
		write := false
		switch t := ac.Parent.(type) {
		case *runOperator:
			write = t.opD.write
		case *runArrayAccess, *runnableForeach, *runDestructure:
			write = true
		}

		if !write {
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use [] for reading"),
				Code: phpv.E_ERROR,
				Loc:  ac.l,
			}
		}
		return nil, nil
	}

	offset, err := ac.getArrayOffset(ctx)
	if err != nil {
		return nil, err
	}

	// PHP 8.1: Deprecation warning for null array offsets (read)
	if offset.GetType() == phpv.ZtNull {
		if err := ctx.Deprecated("Using null as an array offset is deprecated, use an empty string instead", logopt.NoFuncName(true)); err != nil {
			return nil, err
		}
	}

	if v.GetType() == phpv.ZtString {
		// PHP 8: object offsets on strings are not allowed
		if offset.GetType() == phpv.ZtObject {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("Cannot access offset of type %s on string", offset.Value().(phpv.ZObject).GetClass().GetName()))
		}
		// PHP 8: array offsets on strings are not allowed
		if offset.GetType() == phpv.ZtArray {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "Cannot access offset of type array on string")
		}
		// PHP 8: completely non-numeric string offsets on strings throw TypeError.
		// Strings with leading digits (like "0foo") produce a warning instead.
		if offset.GetType() == phpv.ZtString {
			s := strings.TrimSpace(string(offset.AsString(ctx)))
			if len(s) > 0 && !isLeadingNumeric(s) {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "Cannot access offset of type string on string")
			}
		}
		return v.AsString(ctx).Array().OffsetGet(ctx, offset)
	}

	// Check for invalid offset types on arrays
	if offset.GetType() == phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("Cannot access offset of type %s on array", offset.Value().(phpv.ZObject).GetClass().GetName()))
	}
	if offset.GetType() == phpv.ZtArray {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "Cannot access offset of type array on array")
	}

	array := v.Array()
	if array == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot use object of type %s as array", v.GetType()))
	}

	// Use OffsetGetWarn for ZArray to produce "Undefined array key" warnings
	// but not when this access is part of a write chain (auto-vivification)
	if !ac.writeContext {
		if za, ok := array.(*phpv.ZArray); ok {
			return za.OffsetGetWarn(ctx, offset)
		}
	}
	return array.OffsetGet(ctx, offset)
}

func (a *runArrayAccess) IsCompoundWritable() {}

func (a *runArrayAccess) Loc() *phpv.Loc {
	return a.l
}

func (ac *runArrayAccess) SetWriteContext(v bool) {
	ac.writeContext = v
}

func (ac *runArrayAccess) WriteValue(ctx phpv.Context, value *phpv.ZVal) error {
	var v *phpv.ZVal
	var err error

	// Use cached container from compound assignment read phase if available
	if ac.cachedContainer != nil {
		v = ac.cachedContainer
		ac.cachedContainer = nil
	} else {
		// Suppress undefined key/property warnings for inner accesses in write context
		if inner, ok := ac.value.(*runArrayAccess); ok {
			inner.writeContext = true
			defer func() { inner.writeContext = false }()
		}
		if inner, ok := ac.value.(*runObjectVar); ok {
			inner.writeContext = true
			defer func() { inner.writeContext = false }()
		}
		v, err = ac.value.Run(ctx)
		if err != nil {
			return err
		}
	}

	// PHP: writing to a sub-element of an ArrayAccess return value is an indirect
	// modification — offsetGet returns by value, so the write has no effect.
	// But if offsetGet returned an object (e.g. another ArrayAccess), writes to it
	// go through that object's offsetSet and work correctly.
	if inner, ok := ac.value.(*runArrayAccess); ok && inner.lastContainerIsOverloaded && v.GetType() != phpv.ZtObject {
		return ctx.Notice("Indirect modification of overloaded element of %s has no effect", inner.lastContainerClassName, logopt.Data{Loc: ac.l, NoFuncName: true})
	}

	// PHP 8.1: Cannot indirectly modify readonly property.
	// If the container expression chain resolves through a readonly property
	// access ($obj->prop[...] = val or $obj->prop[0][...] = val), block it.
	if err := checkReadonlyIndirectModification(ctx, ac.value); err != nil {
		return err
	}

	// Handle unset ($a[x] = nil means unset)
	if value == nil {
		array := v.Array()
		if array == nil {
			return nil
		}
		if ac.offset == nil {
			return nil
		}
		offset, err := ac.getArrayOffset(ctx)
		if err != nil {
			return err
		}
		return array.OffsetUnset(ctx, offset)
	}

	switch v.GetType() {
	case phpv.ZtString:
		return ac.writeValueToString(ctx, value)

	case phpv.ZtArray:
	case phpv.ZtObject:
	case phpv.ZtNull:
		// null can be auto-vivified to array
		err = v.CastTo(ctx, phpv.ZtArray)
		if err != nil {
			return err
		}
		if wr, ok := ac.value.(phpv.Writable); ok {
			wr.WriteValue(ctx, v)
		}
	case phpv.ZtBool:
		if !bool(v.AsBool(ctx)) {
			// PHP 8.1: false auto-vivifies to empty array with deprecation warning
			if err := ctx.Deprecated("Automatic conversion of false to array is deprecated", logopt.NoFuncName(true)); err != nil {
				return err
			}
			// Set to empty array (not cast, which would produce [false])
			v.Set(phpv.NewZArray().ZVal())
			if wr, ok := ac.value.(phpv.Writable); ok {
				wr.WriteValue(ctx, v)
			}
		} else {
			// PHP 8: true cannot be used as array
			return phpobj.ThrowError(ctx, phpobj.Error, "Cannot use a scalar value as an array")
		}
	default:
		// PHP 8: "Cannot use a scalar value as an array"
		return phpobj.ThrowError(ctx, phpobj.Error, "Cannot use a scalar value as an array")
	}

	array := v.Array()
	if array == nil {
		return phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot use object of type %s as array", v.GetType()))
	}

	if ac.offset == nil {
		// append
		return array.OffsetSet(ctx, nil, value)
	}

	offset, err := ac.getArrayOffset(ctx)
	if err != nil {
		return err
	}

	// Check for invalid offset types on arrays
	if offset.GetType() == phpv.ZtObject {
		return phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("Cannot access offset of type %s on array", offset.Value().(phpv.ZObject).GetClass().GetName()))
	}
	if offset.GetType() == phpv.ZtArray {
		return phpobj.ThrowError(ctx, phpobj.TypeError, "Cannot access offset of type array on array")
	}

	// PHP 8.1: Deprecation warning for null array offsets (write)
	if offset.GetType() == phpv.ZtNull {
		if err := ctx.Deprecated("Using null as an array offset is deprecated, use an empty string instead", logopt.NoFuncName(true)); err != nil {
			return err
		}
	}

	// OK...
	return array.OffsetSet(ctx, offset, value)
}

func (ac *runArrayAccess) writeValueToString(ctx phpv.Context, value *phpv.ZVal) error {
	v, err := ac.value.Run(ctx)
	if err != nil {
		return err
	}

	if ac.offset == nil {
		return ctx.Errorf("[] operator not supported for strings")
	}

	offset, err := ac.getArrayOffset(ctx)
	if err != nil {
		return err
	}

	// PHP 8: object offsets on strings are not allowed
	if offset.GetType() == phpv.ZtObject {
		return phpobj.ThrowError(ctx, phpobj.TypeError,
			fmt.Sprintf("Cannot access offset of type %s on string", offset.Value().(phpv.ZObject).GetClass().GetName()))
	}

	if phpv.IsNull(offset) {
		return errors.New("[] operator not supported for string")
	}

	// PHP: when assigning to a string offset, only the first byte of the
	// value is used. If the value is a multi-character string, warn.
	valStr := value.AsString(ctx)
	if len(valStr) == 0 {
		valStr = "\x00"
	} else if len(valStr) > 1 {
		ctx.Warn("Only the first byte will be assigned to the string offset")
		valStr = valStr[:1]
	}
	assignVal := valStr.ZVal()

	array := v.AsString(ctx).Array()

	err = array.OffsetSet(ctx, offset, assignVal)
	if err != nil {
		return err
	}

	if wr, ok := ac.value.(phpv.Writable); ok {
		wr.WriteValue(ctx, array.String().ZVal())
	}

	// Update the passed-in value to reflect what was actually written (single char).
	// This ensures chained assignments like $b = $str[N] = $s return the truncated value.
	value.Set(assignVal)

	return nil
}

func (ac *runArrayAccess) PrepareWrite(ctx phpv.Context) error {
	// Recursively prepare nested LHS expressions
	if inner, ok := ac.value.(phpv.WritePreparable); ok {
		if err := inner.PrepareWrite(ctx); err != nil {
			return err
		}
	}
	// Evaluate and cache the offset expression. We must snapshot the value
	// because the original ZVal may be mutated later (e.g., ++$a returns
	// the same ZVal that gets incremented by subsequent calls).
	if ac.offset != nil {
		offset, err := ac.offset.Run(ctx)
		if err != nil {
			return err
		}
		ac.prepared = true
		ac.cachedOffset = offset.Dup()
	}
	return nil
}

func (ac *runArrayAccess) getArrayOffset(ctx phpv.Context) (*phpv.ZVal, error) {
	var offset *phpv.ZVal
	var err error
	if ac.prepared {
		offset = ac.cachedOffset
		ac.prepared = false
		ac.cachedOffset = nil
	} else {
		offset, err = ac.offset.Run(ctx)
		if err != nil {
			return nil, err
		}
	}

	switch offset.GetType() {
	case phpv.ZtResource, phpv.ZtFloat:
		offset, err = offset.As(ctx, phpv.ZtInt)
		if err != nil {
			return nil, err
		}
	case phpv.ZtString:
	case phpv.ZtInt:
	case phpv.ZtNull:
		// Null converts to empty string as array key (deprecation warning is handled by callers)
	case phpv.ZtObject, phpv.ZtArray:
		// Invalid offset types — callers are responsible for checking and
		// producing context-specific error messages (e.g. "on array" vs "on string").
		return offset, nil
	default:
		offset, err = offset.As(ctx, phpv.ZtString)
	}

	return offset, err
}

func compileArray(i *tokenizer.Item, c compileCtx) (phpv.Runnable, error) {
	res := &runArray{l: i.Loc()}

	array_type := '?'

	if i.IsSingle('[') {
		array_type = ']'
	} else if i.Type == tokenizer.T_ARRAY {
		i, err := c.NextItem()
		if err != nil {
			return nil, err
		}

		if !i.IsSingle('(') {
			return nil, i.Unexpected()
		}
		array_type = ')'
	} else {
		return nil, i.Unexpected()
	}

	for {
		i, err := c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(array_type) {
			break
		}

		// Detect empty array elements (e.g., [1, , 3])
		if i.IsSingle(',') {
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use empty array elements in arrays"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  i.Loc(),
			}
		}

		// Handle spread operator: ...$expr
		if i.Type == tokenizer.T_ELLIPSIS {
			spreadLoc := i.Loc()
			spreadExpr, err := compileExpr(nil, c)
			if err != nil {
				return nil, err
			}
			// Compile-time check: literal non-array/non-object values cannot be unpacked
			if zv, ok := spreadExpr.(*runZVal); ok {
				switch zv.v.(type) {
				case *phpv.ZArray:
					// OK - array literal can be unpacked
				default:
					typeName := zv.v.(phpv.Val).GetType().TypeName()
					phpErr := &phpv.PhpError{
						Err:  fmt.Errorf("Only arrays and Traversables can be unpacked, %s given", typeName),
						Code: phpv.E_ERROR,
						Loc:  spreadLoc,
					}
					c.Global().LogError(phpErr)
					return nil, phpv.ExitError(255)
				}
			}
			res.e = append(res.e, &arrayEntry{v: spreadExpr, spread: true})

			i, err = c.NextItem()
			if err != nil {
				return nil, err
			}
			if i.IsSingle(',') {
				continue
			}
			if i.IsSingle(array_type) {
				break
			}
			return nil, i.Unexpected()
		}

		var k phpv.Runnable
		k, err = compileExpr(i, c)
		if err != nil {
			return nil, err
		}

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(',') {
			res.e = append(res.e, &arrayEntry{v: k})
			continue
		}

		if i.IsSingle(array_type) {
			res.e = append(res.e, &arrayEntry{v: k})
			break
		}

		if i.Type != tokenizer.T_DOUBLE_ARROW {
			return nil, i.Unexpected()
		}

		// ok we got a value now
		var v phpv.Runnable
		v, err = compileExpr(nil, c)
		if err != nil {
			return nil, err
		}

		res.e = append(res.e, &arrayEntry{k: k, v: v})

		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		if i.IsSingle(',') {
			continue
		}

		if i.IsSingle(array_type) {
			break
		}
		return nil, i.Unexpected()
	}

	return res, nil
}

func compileArrayAccess(v phpv.Runnable, c compileCtx) (phpv.Runnable, error) {
	// we got a [
	i, err := c.NextItem()
	if err != nil {
		return nil, err
	}

	var endc rune
	switch i.Rune() {
	case '[':
		endc = ']'
	case '{':
		endc = '}'
	default:
		return nil, i.Unexpected()
	}

	l := i.Loc()

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	if i.IsSingle(endc) {
		// $arr[] — empty offset. Only valid in write context.
		// We compile it and defer the read-context check to runtime
		// since write context is determined by the parent expression.
		v = &runArrayAccess{value: v, offset: nil, l: l}
		return v, nil
	}
	c.backup()

	// don't really need this loop anymore?
	offt, err := compileExpr(nil, c)
	if err != nil {
		return nil, err
	}

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}

	if !i.IsSingle(endc) {
		return nil, i.Unexpected()
	}

	v = &runArrayAccess{value: v, offset: offt, l: l}

	return v, nil
}

// checkReadonlyIndirectModification walks an expression chain to find if it
// resolves through a readonly object property. Returns an error if an indirect
// modification of a readonly property would occur.
func checkReadonlyIndirectModification(ctx phpv.Context, expr phpv.Runnable) error {
	for {
		switch inner := expr.(type) {
		case *runObjectVar:
			objVal, objErr := inner.ref.Run(ctx)
			if objErr != nil || objVal == nil || objVal.GetType() != phpv.ZtObject {
				return nil
			}
			obj, objOk := objVal.Value().(*phpobj.ZObject)
			if !objOk {
				return nil
			}
			propName := inner.varName
			if obj.IsReadonlyProperty(propName) {
				return phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("Cannot indirectly modify readonly property %s::$%s", obj.GetClass().GetName(), propName))
			}
			return nil
		case *runArrayAccess:
			expr = inner.value
			continue
		default:
			return nil
		}
	}
}
