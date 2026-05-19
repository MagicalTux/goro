package compiler

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/KarpelesLab/goro/core/logopt"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
	"github.com/KarpelesLab/goro/core/tokenizer"
)

// IsLeadingNumeric checks if a string starts with a digit, decimal point, or +-digit/+-dot.
// Used to distinguish "foo" (TypeError) from "0foo" or ".5foo" (warning) for string offsets
// and arithmetic operations. PHP considers strings like ".1", "-.1" as leading numeric.
func IsLeadingNumeric(s string) bool {
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
	// Accept digit or decimal point (e.g. ".1", "-.5")
	return (s[i] >= '0' && s[i] <= '9') || s[i] == '.'
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
	array := phpv.NewZArrayTracked(ctx.Global().MemMgrTracker())

	for _, e := range a.e {
		if e.spread {
			v, err := e.v.Run(ctx)
			if err != nil {
				return nil, err
			}
			if err := SpreadIntoArray(ctx, array, v); err != nil {
				return nil, err
			}
			continue
		}

		var k, v *phpv.ZVal

		if e.k != nil {
			k, err = e.k.Run(ctx)
			if err != nil {
				return nil, err
			}
			// Validate key type: objects and arrays cannot be used as array keys
			if k.GetType() == phpv.ZtObject {
				objName := "object"
				if obj, ok := k.Value().(phpv.ZObject); ok {
					objName = string(obj.GetClass().GetName())
				}
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
					fmt.Sprintf("Cannot access offset of type %s on array", objName))
			}
			if k.GetType() == phpv.ZtArray {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "Cannot access offset of type array on array")
			}
			// PHP 8.1: Deprecation for float keys that lose precision
			if k.GetType() == phpv.ZtFloat {
				if _, err := phpv.FloatToIntImplicit(ctx, k.Value().(phpv.ZFloat)); err != nil {
					return nil, err
				}
			}
			// PHP 8.1: Deprecation for null used as array key
			if k.GetType() == phpv.ZtNull {
				if err := ctx.Deprecated("Using null as an array offset is deprecated, use an empty string instead", logopt.NoFuncName(true)); err != nil {
					return nil, err
				}
			}
			// Resource used as array key: emit warning and cast to integer
			if k.GetType() == phpv.ZtResource {
				if r, ok := k.Value().(phpv.Resource); ok {
					id := r.GetResourceID()
					if err := ctx.Warn("Resource ID#%d used as offset, casting to integer (%d)", id, id); err != nil {
						return nil, err
					}
					k = phpv.ZInt(id).ZVal()
				}
			}
		}
		v, err = e.v.Run(ctx)
		if err != nil {
			return nil, err
		}

		// Preserve references (e.g., array(&$a)) by not calling v.ZVal()
		// which would unwrap the reference. Pass the ref ZVal directly.
		var setErr error
		if v.IsRef() {
			setErr = array.OffsetSet(ctx, k, v)
		} else {
			setErr = array.OffsetSet(ctx, k, v.ZVal())
		}
		if setErr != nil {
			if setErr == phpv.ErrNextElementOccupied {
				return nil, phpobj.ThrowError(ctx, phpobj.Error, setErr.Error())
			}
			return nil, setErr
		}
	}

	return array.ZVal(), nil
}

// SpreadIntoArray appends the spread source `v` into the in-progress
// array `array`, matching PHP 8.1+ semantics: string keys preserved
// (numeric-string keys re-indexed), int keys re-indexed, Traversable
// objects iterated through Iterator/IteratorAggregate. Used by both
// AST runArray and the VM's OP_ARRAY_SPREAD_APPEND.
func SpreadIntoArray(ctx phpv.Context, array *phpv.ZArray, v *phpv.ZVal) error {
	switch v.GetType() {
	case phpv.ZtArray:
		src := v.AsArray(ctx)
		for k, val := range src.Iterate(ctx) {
			if k.GetType() == phpv.ZtString {
				array.OffsetSet(ctx, k, val.Dup())
			} else {
				if err := array.OffsetSet(ctx, nil, val.Dup()); err != nil {
					return phpobj.ThrowError(ctx, phpobj.Error, err.Error())
				}
			}
		}
		return nil
	case phpv.ZtObject:
		obj, ok := v.Value().(*phpobj.ZObject)
		if !ok {
			return phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Only arrays and Traversables can be unpacked, %s given", "object"))
		}
		if obj.GetClass().Implements(phpobj.IteratorAggregate) {
			iterResult, iterErr := obj.CallMethod(ctx, "getIterator")
			if iterErr == nil && iterResult != nil && iterResult.GetType() == phpv.ZtObject {
				if iterObj, ok := iterResult.Value().(*phpobj.ZObject); ok && iterObj.GetClass().Implements(phpobj.Iterator) {
					obj = iterObj
				}
			}
		}
		if !obj.GetClass().Implements(phpobj.Iterator) {
			typeName := string(obj.GetClass().GetName())
			return phpobj.ThrowError(ctx, phpobj.Error,
				fmt.Sprintf("Only arrays and Traversables can be unpacked, %s given", typeName))
		}
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
			if key.GetType() != phpv.ZtInt && key.GetType() != phpv.ZtString {
				return phpobj.ThrowError(ctx, phpobj.Error,
					"Keys must be of type int|string during array unpacking")
			}
			value, verr := obj.CallMethod(ctx, "current")
			if verr != nil {
				break
			}
			if key.GetType() == phpv.ZtString {
				keyStr := string(key.AsString(ctx))
				if isNumericString(keyStr) {
					array.OffsetSet(ctx, nil, value.Dup())
				} else {
					array.OffsetSet(ctx, key, value.Dup())
				}
			} else {
				array.OffsetSet(ctx, nil, value.Dup())
			}
			obj.CallMethod(ctx, "next")
		}
		return nil
	default:
		return phpobj.ThrowError(ctx, phpobj.Error,
			fmt.Sprintf("Only arrays and Traversables can be unpacked, %s given", v.GetType().TypeName()))
	}
}

func (a *runArray) Loc() *phpv.Loc {
	return a.l
}

func (a runArray) Dump(w io.Writer) error {
	_, err := w.Write([]byte{'['})
	if err != nil {
		return err
	}
	for i, s := range a.e {
		if i > 0 {
			if _, err = w.Write([]byte(", ")); err != nil {
				return err
			}
		}
		if s.k != nil {
			err = s.k.Dump(w)
			if err != nil {
				return err
			}
			if _, err = w.Write([]byte(" => ")); err != nil {
				return err
			}
		}
		if s.v != nil {
			err = s.v.Dump(w)
			if err != nil {
				return err
			}
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
	nullChain    bool // propagate null from inner nullsafe chain

	// Set by Run() to indicate the container was an ArrayAccess object.
	// Used by runOperator to emit "Indirect modification" notices for compound ops.
	lastContainerIsOverloaded bool
	lastContainerClassName    string
	// True when the ArrayAccess object's offsetGet method returns by reference (&offsetGet).
	// When true, indirect modifications go through the reference and have effect,
	// so the "Indirect modification has no effect" notice should be suppressed.
	lastContainerOffsetGetReturnsRef bool
	// True when the container was a string. Used by runVariableRef to detect
	// "Cannot create references to/from string offsets".
	lastContainerWasString bool

	// PrepareWrite caching
	prepared     bool
	cachedOffset *phpv.ZVal

	// Compound assignment caching: during .= += etc., cache the container
	// from the read phase so WriteValue doesn't re-evaluate the chain.
	compoundCache    bool      // set by runOperator to enable caching
	cachedContainer  *phpv.ZVal // cached result of ac.value.Run(ctx) from Run()

	// Compound offset caching: for compound ops, cache the offset evaluated
	// during Read so WriteValue doesn't re-evaluate it (avoiding duplicate warnings).
	compoundCachedOffset    *phpv.ZVal
	compoundOffsetFromCache bool // set when getArrayOffset used compound cache

	// readKeyWasUndefined is set when an undefined array key was read during
	// a compound assignment (e.g. $a['b'] += 1 where 'b' doesn't exist).
	// If the error handler then creates the key, the write-back is skipped
	// to match PHP behavior where the error handler's value wins.
	readKeyWasUndefined bool

	// returnedLiveRef is set by Run() when the returned value is a "live reference" —
	// a ZVal pointer that directly references storage in a hash table. When true,
	// modifications through CastTo/Set on the returned ZVal are visible to the
	// original container, so WriteValue can skip write-back after auto-vivification.
	// This is set when: (1) offsetGet returns a by-ref value from ArrayAccess, or
	// (2) an array append in write context returns a reference to the new element.
	returnedLiveRef bool

	// cachedAppendRef caches the result of offsetGet(null) for ArrayAccess objects
	// with offset==nil in write context. This prevents double offsetGet calls when
	// WriteValue re-runs the inner expression during write-back.
	cachedAppendRef *phpv.ZVal
}

func (r *runArrayAccess) isNullSafeChain() bool { return r.nullChain }

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
	// Set writeContext to suppress "Undefined array key" warnings during the check
	if inner, ok := ac.value.(*runArrayAccess); ok {
		inner.writeContext = true
		defer func() { inner.writeContext = false }()
	}
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
	ac.lastContainerOffsetGetReturnsRef = false
	ac.returnedLiveRef = false

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

	// Propagate live reference tracking from inner expression
	if inner, ok := ac.value.(*runArrayAccess); ok && inner.returnedLiveRef {
		ac.returnedLiveRef = true
	}

	// Nullsafe chain propagation: if the value is null and we're in a chain, return null
	if ac.nullChain && v.GetType() == phpv.ZtNull {
		return phpv.ZNULL.ZVal(), nil
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
			// Only count as write context if this array access is the
			// LHS (target) of the assignment, not the RHS (value).
			write = t.opD != nil && t.opD.write && isDescendantOf(ac, t.a)
		case *runArrayAccess, *runnableForeach, *runDestructure:
			write = true
		}
		if !write {
			// PHP 8: For strings, throw a runtime Error (catchable exception)
			if v.GetType() == phpv.ZtString {
				return nil, phpobj.ThrowError(ctx, phpobj.Error, "[] operator not supported for strings")
			}
			// For objects implementing ArrayAccess, let offsetGet handle the null key
			// (e.g. SplFixedArray throws "[] operator not supported for SplFixedArray")
			if v.GetType() == phpv.ZtArray {
				// PHP 8: reading from an array with [] (no index) throws a catchable Error
				return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot use [] for reading")
			} else if v.GetType() != phpv.ZtObject {
				// For null/undefined/other types: plain fatal error
				return nil, &phpv.PhpError{
					Err:  fmt.Errorf("Cannot use [] for reading"),
					Code: phpv.E_ERROR,
					Loc:  ac.l,
				}
			}
		}
	}

	// Track whether container was a string, for "Cannot use string offset as an array" detection.
	ac.lastContainerWasString = (v.GetType() == phpv.ZtString)

	switch v.GetType() {
	case phpv.ZtString:
		// PHP 8: [] operator (append) is not supported for strings.
		if ac.offset == nil {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "[] operator not supported for strings")
		}
		// PHP 8: compound assignment operators (+=, -=, .=, etc.) are not allowed on string offsets.
		// Check this BEFORE evaluating the offset to prevent spurious "Uninitialized string offset" warnings.
		if op, ok := ac.Parent.(*runOperator); ok && op.opD != nil && op.opD.write && op.opD.op != nil && op.a == ac {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot use assign-op operators with string offsets")
		}
		// When writing through a string access ($str[0][1] = 'a'), throw "Cannot use string offset as an array".
		// This is detected in WriteValue when inner is a string access; mark container as string here.
		if ac.writeContext {
			// In write context on a string container, return the container itself so the outer
			// WriteValue can detect that this is a string and throw the appropriate error.
			return v, nil
		}
	case phpv.ZtArray:
	case phpv.ZtObject:
		// In constant expression context (class constant/property default evaluation),
		// [] on objects is not allowed.
		if ctx.Global().GetCompilingClass() != nil {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot use [] on objects in constant expression")
		}
		// Track if container is an ArrayAccess object (not a plain array)
		ac.lastContainerIsOverloaded = true
		obj := v.AsObject(ctx)
		ac.lastContainerClassName = string(obj.GetClass().GetName())
		// Check if offsetGet returns by reference (&offsetGet)
		if zo, ok := obj.(*phpobj.ZObject); ok {
			ac.lastContainerOffsetGetReturnsRef = zo.OffsetGetReturnsByRef()
		}
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
			// If v is a non-nil live reference in write context (e.g., from inner
			// ArrayAccess offsetGet or array append), auto-vivify it in place.
			// CastTo modifies through the Go pointer, so the original hash table
			// entry is updated. Then fall through to the offset handling below.
			if v != nil && ac.returnedLiveRef && ac.offset == nil {
				err = v.CastTo(ctx, phpv.ZtArray)
				if err != nil {
					return nil, err
				}
				break // continue to offset==nil handling below
			}
			return phpv.ZNULL.ZVal(), nil
		}
	case phpv.ZtBool:
		// Check for compound write context ($bool[x] += val)
		isCompoundBoolWrite := false
		if !ac.writeContext {
			if op, ok := ac.Parent.(*runOperator); ok && op.opD != nil && op.opD.write && op.opD.op != nil && op.a == ac {
				isCompoundBoolWrite = true
			}
		}
		if !ac.writeContext && !isCompoundBoolWrite {
			boolName := "true"
			if !bool(v.AsBool(ctx)) {
				boolName = "false"
			}
			if err := ctx.Warn("Trying to access array offset on %s", boolName, logopt.NoFuncName(true)); err != nil {
				return nil, err
			}
			return phpv.ZNULL.ZVal(), nil
		}
		// In write context (including compound), true cannot be used as array
		if bool(v.AsBool(ctx)) {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot use a scalar value as an array")
		}
		// false in write context: auto-vivify with deprecation warning
		v, err = v.As(ctx, phpv.ZtArray)
		if err != nil {
			return nil, err
		}
	default:
		// PHP 8: accessing a scalar with array syntax in write context throws Error
		isWriteOp := ac.writeContext
		if !isWriteOp {
			if op, ok := ac.Parent.(*runOperator); ok && op.opD != nil && op.opD.write && op.a == ac {
				// Only treat as write if this array access is the LHS of the assignment
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
		write := ac.writeContext // writeContext from caller (e.g. by-ref function arg)
		switch t := ac.Parent.(type) {
		case *runOperator:
			write = t.opD.write
		case *runArrayAccess, *runnableForeach, *runDestructure:
			write = true
		}

		if !write {
			// PHP 8: For strings, throw a runtime Error (catchable exception)
			if v.GetType() == phpv.ZtString {
				return nil, phpobj.ThrowError(ctx, phpobj.Error, "[] operator not supported for strings")
			}
			// For objects implementing ArrayAccess, call offsetGet(null)
			// which lets the object throw its own error (e.g. SplFixedArray)
			if v.GetType() == phpv.ZtObject {
				array := v.Array()
				if array != nil {
					return array.OffsetGet(ctx, phpv.ZNULL.ZVal())
				}
			}
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use [] for reading"),
				Code: phpv.E_ERROR,
				Loc:  ac.l,
			}
		}
		// PHP 8: For strings, write with [] is not supported
		if v.GetType() == phpv.ZtString {
			return nil, phpobj.ThrowError(ctx, phpobj.Error, "[] operator not supported for strings")
		}
		// For ArrayAccess objects in write context, call offsetGet(NULL) to get
		// a reference to the append position. This is needed for:
		// - compound assignments ($arr[] += 5) to read the current value
		// - multi-dimension writes ($arr[][0] = 2) to get the intermediate container
		// Cache the result to prevent double calls when WriteValue re-evaluates.
		if v.GetType() == phpv.ZtObject {
			array := v.Array()
			if array != nil {
				if ac.cachedAppendRef != nil {
					ref := ac.cachedAppendRef
					ac.cachedAppendRef = nil
					ac.returnedLiveRef = true
					return ref, nil
				}
				ref, err := array.OffsetGet(ctx, phpv.ZNULL.ZVal())
				if err != nil {
					return nil, err
				}
				ac.cachedAppendRef = ref
				ac.returnedLiveRef = true
				return ref, nil
			}
		}
		// For arrays in write context, append a new null element and return a
		// reference to it. Modifications through this reference are visible to
		// the original array (same Go pointer), so outer dimensions can write
		// through it without write-back. This handles return-by-ref ($arr[])
		// and intermediate dimensions in multi-dim writes ($arr[][0] = 2).
		// Cache the result to prevent double appends when PrepareWrite and
		// WriteValue both trigger Run().
		if v.GetType() == phpv.ZtArray {
			if ac.cachedAppendRef != nil {
				ref := ac.cachedAppendRef
				ac.cachedAppendRef = nil
				ac.returnedLiveRef = true
				return ref, nil
			}
			za, ok := v.Array().(*phpv.ZArray)
			if ok {
				newVal := phpv.ZNULL.ZVal()
				if err := za.OffsetSet(ctx, nil, newVal); err != nil {
					return nil, err
				}
				if lastKey, ok2 := za.H().LastIntKey(); ok2 {
					ref, err := za.OffsetGet(ctx, phpv.ZInt(lastKey).ZVal())
					if err != nil {
						return nil, err
					}
					ac.cachedAppendRef = ref
					ac.cachedOffset = phpv.ZInt(lastKey).ZVal()
					ac.returnedLiveRef = true
					return ref, nil
				}
			}
		}
		// If we already appended (from a by-ref WriteValue), use the cached key
		// to look up the newly-created element instead of returning nil.
		if ac.prepared && ac.cachedOffset != nil {
			cachedKey := ac.cachedOffset
			ac.prepared = false
			ac.cachedOffset = nil
			array := v.Array()
			if array != nil {
				return array.OffsetGet(ctx, cachedKey)
			}
		}
		return nil, nil
	}

	offset, err := ac.getArrayOffset(ctx)
	if err != nil {
		return nil, err
	}

	// If the container was mutated by an error handler during offset evaluation
	// (e.g. $a[$c] .= 'x' where $a was auto-vivified to array but error handler
	// reset $a = null), abort gracefully and return null. WriteValue will also
	// bail out when it sees the scalar cachedContainer.
	if ac.cachedContainer != nil {
		vt := v.GetType()
		if vt != phpv.ZtArray && vt != phpv.ZtObject && vt != phpv.ZtString {
			return phpv.ZNULL.ZVal(), nil
		}
	}

	// For compound assignments, cache the offset so WriteValue doesn't re-evaluate it
	if ac.cachedContainer != nil && ac.compoundCachedOffset == nil {
		ac.compoundCachedOffset = offset.Dup()
	}

	// PHP 8.1: Deprecation warning for null array offsets (read).
	if offset.GetType() == phpv.ZtNull && (v.GetType() == phpv.ZtString || v.GetType() == phpv.ZtArray || v.GetType() == phpv.ZtObject) {
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
			if len(s) > 0 && !IsLeadingNumeric(s) {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "Cannot access offset of type string on string")
			}
			// Convert numeric string to int to avoid "Illegal string offset" warning
			if offset.AsString(ctx).IsNumeric() {
				offset = offset.AsInt(ctx).ZVal()
			}
		}
		return v.AsString(ctx).Array().OffsetGet(ctx, offset)
	}

	// Check for invalid offset types on arrays (but not on ArrayAccess objects,
	// where any offset type can be passed to offsetGet/offsetSet)
	if v.GetType() != phpv.ZtObject {
		if offset.GetType() == phpv.ZtObject {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("Cannot access offset of type %s on array", offset.Value().(phpv.ZObject).GetClass().GetName()))
		}
		if offset.GetType() == phpv.ZtArray {
			return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "Cannot access offset of type array on array")
		}
	}

	array := v.Array()
	if array == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, fmt.Sprintf("Cannot use object of type %s as array", v.GetType()))
	}

	// In write context (e.g. $b[0][0] = $b), force COW separation on the container
	// array before reading elements. This prevents cycles when the assigned value
	// is a COW copy of the same array: without separation, the inner element
	// is shared between the original and the copy, so writing to it creates a cycle.
	// PHP C handles this via refcount-based separation (SEPARATE_ZVAL_IF_NOT_REF).
	if ac.writeContext {
		if za, ok := array.(*phpv.ZArray); ok {
			za.SeparateCow()
		}
	}

	// Use OffsetGetWarn for ZArray to produce "Undefined array key" warnings
	// but not when this access is part of a write chain (auto-vivification)
	if !ac.writeContext {
		if za, ok := array.(*phpv.ZArray); ok {
			// For compound assignment (cachedContainer is set), track whether the
			// key is undefined before the error handler runs. If error handler creates
			// the key, the write-back in WriteValue will be skipped (PHP behavior).
			if ac.cachedContainer != nil {
				_, keyExists, _ := za.OffsetCheck(ctx, offset)
				if !keyExists {
					ac.readKeyWasUndefined = true
				}
			}
			return za.OffsetGetWarn(ctx, offset)
		}
	}
	return array.OffsetGet(ctx, offset)
}

func (a *runArrayAccess) IsCompoundWritable() {}

// LastContainerWasOverloaded implements phpv.OverloadedContainerChecker.
// Returns true if the most recent Run() call found that the container was
// an ArrayAccess object (overloaded container).
func (a *runArrayAccess) LastContainerWasOverloaded() bool {
	return a.lastContainerIsOverloaded
}

// LastContainerWasString implements phpv.StringContainerChecker.
// Returns true if the most recent Run() call found that the container was
// a string (string offset access). Used to detect "Cannot create references
// to/from string offsets".
func (a *runArrayAccess) LastContainerWasString() bool {
	return a.lastContainerWasString
}

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
		// If an error handler changed the container to a scalar (e.g. $my_var[0] .= "xyz"
		// where $my_var was null, auto-vivified to array, then error handler set $my_var = 0),
		// abort the write silently to match PHP behavior (the write is lost, handler wins).
		if v != nil {
			vt := v.GetType()
			if vt != phpv.ZtArray && vt != phpv.ZtObject && vt != phpv.ZtNull && vt != phpv.ZtString {
				ac.readKeyWasUndefined = false
				return nil
			}
		}
		// If the key was undefined during read but the error handler created it,
		// abort the write so the error handler's value wins (PHP behavior: the
		// write goes to an orphaned zval in PHP C, so the array retains the
		// error handler's value).
		if ac.readKeyWasUndefined && v != nil && v.GetType() == phpv.ZtArray {
			ac.readKeyWasUndefined = false
			// Use the compoundCachedOffset (without consuming it) to check existence
			var checkOffset *phpv.ZVal
			if ac.compoundCachedOffset != nil {
				checkOffset = ac.compoundCachedOffset
			} else if ac.cachedOffset != nil {
				checkOffset = ac.cachedOffset
			}
			if checkOffset != nil {
				za, ok := v.Array().(*phpv.ZArray)
				if ok {
					_, exists, _ := za.OffsetCheck(ctx, checkOffset)
					if exists {
						// Key was created by error handler - abort write
						// (clear the cached offsets too to prevent re-use)
						ac.compoundCachedOffset = nil
						ac.cachedOffset = nil
						return nil
					}
				}
			}
		} else {
			ac.readKeyWasUndefined = false
		}
	} else {
		// Suppress undefined key/property warnings for inner accesses in write context
		if inner, ok := ac.value.(*runArrayAccess); ok {
			inner.writeContext = true
			defer func() { inner.writeContext = false }()
		}
		if inner, ok := ac.value.(*runObjectVar); ok {
			inner.writeContext = true
			defer func() { inner.writeContext = false }()
			// For unset($a->b->c['d']) where $a->b is null, mark the object chain
			// with unsetChain so that hitting a null/non-object returns NULL silently
			// instead of throwing "Attempt to modify property".
			if value == nil {
				inner.setUnsetChain(true)
				defer inner.setUnsetChain(false)
			}
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
	// Also skip the notice if offsetGet returns by reference (&offsetGet),
	// since modifications through the reference are effective.
	if inner, ok := ac.value.(*runArrayAccess); ok && inner.lastContainerIsOverloaded && v.GetType() != phpv.ZtObject && !inner.lastContainerOffsetGetReturnsRef {
		return ctx.Notice("Indirect modification of overloaded element of %s has no effect", inner.lastContainerClassName, logopt.Data{Loc: ac.l, NoFuncName: true})
	}

	// PHP 8.1: Cannot modify readonly property.
	// If the container expression chain resolves through a readonly property
	// access ($obj->prop[...] = val or $obj->prop[0][...] = val), block it.
	if err := checkReadonlyIndirectModification(ctx, ac.value); err != nil {
		return err
	}

	// Handle unset ($a[x] = nil means unset)
	if value == nil {
		if ac.offset == nil {
			return &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use [] for unsetting"),
				Code: phpv.E_ERROR,
				Loc:  ac.l,
			}
		}
		// Cannot use string offset as an array (unset on nested string access)
		if v != nil && v.GetType() == phpv.ZtString {
			return phpobj.ThrowError(ctx, phpobj.Error, "Cannot use string offset as an array")
		}
		array := v.Array()
		if array == nil {
			return nil
		}
		offset, err := ac.getArrayOffset(ctx)
		if err != nil {
			return err
		}
		return array.OffsetUnset(ctx, offset)
	}

	// Handle nil v (e.g., from $arr[] in write context where the element doesn't exist yet)
	if v == nil {
		v = phpv.ZNULL.ZVal()
	}

	switch v.GetType() {
	case phpv.ZtString:
		// If the container expression is itself a string array access ($str[0][1] = val),
		// this is "Cannot use string offset as an array" - not a simple string offset write.
		if _, ok := ac.value.(*runArrayAccess); ok {
			return phpobj.ThrowError(ctx, phpobj.Error, "Cannot use string offset as an array")
		}
		return ac.writeValueToString(ctx, value)

	case phpv.ZtArray:
	case phpv.ZtObject:
	case phpv.ZtNull:
		// null can be auto-vivified to array
		err = v.CastTo(ctx, phpv.ZtArray)
		if err != nil {
			return err
		}
		// Skip write-back when the value is a live reference from an inner
		// ArrayAccess offsetGet or array append. CastTo already modified the
		// storage through the Go pointer, so write-back would cause duplicate
		// entries (e.g., double offsetSet calls or double array appends).
		innerHasLiveRef := false
		if innerAA, ok := ac.value.(*runArrayAccess); ok && innerAA.returnedLiveRef {
			innerHasLiveRef = true
		}
		if !innerHasLiveRef {
			if wr, ok := ac.value.(phpv.Writable); ok {
				wr.WriteValue(ctx, v)
			}
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
		// For compound assignments ($arr[] .= "x"), Run() already appended a
		// null element and cached its key. Write to that element instead of
		// appending another one.
		if ac.cachedOffset != nil {
			offset := ac.cachedOffset
			ac.cachedOffset = nil
			ac.prepared = false
			return array.OffsetSet(ctx, offset, value)
		}
		// append
		if err := array.OffsetSet(ctx, nil, value); err != nil {
			if err == phpv.ErrNextElementOccupied {
				return phpobj.ThrowError(ctx, phpobj.Error, err.Error())
			}
			return err
		}
		return nil
	}

	offset, err := ac.getArrayOffset(ctx)
	if err != nil {
		return err
	}

	// Check for invalid offset types on arrays (but not on ArrayAccess objects)
	if v.GetType() != phpv.ZtObject {
		if offset.GetType() == phpv.ZtObject {
			return phpobj.ThrowError(ctx, phpobj.TypeError,
				fmt.Sprintf("Cannot access offset of type %s on array", offset.Value().(phpv.ZObject).GetClass().GetName()))
		}
		if offset.GetType() == phpv.ZtArray {
			return phpobj.ThrowError(ctx, phpobj.TypeError, "Cannot access offset of type array on array")
		}
	}

	// PHP 8.1: Deprecation warning for null array offsets (write)
	// Skip if offset was cached from compound assignment read phase (warning already emitted)
	if offset.GetType() == phpv.ZtNull && v.GetType() != phpv.ZtObject && !ac.compoundOffsetFromCache {
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
		return phpobj.ThrowError(ctx, phpobj.Error, "[] operator not supported for strings")
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
	// PHP 8: array offsets on strings are not allowed
	if offset.GetType() == phpv.ZtArray {
		return phpobj.ThrowError(ctx, phpobj.TypeError, "Cannot access offset of type array on string")
	}
	// PHP 8: completely non-numeric string offsets on strings throw TypeError
	if offset.GetType() == phpv.ZtString {
		s := strings.TrimSpace(string(offset.AsString(ctx)))
		if len(s) > 0 && !IsLeadingNumeric(s) {
			return phpobj.ThrowError(ctx, phpobj.TypeError, "Cannot access offset of type string on string")
		}
	}

	if phpv.IsNull(offset) {
		return errors.New("[] operator not supported for string")
	}

	// Check for negative offsets that are out of bounds on the string.
	// When this happens, PHP warns and the assignment expression evaluates to NULL.
	if offset.GetType() == phpv.ZtInt {
		i := int(offset.Value().(phpv.ZInt))
		str := v.AsString(ctx)
		if i < 0 && i+len(str) < 0 {
			ctx.Warn("Illegal string offset %d", i, logopt.NoFuncName(true))
			// Set value to NULL to reflect the failed assignment
			value.Set(phpv.ZNULL.ZVal())
			return nil
		}
	}

	// PHP: when assigning to a string offset, only the first byte of the
	// value is used. If the value is a multi-character string, warn.
	// Use As() instead of AsString() so that __toString() exceptions propagate.
	valZVal, err := value.As(ctx, phpv.ZtString)
	if err != nil {
		return err
	}
	valStr := valZVal.Value().(phpv.ZString)
	if len(valStr) == 0 {
		return phpobj.ThrowError(ctx, phpobj.Error, "Cannot assign an empty string to a string offset")
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
	} else {
		// For [] (append), pre-check if the array would overflow. PHP evaluates
		// the LHS target before the RHS expression, so the overflow error must
		// fire before "Only variables should be assigned by reference" notices.
		// Set writeContext on the inner access to suppress "Undefined array key"
		// and "Undefined property" warnings — we're about to write to this location, not read it.
		if inner, ok := ac.value.(*runArrayAccess); ok {
			inner.writeContext = true
			defer func() { inner.writeContext = false }()
		}
		if inner, ok := ac.value.(*runObjectVar); ok {
			inner.writeContext = true
			defer func() { inner.writeContext = false }()
		}
		v, err := ac.value.Run(ctx)
		if err != nil {
			return err
		}
		if v != nil && v.GetType() == phpv.ZtArray {
			if za, ok := v.Array().(*phpv.ZArray); ok {
				if za.WouldOverflow() {
					return phpobj.ThrowError(ctx, phpobj.Error, phpv.ErrNextElementOccupied.Error())
				}
				// Cache the container to avoid re-evaluating it in WriteValue
				ac.cachedContainer = v
			}
		}
	}
	return nil
}

// NormalizeArrayOffset coerces an array offset value to the form
// that's safe to pass to ZArray.OffsetUnset / OffsetSet — int for
// resource/float/bool, string/int passthrough, deprecation warning
// for null (which becomes ""), and TypeError for object/array keys.
// Shared by the AST runArrayAccess.getArrayOffset and the VM's
// native unset/set dim path.
func NormalizeArrayOffset(ctx phpv.Context, offset *phpv.ZVal) (*phpv.ZVal, error) {
	if offset == nil {
		return offset, nil
	}
	switch offset.GetType() {
	case phpv.ZtResource:
		if r, ok := offset.Value().(phpv.Resource); ok {
			id := r.GetResourceID()
			if err := ctx.Warn("Resource ID#%d used as offset, casting to integer (%d)", id, id); err != nil {
				return nil, err
			}
		}
		return offset.As(ctx, phpv.ZtInt)
	case phpv.ZtFloat:
		return offset.As(ctx, phpv.ZtInt)
	case phpv.ZtBool:
		return offset.As(ctx, phpv.ZtInt)
	case phpv.ZtString, phpv.ZtInt, phpv.ZtNull:
		return offset, nil
	}
	return offset, nil
}

// UnsetArrayDim removes container[key] in place. Mirrors the
// `value == nil` branch of runArrayAccess.WriteValue: refuses string
// containers (`Cannot use string offset as an array`), no-ops on nil
// containers, and otherwise delegates to ZArray.OffsetUnset which
// fires destructors for object elements. Used by the VM's
// OP_UNSET_DIM.
func UnsetArrayDim(ctx phpv.Context, container, key *phpv.ZVal) error {
	if container != nil && container.GetType() == phpv.ZtString {
		return phpobj.ThrowError(ctx, phpobj.Error, "Cannot use string offset as an array")
	}
	if container == nil {
		return nil
	}
	array := container.Array()
	if array == nil {
		return nil
	}
	off, err := NormalizeArrayOffset(ctx, key)
	if err != nil {
		return err
	}
	return array.OffsetUnset(ctx, off.Value())
}

func (ac *runArrayAccess) getArrayOffset(ctx phpv.Context) (*phpv.ZVal, error) {
	var offset *phpv.ZVal
	var err error
	ac.compoundOffsetFromCache = false
	if ac.compoundCachedOffset != nil {
		// Use compound-cached offset from the read phase of a compound assignment
		offset = ac.compoundCachedOffset
		ac.compoundCachedOffset = nil
		ac.compoundOffsetFromCache = true
	} else if ac.prepared {
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
	case phpv.ZtResource:
		if r, ok := offset.Value().(phpv.Resource); ok {
			id := r.GetResourceID()
			if err := ctx.Warn("Resource ID#%d used as offset, casting to integer (%d)", id, id); err != nil {
				return nil, err
			}
		}
		offset, err = offset.As(ctx, phpv.ZtInt)
		if err != nil {
			return nil, err
		}
	case phpv.ZtFloat:
		offset, err = offset.As(ctx, phpv.ZtInt)
		if err != nil {
			return nil, err
		}
	case phpv.ZtBool:
		// Bool is silently coerced to int: true→1, false→0
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

	var lastCommaLoc *phpv.Loc
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
			errLoc := i.Loc()
			if lastCommaLoc != nil {
				errLoc = lastCommaLoc
			}
			return nil, &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use empty array elements in arrays"),
				Code: phpv.E_COMPILE_ERROR,
				Loc:  errLoc,
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
				loc := i.Loc()
				lastCommaLoc = loc
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
			loc := i.Loc()
			lastCommaLoc = loc
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
			loc := i.Loc()
			lastCommaLoc = loc
			continue
		}

		if i.IsSingle(array_type) {
			break
		}
		return nil, i.Unexpected()
	}

	return res, nil
}

// checkEmptySubscriptRead checks an expression for nil-offset array access in
// read context (e.g. $b = $a[]) and returns a compile-time E_ERROR if found.
// inWriteContext should be true when the expression is the LHS of an assignment.
func checkEmptySubscriptRead(r phpv.Runnable, inWriteContext bool) error {
	if r == nil {
		return nil
	}
	switch t := r.(type) {
	case *runArray:
		// Check element expressions inside an array literal
		for _, e := range t.e {
			if e.k != nil {
				if err := checkEmptySubscriptRead(e.k, false); err != nil {
					return err
				}
			}
			if err := checkEmptySubscriptRead(e.v, false); err != nil {
				return err
			}
		}
	case *runArrayAccess:
		if t.offset == nil && !inWriteContext {
			return &phpv.PhpError{
				Err:  fmt.Errorf("Cannot use [] for reading"),
				Code: phpv.E_ERROR,
				Loc:  t.l,
			}
		}
		// The value (container) is recursed. Only pass inWriteContext down when the
		// value is itself an array access (to allow nil-offsets in write chains
		// like $a[][][0] = 1). For other expressions (variables, function calls, etc.),
		// the inWriteContext check is not needed at this level.
		innerWriteCtx := false
		if _, isAA := t.value.(*runArrayAccess); isAA {
			innerWriteCtx = inWriteContext
		}
		if err := checkEmptySubscriptRead(t.value, innerWriteCtx); err != nil {
			return err
		}
		if err := checkEmptySubscriptRead(t.offset, false); err != nil {
			return err
		}
	case *runOperator:
		// ++/-- (inc/dec) are write operations on their operand; treat both
		// prefix (r.b) and postfix (r.a) as being in write context so that
		// $str[]-- is not flagged as "Cannot use [] for reading" at compile time
		// (it should throw "[] operator not supported for strings" at runtime).
		isIncDec := t.op == tokenizer.T_INC || t.op == tokenizer.T_DEC
		if isIncDec {
			if err := checkEmptySubscriptRead(t.a, true); err != nil {
				return err
			}
			if err := checkEmptySubscriptRead(t.b, true); err != nil {
				return err
			}
		} else if t.opD != nil && t.opD.write {
			// LHS of write operator is in write context.
			// First check if the direct LHS is a subscript of a function call result.
			if ac, ok := t.a.(*runArrayAccess); ok && ac.offset != nil {
				switch ac.value.(type) {
				case *runnableFunctionCall, *runnableFunctionCallRef:
					return &phpv.PhpError{
						Err:  fmt.Errorf("Cannot use result of built-in function in write context"),
						Code: phpv.E_ERROR,
						Loc:  ac.l,
					}
				}
			}
			if err := checkEmptySubscriptRead(t.a, true); err != nil {
				return err
			}
			// RHS of write operator is in read context
			if err := checkEmptySubscriptRead(t.b, false); err != nil {
				return err
			}
		} else {
			if err := checkEmptySubscriptRead(t.a, false); err != nil {
				return err
			}
			if err := checkEmptySubscriptRead(t.b, false); err != nil {
				return err
			}
		}
	}
	return nil
}

// isDescendantOf checks whether node is the same as root or is reachable
// via the children of root. Used to determine if an array access node is
// on the LHS (write side) of an assignment operator.
func isDescendantOf(node phpv.Runnable, root phpv.Runnable) bool {
	if node == root {
		return true
	}
	for _, c := range GetChildren(root) {
		if c != nil && isDescendantOf(node, c) {
			return true
		}
	}
	return false
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

	// Check if the value being indexed is part of a nullsafe chain
	chainProp := isNullSafeChainRef(v)

	i, err = c.NextItem()
	if err != nil {
		return nil, err
	}
	if i.IsSingle(endc) {
		// $arr[] — empty offset. Only valid in write context.
		// We compile it and defer the read-context check to runtime
		// since write context is determined by the parent expression.
		v = &runArrayAccess{value: v, offset: nil, l: l, nullChain: chainProp}
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

	v = &runArrayAccess{value: v, offset: offt, l: l, nullChain: chainProp}

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
			// If varName starts with '$', it's a variable-named property ($obj->$var).
			// Look up the variable to get the actual property name.
			if len(propName) > 0 && propName[0] == '$' {
				varLookup, lookupErr := ctx.OffsetGet(ctx, propName[1:].ZVal())
				if lookupErr != nil || varLookup == nil {
					return nil
				}
				propName = varLookup.AsString(ctx)
			}
			if obj.IsReadonlyProperty(propName) {
				return phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("Cannot modify readonly property %s::$%s", obj.GetClass().GetName(), propName))
			}
			// Check asymmetric visibility for indirect modification
			if err := checkAsymmetricVisibilityIndirect(ctx, obj, propName); err != nil {
				return err
			}
			return nil
		case *runObjectDynVar:
			// Dynamic property access: $obj->{expr}[] = val
			objVal, objErr := inner.ref.Run(ctx)
			if objErr != nil || objVal == nil || objVal.GetType() != phpv.ZtObject {
				return nil
			}
			obj, objOk := objVal.Value().(*phpobj.ZObject)
			if !objOk {
				return nil
			}
			nameVal, nameErr := inner.nameExpr.Run(ctx)
			if nameErr != nil || nameVal == nil {
				return nil
			}
			propName := nameVal.AsString(ctx)
			if obj.IsReadonlyProperty(propName) {
				return phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("Cannot modify readonly property %s::$%s", obj.GetClass().GetName(), propName))
			}
			// Check asymmetric visibility for indirect modification
			if err := checkAsymmetricVisibilityIndirect(ctx, obj, propName); err != nil {
				return err
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

// checkAsymmetricVisibilityIndirect checks if an indirect modification (e.g., $obj->prop[] = x,
// $ref = &$obj->prop, ++$obj->prop) violates asymmetric set-visibility constraints.
// Returns an error if the current scope cannot write to the property.
func checkAsymmetricVisibilityIndirect(ctx phpv.Context, obj *phpobj.ZObject, propName phpv.ZString) error {
	class := obj.GetClass().(*phpobj.ZClass)
	for cur := class; cur != nil; cur = cur.Extends {
		for _, prop := range cur.Props {
			if prop.VarName != propName || prop.Modifiers.IsStatic() {
				continue
			}
			if prop.SetModifiers == 0 {
				return nil // no asymmetric visibility
			}
			callerClass := ctx.Class()
			if prop.SetModifiers.IsPrivate() {
				if callerClass != nil && callerClass.GetName() == cur.GetName() {
					return nil
				}
				return phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("Cannot indirectly modify private(set) property %s::$%s from %s",
						cur.GetName(), propName, phpobj.ScopeName(callerClass)))
			}
			if prop.SetModifiers.IsProtected() {
				if callerClass != nil && (callerClass.InstanceOf(cur) || cur.InstanceOf(callerClass)) {
					return nil
				}
				return phpobj.ThrowError(ctx, phpobj.Error,
					fmt.Sprintf("Cannot indirectly modify protected(set) property %s::$%s from %s",
						cur.GetName(), propName, phpobj.ScopeName(callerClass)))
			}
			return nil
		}
	}
	return nil
}
