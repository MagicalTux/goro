package spl

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// ============================================================================
// IteratorIterator - wraps a Traversable/Iterator
// ============================================================================

type iteratorIteratorData struct {
	inner *phpobj.ZObject
}

func (d *iteratorIteratorData) Clone() any {
	return &iteratorIteratorData{inner: d.inner}
}

func getIteratorIteratorData(o *phpobj.ZObject, class *phpobj.ZClass) *iteratorIteratorData {
	d := o.GetOpaque(class)
	if d == nil {
		return nil
	}
	return d.(*iteratorIteratorData)
}

func initIteratorIterator() {
	IteratorIteratorClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) > 2 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						fmt.Sprintf("IteratorIterator::__construct() expects at most 2 arguments, %d given", len(args)))
				}
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "IteratorIterator::__construct(): Argument #1 ($iterator) must be of type Traversable")
				}
				inner, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "IteratorIterator::__construct(): Argument #1 ($iterator) must be of type Traversable")
				}
				// If it's an IteratorAggregate, get the real iterator
				if inner.GetClass().Implements(phpobj.IteratorAggregate) && !inner.GetClass().Implements(phpobj.Iterator) {
					iterResult, err := inner.CallMethod(ctx, "getIterator")
					if err != nil {
						return nil, err
					}
					if iterResult != nil && iterResult.GetType() == phpv.ZtObject {
						if io, ok := iterResult.Value().(*phpobj.ZObject); ok {
							inner = io
						}
					}
				}
				o.SetOpaque(IteratorIteratorClass, &iteratorIteratorData{inner: inner})
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getIteratorIteratorData(o, IteratorIteratorClass)
				if d == nil {
					return nil, nil
				}
				_, err := d.inner.CallMethod(ctx, "rewind")
				return nil, err
			}),
		},
		"current": {
			Name: "current",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getIteratorIteratorData(o, IteratorIteratorClass)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return d.inner.CallMethod(ctx, "current")
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getIteratorIteratorData(o, IteratorIteratorClass)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.inner.CallMethod(ctx, "key")
			}),
		},
		"next": {
			Name: "next",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getIteratorIteratorData(o, IteratorIteratorClass)
				if d == nil {
					return nil, nil
				}
				_, err := d.inner.CallMethod(ctx, "next")
				return nil, err
			}),
		},
		"valid": {
			Name: "valid",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getIteratorIteratorData(o, IteratorIteratorClass)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				v, err := d.inner.CallMethod(ctx, "valid")
				if err != nil {
					return phpv.ZFalse.ZVal(), err
				}
				return v, nil
			}),
		},
		"getinneriterator": {
			Name: "getInnerIterator",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getIteratorIteratorData(o, IteratorIteratorClass)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.inner.ZVal(), nil
			}),
		},
		"__call": {
			Name: "__call",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getIteratorIteratorData(o, IteratorIteratorClass)
				if d == nil || len(args) < 2 {
					return phpv.ZNULL.ZVal(), nil
				}
				methodName := string(args[0].AsString(ctx))
				callArgs := args[1].AsArray(ctx)
				var fwdArgs []*phpv.ZVal
				if callArgs != nil {
					for _, v := range callArgs.Iterate(ctx) {
						fwdArgs = append(fwdArgs, v)
					}
				}
				result, err := d.inner.CallMethod(ctx, methodName, fwdArgs...)
				if err != nil {
					errStr := err.Error()
					if strings.Contains(errStr, "Call to undefined method") {
						return nil, phpobj.ThrowError(ctx, phpobj.Error,
							fmt.Sprintf("Call to undefined method %s::%s()", o.GetClass().GetName(), methodName))
					}
					return nil, err
				}
				return result, nil
			}),
		},
	}
}

var IteratorIteratorClass = &phpobj.ZClass{
	Name:            "IteratorIterator",
	Implementations: []*phpobj.ZClass{OuterIterator},
}

// ============================================================================
// LimitIterator
// ============================================================================

type limitIteratorData struct {
	inner  *phpobj.ZObject
	offset int
	limit  int
	pos    int
}

func (d *limitIteratorData) Clone() any {
	return &limitIteratorData{inner: d.inner, offset: d.offset, limit: d.limit, pos: d.pos}
}

func getLimitIteratorData(o *phpobj.ZObject) *limitIteratorData {
	d := o.GetOpaque(LimitIteratorClass)
	if d == nil {
		return nil
	}
	return d.(*limitIteratorData)
}

func initLimitIterator() {
	LimitIteratorClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "LimitIterator::__construct() expects at least 1 argument, 0 given")
				}
				if args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "LimitIterator::__construct(): Argument #1 ($iterator) must be of type Iterator")
				}
				inner, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "LimitIterator::__construct(): Argument #1 ($iterator) must be of type Iterator")
				}
				offset := 0
				limit := -1
				if len(args) > 1 {
					offset = int(args[1].AsInt(ctx))
				}
				if len(args) > 2 {
					limit = int(args[2].AsInt(ctx))
				}
				if offset < 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "LimitIterator::__construct(): Argument #2 ($offset) must be greater than or equal to 0")
				}
				if limit < -1 {
					return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "LimitIterator::__construct(): Argument #3 ($limit) must be greater than or equal to -1")
				}
				o.SetOpaque(LimitIteratorClass, &limitIteratorData{
					inner:  inner,
					offset: offset,
					limit:  limit,
					pos:    0,
				})
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getLimitIteratorData(o)
				if d == nil {
					return nil, nil
				}
				_, err := d.inner.CallMethod(ctx, "rewind")
				if err != nil {
					return nil, err
				}
				d.pos = 0
				// Seek to offset using the common seek logic
				return nil, limitIteratorSeekTo(ctx, d, d.offset)
			}),
		},
		"current": {
			Name: "current",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getLimitIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return d.inner.CallMethod(ctx, "current")
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getLimitIteratorData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.inner.CallMethod(ctx, "key")
			}),
		},
		"next": {
			Name: "next",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getLimitIteratorData(o)
				if d == nil {
					return nil, nil
				}
				_, err := d.inner.CallMethod(ctx, "next")
				if err != nil {
					return nil, err
				}
				d.pos++
				return nil, nil
			}),
		},
		"valid": {
			Name: "valid",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getLimitIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				if d.pos < d.offset {
					return phpv.ZFalse.ZVal(), nil
				}
				if d.limit >= 0 && (d.pos-d.offset) >= d.limit {
					return phpv.ZFalse.ZVal(), nil
				}
				v, err := d.inner.CallMethod(ctx, "valid")
				if err != nil {
					return phpv.ZFalse.ZVal(), err
				}
				return v, nil
			}),
		},
		"getinneriterator": {
			Name: "getInnerIterator",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getLimitIteratorData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.inner.ZVal(), nil
			}),
		},
		"getposition": {
			Name: "getPosition",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getLimitIteratorData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return phpv.ZInt(d.pos).ZVal(), nil
			}),
		},
		"seek": {
			Name: "seek",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getLimitIteratorData(o)
				if d == nil || len(args) == 0 {
					return nil, nil
				}
				position := int(args[0].AsInt(ctx))
				if position < d.offset {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfBoundsException, fmt.Sprintf("Cannot seek to %d which is below the offset %d", position, d.offset))
				}
				if d.limit >= 0 && position >= d.offset+d.limit {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfBoundsException, fmt.Sprintf("Cannot seek to %d which is behind offset %d plus count %d", position, d.offset, d.limit))
				}
				// Rewind and advance to position
				_, err := d.inner.CallMethod(ctx, "rewind")
				if err != nil {
					return nil, err
				}
				d.pos = 0
				return nil, limitIteratorSeekTo(ctx, d, position)
			}),
		},
		"__call": {
			Name: "__call",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getLimitIteratorData(o)
				if d == nil || len(args) < 2 {
					return phpv.ZNULL.ZVal(), nil
				}
				methodName := string(args[0].AsString(ctx))
				callArgs := args[1].AsArray(ctx)
				var fwdArgs []*phpv.ZVal
				if callArgs != nil {
					for _, v := range callArgs.Iterate(ctx) {
						fwdArgs = append(fwdArgs, v)
					}
				}
				result, err := d.inner.CallMethod(ctx, methodName, fwdArgs...)
				if err != nil {
					errStr := err.Error()
					if strings.Contains(errStr, "Call to undefined method") {
						return nil, phpobj.ThrowError(ctx, phpobj.Error,
							fmt.Sprintf("Call to undefined method %s::%s()", o.GetClass().GetName(), methodName))
					}
					return nil, err
				}
				return result, nil
			}),
		},
	}
}

// limitIteratorSeekTo seeks the inner iterator to the given position.
// It advances from current position d.pos to target, calling next/valid on inner.
// After reaching target, it calls valid one final time (matching PHP behavior).
func limitIteratorSeekTo(ctx phpv.Context, d *limitIteratorData, target int) error {
	for d.pos < target {
		v, err := d.inner.CallMethod(ctx, "valid")
		if err != nil {
			return err
		}
		if !bool(v.AsBool(ctx)) {
			return phpobj.ThrowError(ctx, phpobj.OutOfBoundsException,
				fmt.Sprintf("Seek position %d is out of range", target))
		}
		_, err = d.inner.CallMethod(ctx, "next")
		if err != nil {
			return err
		}
		d.pos++
	}
	// Final valid check at the target position (matches PHP behavior)
	d.inner.CallMethod(ctx, "valid")
	return nil
}

var LimitIteratorClass = &phpobj.ZClass{
	Name:            "LimitIterator",
	Implementations: []*phpobj.ZClass{OuterIterator},
}

// ============================================================================
// CachingIterator
// ============================================================================

const (
	cachingIteratorCallToString        = 1
	cachingIteratorCatchGetChild       = 16
	cachingIteratorToStringUseKey      = 2
	cachingIteratorToStringUseCurrent  = 4
	cachingIteratorToStringUseInner    = 8
	cachingIteratorFullCache           = 256
)

type cachingIteratorData struct {
	inner      *phpobj.ZObject
	flags      int
	currentVal *phpv.ZVal
	currentKey *phpv.ZVal
	hasNext    bool
	started    bool
	cache      *phpv.ZArray
}

func (d *cachingIteratorData) Clone() any {
	return &cachingIteratorData{
		inner:      d.inner,
		flags:      d.flags,
		currentVal: d.currentVal,
		currentKey: d.currentKey,
		hasNext:    d.hasNext,
		started:    d.started,
		cache:      d.cache,
	}
}

func getCachingIteratorData(o *phpobj.ZObject) *cachingIteratorData {
	d := o.GetOpaque(CachingIteratorClass)
	if d == nil {
		return nil
	}
	return d.(*cachingIteratorData)
}

func initCachingIterator() {
	CachingIteratorClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						fmt.Sprintf("%s::__construct() expects at least 1 argument, 0 given", o.GetClass().GetName()))
				}
				if args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "CachingIterator::__construct(): Argument #1 ($iterator) must be of type Iterator")
				}
				inner, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "CachingIterator::__construct(): Argument #1 ($iterator) must be of type Iterator")
				}
				flags := cachingIteratorCallToString
				if len(args) > 1 {
					flags = int(args[1].AsInt(ctx))
				}
				// Validate that only one toString flag is set
				toStringFlags := flags & (cachingIteratorCallToString | cachingIteratorToStringUseKey | cachingIteratorToStringUseCurrent | cachingIteratorToStringUseInner)
				bitCount := 0
				for tf := toStringFlags; tf != 0; tf &= tf - 1 {
					bitCount++
				}
				if bitCount > 1 {
					return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
						"CachingIterator::__construct(): Argument #2 ($flags) must contain only one of CachingIterator::CALL_TOSTRING, CachingIterator::TOSTRING_USE_KEY, CachingIterator::TOSTRING_USE_CURRENT, or CachingIterator::TOSTRING_USE_INNER")
				}
				o.SetOpaque(CachingIteratorClass, &cachingIteratorData{
					inner:   inner,
					flags:   flags,
					hasNext: false,
					started: false,
					cache:   phpv.NewZArray(),
				})
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getCachingIteratorData(o)
				if d == nil {
					return nil, nil
				}
				_, err := d.inner.CallMethod(ctx, "rewind")
				if err != nil {
					return nil, err
				}
				d.started = true
				d.cache = phpv.NewZArray()
				// Fetch first element
				v, err := d.inner.CallMethod(ctx, "valid")
				if err != nil {
					return nil, err
				}
				if bool(v.AsBool(ctx)) {
					d.currentVal, _ = d.inner.CallMethod(ctx, "current")
					d.currentKey, _ = d.inner.CallMethod(ctx, "key")
					// Store in cache if FULL_CACHE
					if d.flags&cachingIteratorFullCache != 0 && d.currentKey != nil {
						d.cache.OffsetSet(ctx, d.currentKey.Value(), d.currentVal)
					}
					// Advance inner to look ahead
					_, err = d.inner.CallMethod(ctx, "next")
					if err != nil {
						return nil, err
					}
					vn, _ := d.inner.CallMethod(ctx, "valid")
					d.hasNext = bool(vn.AsBool(ctx))
				} else {
					d.currentVal = nil
					d.currentKey = nil
					d.hasNext = false
				}
				return nil, nil
			}),
		},
		"current": {
			Name: "current",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getCachingIteratorData(o)
				if d == nil || d.currentVal == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return d.currentVal, nil
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getCachingIteratorData(o)
				if d == nil || d.currentKey == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.currentKey, nil
			}),
		},
		"next": {
			Name: "next",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getCachingIteratorData(o)
				if d == nil {
					return nil, nil
				}
				v, err := d.inner.CallMethod(ctx, "valid")
				if err != nil {
					d.currentVal = nil
					d.currentKey = nil
					d.hasNext = false
					return nil, err
				}
				if bool(v.AsBool(ctx)) {
					d.currentVal, _ = d.inner.CallMethod(ctx, "current")
					d.currentKey, _ = d.inner.CallMethod(ctx, "key")
					// Store in cache if FULL_CACHE
					if d.flags&cachingIteratorFullCache != 0 && d.currentKey != nil {
						d.cache.OffsetSet(ctx, d.currentKey.Value(), d.currentVal)
					}
					_, err = d.inner.CallMethod(ctx, "next")
					if err != nil {
						d.hasNext = false
						return nil, err
					}
					vn, _ := d.inner.CallMethod(ctx, "valid")
					d.hasNext = bool(vn.AsBool(ctx))
				} else {
					d.currentVal = nil
					d.currentKey = nil
					d.hasNext = false
				}
				return nil, nil
			}),
		},
		"valid": {
			Name: "valid",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getCachingIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return phpv.ZBool(d.currentVal != nil).ZVal(), nil
			}),
		},
		"hasnext": {
			Name: "hasNext",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getCachingIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return phpv.ZBool(d.hasNext).ZVal(), nil
			}),
		},
		"getinneriterator": {
			Name: "getInnerIterator",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getCachingIteratorData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.inner.ZVal(), nil
			}),
		},
		"getflags": {
			Name: "getFlags",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getCachingIteratorData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return phpv.ZInt(d.flags).ZVal(), nil
			}),
		},
		"setflags": {
			Name: "setFlags",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getCachingIteratorData(o)
				if d == nil {
					return nil, nil
				}
				if len(args) > 0 {
					newFlags := int(args[0].AsInt(ctx))
					// Validate that only one toString flag is set
					toStringFlags := newFlags & (cachingIteratorCallToString | cachingIteratorToStringUseKey | cachingIteratorToStringUseCurrent | cachingIteratorToStringUseInner)
					bitCount := 0
					for tf := toStringFlags; tf != 0; tf &= tf - 1 {
						bitCount++
					}
					if bitCount > 1 {
						return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
							"CachingIterator::setFlags(): Argument #1 ($flags) must contain only one of CachingIterator::CALL_TOSTRING, CachingIterator::TOSTRING_USE_KEY, CachingIterator::TOSTRING_USE_CURRENT, or CachingIterator::TOSTRING_USE_INNER")
					}
					// Cannot unset CALL_TOSTRING if it was set
					if d.flags&cachingIteratorCallToString != 0 && newFlags&cachingIteratorCallToString == 0 {
						return nil, phpobj.ThrowError(ctx, phpobj.InvalidArgumentException,
							"Unsetting flag CALL_TO_STRING is not possible")
					}
					// Cannot unset TOSTRING_USE_INNER if it was set
					if d.flags&cachingIteratorToStringUseInner != 0 && newFlags&cachingIteratorToStringUseInner == 0 {
						return nil, phpobj.ThrowError(ctx, phpobj.InvalidArgumentException,
							"Unsetting flag TOSTRING_USE_INNER is not possible")
					}
					d.flags = newFlags
				}
				return nil, nil
			}),
		},
		"__tostring": {
			Name: "__toString",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getCachingIteratorData(o)
				if d == nil {
					return phpv.ZString("").ZVal(), nil
				}
				// Check if any toString flag is set
				toStringFlags := d.flags & (cachingIteratorCallToString | cachingIteratorToStringUseKey | cachingIteratorToStringUseCurrent | cachingIteratorToStringUseInner)
				if toStringFlags == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.BadMethodCallException,
						fmt.Sprintf("%s does not fetch string value (see CachingIterator::__construct)", o.GetClass().GetName()))
				}
				if d.currentVal == nil {
					return phpv.ZString("").ZVal(), nil
				}
				if d.flags&cachingIteratorToStringUseKey != 0 && d.currentKey != nil {
					return phpv.ZString(d.currentKey.AsString(ctx)).ZVal(), nil
				}
				if d.flags&cachingIteratorToStringUseCurrent != 0 && d.currentVal != nil {
					return phpv.ZString(d.currentVal.AsString(ctx)).ZVal(), nil
				}
				if d.flags&cachingIteratorToStringUseInner != 0 {
					// Call __toString on the inner iterator
					result, err := d.inner.CallMethod(ctx, "__toString")
					if err != nil {
						return nil, err
					}
					if result != nil {
						return phpv.ZString(result.AsString(ctx)).ZVal(), nil
					}
					return phpv.ZString("").ZVal(), nil
				}
				// CALL_TOSTRING: call __toString on the current value
				if d.flags&cachingIteratorCallToString != 0 {
					return phpv.ZString(d.currentVal.AsString(ctx)).ZVal(), nil
				}
				return phpv.ZString(d.currentVal.AsString(ctx)).ZVal(), nil
			}),
		},
		"count": {
			Name: "count",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getCachingIteratorData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				if d.flags&cachingIteratorFullCache == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.BadMethodCallException, fmt.Sprintf("%s does not use a full cache (see CachingIterator::__construct)", o.GetClass().GetName()))
				}
				return d.cache.Count(ctx).ZVal(), nil
			}),
		},
		"getcache": {
			Name: "getCache",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getCachingIteratorData(o)
				if d == nil {
					return phpv.NewZArray().ZVal(), nil
				}
				if d.flags&cachingIteratorFullCache == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.BadMethodCallException, fmt.Sprintf("%s does not use a full cache (see CachingIterator::__construct)", o.GetClass().GetName()))
				}
				return d.cache.ZVal(), nil
			}),
		},
		"offsetget": {
			Name: "offsetGet",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getCachingIteratorData(o)
				if d == nil || len(args) == 0 {
					return phpv.ZNULL.ZVal(), nil
				}
				if d.flags&cachingIteratorFullCache == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.BadMethodCallException, fmt.Sprintf("%s does not use a full cache (see CachingIterator::__construct)", o.GetClass().GetName()))
				}
				return d.cache.OffsetGet(ctx, args[0].Value())
			}),
		},
		"offsetset": {
			Name: "offsetSet",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getCachingIteratorData(o)
				if d == nil || len(args) < 2 {
					return nil, nil
				}
				if d.flags&cachingIteratorFullCache == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.BadMethodCallException, fmt.Sprintf("%s does not use a full cache (see CachingIterator::__construct)", o.GetClass().GetName()))
				}
				d.cache.OffsetSet(ctx, args[0].Value(), args[1])
				return nil, nil
			}),
		},
		"offsetexists": {
			Name: "offsetExists",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getCachingIteratorData(o)
				if d == nil || len(args) == 0 {
					return phpv.ZFalse.ZVal(), nil
				}
				if d.flags&cachingIteratorFullCache == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.BadMethodCallException, fmt.Sprintf("%s does not use a full cache (see CachingIterator::__construct)", o.GetClass().GetName()))
				}
				exists, _ := d.cache.OffsetExists(ctx, args[0])
				return phpv.ZBool(exists).ZVal(), nil
			}),
		},
		"offsetunset": {
			Name: "offsetUnset",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getCachingIteratorData(o)
				if d == nil || len(args) == 0 {
					return nil, nil
				}
				if d.flags&cachingIteratorFullCache == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.BadMethodCallException, fmt.Sprintf("%s does not use a full cache (see CachingIterator::__construct)", o.GetClass().GetName()))
				}
				d.cache.OffsetUnset(ctx, args[0].Value())
				return nil, nil
			}),
		},
	}
}

var CachingIteratorClass = &phpobj.ZClass{
	Name:            "CachingIterator",
	Implementations: []*phpobj.ZClass{OuterIterator, Countable, phpobj.ArrayAccess},
	Const: map[phpv.ZString]*phpv.ZClassConst{
		"CALL_TOSTRING":        {Value: phpv.ZInt(cachingIteratorCallToString)},
		"CATCH_GET_CHILD":      {Value: phpv.ZInt(cachingIteratorCatchGetChild)},
		"TOSTRING_USE_KEY":     {Value: phpv.ZInt(cachingIteratorToStringUseKey)},
		"TOSTRING_USE_CURRENT": {Value: phpv.ZInt(cachingIteratorToStringUseCurrent)},
		"TOSTRING_USE_INNER":   {Value: phpv.ZInt(cachingIteratorToStringUseInner)},
		"FULL_CACHE":           {Value: phpv.ZInt(cachingIteratorFullCache)},
	},
}

// ============================================================================
// AppendIterator
// ============================================================================

type appendIteratorData struct {
	iterators []*phpobj.ZObject
	current   int
}

func (d *appendIteratorData) Clone() any {
	nd := &appendIteratorData{
		iterators: make([]*phpobj.ZObject, len(d.iterators)),
		current:   d.current,
	}
	copy(nd.iterators, d.iterators)
	return nd
}

func getAppendIteratorData(o *phpobj.ZObject) *appendIteratorData {
	d := o.GetOpaque(AppendIteratorClass)
	if d == nil {
		return nil
	}
	return d.(*appendIteratorData)
}

func initAppendIterator() {
	AppendIteratorClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) > 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.ArgumentCountError,
						fmt.Sprintf("AppendIterator::__construct() expects exactly 0 arguments, %d given", len(args)))
				}
				o.SetOpaque(AppendIteratorClass, &appendIteratorData{
					iterators: nil,
					current:   0,
				})
				return nil, nil
			}),
		},
		"append": {
			Name: "append",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getAppendIteratorData(o)
				if d == nil {
					return nil, nil
				}
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
					typeName := "null"
					if len(args) > 0 && args[0] != nil {
						typeName = args[0].GetType().TypeName()
					}
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("AppendIterator::append(): Argument #1 ($iterator) must be of type Iterator, %s given", typeName))
				}
				inner, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "AppendIterator::append(): Argument #1 ($iterator) must be of type Iterator, object given")
				}
				d.iterators = append(d.iterators, inner)
				// Rewind the newly appended iterator
				inner.CallMethod(ctx, "rewind")
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getAppendIteratorData(o)
				if d == nil {
					return nil, nil
				}
				d.current = 0
				if len(d.iterators) > 0 {
					_, err := d.iterators[0].CallMethod(ctx, "rewind")
					return nil, err
				}
				return nil, nil
			}),
		},
		"current": {
			Name: "current",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getAppendIteratorData(o)
				if d == nil || d.current >= len(d.iterators) {
					return phpv.ZFalse.ZVal(), nil
				}
				return d.iterators[d.current].CallMethod(ctx, "current")
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getAppendIteratorData(o)
				if d == nil || d.current >= len(d.iterators) {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.iterators[d.current].CallMethod(ctx, "key")
			}),
		},
		"next": {
			Name: "next",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getAppendIteratorData(o)
				if d == nil || d.current >= len(d.iterators) {
					return nil, nil
				}
				_, err := d.iterators[d.current].CallMethod(ctx, "next")
				if err != nil {
					return nil, err
				}
				// Check if current iterator is exhausted, move to next
				v, err := d.iterators[d.current].CallMethod(ctx, "valid")
				if err != nil {
					return nil, err
				}
				if !bool(v.AsBool(ctx)) {
					d.current++
					if d.current < len(d.iterators) {
						_, err := d.iterators[d.current].CallMethod(ctx, "rewind")
						if err != nil {
							return nil, err
						}
					}
				}
				return nil, nil
			}),
		},
		"valid": {
			Name: "valid",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getAppendIteratorData(o)
				if d == nil || d.current >= len(d.iterators) {
					return phpv.ZFalse.ZVal(), nil
				}
				return d.iterators[d.current].CallMethod(ctx, "valid")
			}),
		},
		"getinneriterator": {
			Name: "getInnerIterator",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getAppendIteratorData(o)
				if d == nil || d.current >= len(d.iterators) {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.iterators[d.current].ZVal(), nil
			}),
		},
		"getiteratorindex": {
			Name: "getIteratorIndex",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getAppendIteratorData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return phpv.ZInt(d.current).ZVal(), nil
			}),
		},
		"getarrayiterator": {
			Name: "getArrayIterator",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getAppendIteratorData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				// Build an array of iterators and wrap in ArrayIterator
				arr := phpv.NewZArray()
				for i, it := range d.iterators {
					arr.OffsetSet(ctx, phpv.ZInt(i), it.ZVal())
				}
				aiObj, err := phpobj.NewZObject(ctx, ArrayIteratorClass, arr.ZVal())
				if err != nil {
					return phpv.ZNULL.ZVal(), err
				}
				return aiObj.ZVal(), nil
			}),
		},
	}
}

var AppendIteratorClass = &phpobj.ZClass{
	Name:            "AppendIterator",
	Implementations: []*phpobj.ZClass{OuterIterator},
}

// ============================================================================
// RegexIterator
// ============================================================================

const (
	regexIteratorMatch    = 0
	regexIteratorGetMatch = 1
	regexIteratorAllMatches = 2
	regexIteratorSplit    = 3
	regexIteratorReplace  = 4

	regexIteratorUseKey = 1
)

type regexIteratorData struct {
	inner         *phpobj.ZObject
	pattern       phpv.ZString
	mode          int
	flags         int
	pregFlags     int
	currentResult *phpv.ZVal // cached result for GET_MATCH, ALL_MATCHES, SPLIT, REPLACE modes
}

func (d *regexIteratorData) Clone() any {
	return &regexIteratorData{
		inner:     d.inner,
		pattern:   d.pattern,
		mode:      d.mode,
		flags:     d.flags,
		pregFlags: d.pregFlags,
	}
}

func getRegexIteratorData(o *phpobj.ZObject) *regexIteratorData {
	d := o.GetOpaque(RegexIteratorClass)
	if d == nil {
		return nil
	}
	return d.(*regexIteratorData)
}

func initRegexIterator() {
	RegexIteratorClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) < 2 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "RegexIterator::__construct() expects at least 2 arguments")
				}
				if args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "RegexIterator::__construct(): Argument #1 ($iterator) must be of type Iterator")
				}
				inner, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "RegexIterator::__construct(): Argument #1 ($iterator) must be of type Iterator")
				}
				pattern := args[1].AsString(ctx)
				mode := regexIteratorMatch
				if len(args) > 2 {
					mode = int(args[2].AsInt(ctx))
				}
				flags := 0
				if len(args) > 3 {
					flags = int(args[3].AsInt(ctx))
				}
				o.SetOpaque(RegexIteratorClass, &regexIteratorData{
					inner:   inner,
					pattern: pattern,
					mode:    mode,
					flags:   flags,
				})
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRegexIteratorData(o)
				if d == nil {
					return nil, nil
				}
				_, err := d.inner.CallMethod(ctx, "rewind")
				if err != nil {
					return nil, err
				}
				// Skip to first matching element
				for {
					v, err := d.inner.CallMethod(ctx, "valid")
					if err != nil || !bool(v.AsBool(ctx)) {
						break
					}
					if d.accept(ctx, o) {
						break
					}
					_, err = d.inner.CallMethod(ctx, "next")
					if err != nil {
						return nil, err
					}
				}
				return nil, nil
			}),
		},
		"current": {
			Name: "current",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRegexIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				// For GET_MATCH, ALL_MATCHES, SPLIT, and REPLACE modes,
				// return the cached result from accept()
				if d.currentResult != nil && d.mode != regexIteratorMatch {
					return d.currentResult, nil
				}
				return d.inner.CallMethod(ctx, "current")
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRegexIteratorData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.inner.CallMethod(ctx, "key")
			}),
		},
		"next": {
			Name: "next",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRegexIteratorData(o)
				if d == nil {
					return nil, nil
				}
				// Advance to next matching element
				for {
					_, err := d.inner.CallMethod(ctx, "next")
					if err != nil {
						return nil, err
					}
					v, err := d.inner.CallMethod(ctx, "valid")
					if err != nil {
						return nil, err
					}
					if !bool(v.AsBool(ctx)) {
						break
					}
					// Check if current element matches the regex
					if d.accept(ctx, o) {
						break
					}
				}
				return nil, nil
			}),
		},
		"valid": {
			Name: "valid",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRegexIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				v, err := d.inner.CallMethod(ctx, "valid")
				if err != nil {
					return phpv.ZFalse.ZVal(), err
				}
				return v, nil
			}),
		},
		"accept": {
			Name: "accept",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRegexIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return phpv.ZBool(d.accept(ctx, o)).ZVal(), nil
			}),
		},
		"getregex": {
			Name: "getRegex",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRegexIteratorData(o)
				if d == nil {
					return phpv.ZString("").ZVal(), nil
				}
				return d.pattern.ZVal(), nil
			}),
		},
		"getmode": {
			Name: "getMode",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRegexIteratorData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return phpv.ZInt(d.mode).ZVal(), nil
			}),
		},
		"getflags": {
			Name: "getFlags",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRegexIteratorData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return phpv.ZInt(d.flags).ZVal(), nil
			}),
		},
		"getinneriterator": {
			Name: "getInnerIterator",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRegexIteratorData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.inner.ZVal(), nil
			}),
		},
		"setmode": {
			Name: "setMode",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRegexIteratorData(o)
				if d == nil || len(args) == 0 {
					return nil, nil
				}
				mode := int(args[0].AsInt(ctx))
				if mode < 0 || mode > 4 {
					return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("RegexIterator::setMode(): Argument #1 ($mode) must be RegexIterator::MATCH, RegexIterator::GET_MATCH, RegexIterator::ALL_MATCHES, RegexIterator::SPLIT, or RegexIterator::REPLACE"))
				}
				d.mode = mode
				return nil, nil
			}),
		},
		"setflags": {
			Name: "setFlags",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRegexIteratorData(o)
				if d == nil || len(args) == 0 {
					return nil, nil
				}
				d.flags = int(args[0].AsInt(ctx))
				return nil, nil
			}),
		},
		"getpregflags": {
			Name: "getPregFlags",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRegexIteratorData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return phpv.ZInt(d.pregFlags).ZVal(), nil
			}),
		},
		"setpregflags": {
			Name: "setPregFlags",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRegexIteratorData(o)
				if d == nil || len(args) == 0 {
					return nil, nil
				}
				d.pregFlags = int(args[0].AsInt(ctx))
				return nil, nil
			}),
		},
	}
}

// parsePCREPattern extracts the Go regexp from a PHP PCRE-style pattern like /foo/i
func parsePCREPattern(pattern string) (*regexp.Regexp, error) {
	if len(pattern) < 2 {
		return nil, nil
	}
	delimiter, dLen := utf8.DecodeRuneInString(pattern)
	pattern = pattern[dLen:]
	endDelimiter := delimiter
	switch delimiter {
	case '(':
		endDelimiter = ')'
	case '{':
		endDelimiter = '}'
	case '[':
		endDelimiter = ']'
	case '<':
		endDelimiter = '>'
	}

	var skip, found bool
	var stack, pos int
	for i, c := range pattern {
		if skip {
			skip = false
			continue
		}
		switch c {
		case '\\':
			skip = true
		case delimiter:
			if delimiter != endDelimiter {
				stack++
				break
			}
			fallthrough
		case endDelimiter:
			if stack > 0 {
				stack--
			} else {
				found = true
				pos = i
			}
		}
		if found {
			break
		}
	}
	if !found {
		return nil, nil
	}

	flags := pattern[pos+utf8.RuneLen(endDelimiter):]
	pattern = pattern[:pos]

	var goFlags strings.Builder
	goFlags.WriteString("(?")
	hasFlags := false
	for _, f := range flags {
		switch f {
		case 'i':
			goFlags.WriteRune('i')
			hasFlags = true
		case 'm':
			goFlags.WriteRune('m')
			hasFlags = true
		case 's':
			goFlags.WriteRune('s')
			hasFlags = true
		case 'U':
			goFlags.WriteRune('U')
			hasFlags = true
		}
	}

	var finalPattern string
	if hasFlags {
		goFlags.WriteRune(')')
		finalPattern = goFlags.String() + pattern
	} else {
		finalPattern = pattern
	}

	return regexp.Compile(finalPattern)
}

// accept checks if the current inner element matches the regex pattern.
// It also stores the match result for use by current() in non-MATCH modes.
// The optional obj parameter is used to read $replacement property for REPLACE mode.
func (d *regexIteratorData) accept(ctx phpv.Context, obj ...*phpobj.ZObject) bool {
	d.currentResult = nil

	// Determine subject based on USE_KEY flag
	var subject string
	if d.flags&regexIteratorUseKey != 0 {
		keyVal, err := d.inner.CallMethod(ctx, "key")
		if err != nil || keyVal == nil {
			return false
		}
		subject = string(keyVal.AsString(ctx))
	} else {
		val, err := d.inner.CallMethod(ctx, "current")
		if err != nil || val == nil {
			return false
		}
		subject = string(val.AsString(ctx))
	}

	re, err := parsePCREPattern(string(d.pattern))
	if err != nil || re == nil {
		return false
	}

	invertMatch := d.flags&2 != 0 // INVERT_MATCH = 2

	switch d.mode {
	case regexIteratorMatch:
		matched := re.MatchString(subject)
		if invertMatch {
			matched = !matched
		}
		return matched

	case regexIteratorGetMatch:
		matches := re.FindStringSubmatch(subject)
		if matches == nil {
			return invertMatch
		}
		if invertMatch {
			return false
		}
		// Build result array
		result := phpv.NewZArray()
		for i, m := range matches {
			result.OffsetSet(ctx, phpv.ZInt(i), phpv.ZString(m).ZVal())
		}
		d.currentResult = result.ZVal()
		return true

	case regexIteratorAllMatches:
		allMatches := re.FindAllStringSubmatch(subject, -1)
		if len(allMatches) == 0 {
			return invertMatch
		}
		if invertMatch {
			return false
		}
		// Build result as array of arrays (transposed: by capture group)
		numGroups := len(allMatches[0])
		result := phpv.NewZArray()
		for g := 0; g < numGroups; g++ {
			groupArr := phpv.NewZArray()
			for _, match := range allMatches {
				if g < len(match) {
					groupArr.OffsetSet(ctx, nil, phpv.ZString(match[g]).ZVal())
				}
			}
			result.OffsetSet(ctx, phpv.ZInt(g), groupArr.ZVal())
		}
		d.currentResult = result.ZVal()
		return true

	case regexIteratorSplit:
		// SPLIT mode: only accept if the regex matches the subject
		matched := re.MatchString(subject)
		if invertMatch {
			matched = !matched
		}
		if !matched {
			return false
		}
		parts := re.Split(subject, -1)
		result := phpv.NewZArray()
		for i, p := range parts {
			result.OffsetSet(ctx, phpv.ZInt(i), phpv.ZString(p).ZVal())
		}
		d.currentResult = result.ZVal()
		return true

	case regexIteratorReplace:
		// Get the replacement string from $replacement property if available
		replacement := ""
		if len(obj) > 0 && obj[0] != nil {
			propVal, err := obj[0].ObjectGet(ctx, phpv.ZString("replacement"))
			if err == nil && propVal != nil && propVal.GetType() != phpv.ZtNull {
				replacement = string(propVal.AsString(ctx))
			}
		}
		replaced := re.ReplaceAllString(subject, replacement)
		d.currentResult = phpv.ZString(replaced).ZVal()
		if invertMatch {
			return !re.MatchString(subject)
		}
		return re.MatchString(subject)

	default:
		matched := re.MatchString(subject)
		if invertMatch {
			matched = !matched
		}
		return matched
	}
}

var RegexIteratorClass = &phpobj.ZClass{
	Name:            "RegexIterator",
	Extends:         nil, // Would extend FilterIterator in full impl
	Implementations: []*phpobj.ZClass{OuterIterator},
	Props: []*phpv.ZClassProp{
		{VarName: "replacement", Modifiers: phpv.ZAttrPublic}, // PHP has public $replacement
	},
	Const: map[phpv.ZString]*phpv.ZClassConst{
		"MATCH":        {Value: phpv.ZInt(regexIteratorMatch)},
		"GET_MATCH":    {Value: phpv.ZInt(regexIteratorGetMatch)},
		"ALL_MATCHES":  {Value: phpv.ZInt(regexIteratorAllMatches)},
		"SPLIT":        {Value: phpv.ZInt(regexIteratorSplit)},
		"REPLACE":      {Value: phpv.ZInt(regexIteratorReplace)},
		"USE_KEY":      {Value: phpv.ZInt(regexIteratorUseKey)},
		"INVERT_MATCH": {Value: phpv.ZInt(2)},
	},
}

// ============================================================================
// RecursiveArrayIterator
// ============================================================================

type recursiveArrayIteratorData struct {
	array *phpv.ZArray
	iter  phpv.ZIterator
	flags phpv.ZInt
}

func (d *recursiveArrayIteratorData) Clone() any {
	return &recursiveArrayIteratorData{
		array: d.array.Dup(),
		iter:  nil,
	}
}

func getRecursiveArrayIteratorData(o *phpobj.ZObject) *recursiveArrayIteratorData {
	d := o.GetOpaque(RecursiveArrayIteratorClass)
	if d == nil {
		return nil
	}
	return d.(*recursiveArrayIteratorData)
}

func initRecursiveArrayIterator() {
	RecursiveArrayIteratorClass.H = &phpv.ZClassHandlers{
		HandleForeachByRef: func(ctx phpv.Context, o phpv.ZObject) (*phpv.ZArray, error) {
			if zo, ok := o.(*phpobj.ZObject); ok {
				// Subclasses that override current() cannot use foreach by reference
				if overridesMethod(zo, RecursiveArrayIteratorClass, "current") {
					return nil, nil
				}
				d := getRecursiveArrayIteratorData(zo)
				if d != nil {
					return d.array, nil
				}
			}
			return nil, nil
		},
	}

	RecursiveArrayIteratorClass.Const = map[phpv.ZString]*phpv.ZClassConst{
		"CHILD_ARRAYS_ONLY": {Value: phpv.ZInt(4)},
		"STD_PROP_LIST":     {Value: phpv.ZInt(1)},
		"ARRAY_AS_PROPS":    {Value: phpv.ZInt(2)},
	}

	RecursiveArrayIteratorClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := &recursiveArrayIteratorData{}
				if len(args) > 0 && args[0] != nil {
					switch args[0].GetType() {
					case phpv.ZtArray:
						d.array = args[0].Value().(*phpv.ZArray).Dup()
					case phpv.ZtObject:
						// Emit deprecation for object backing
						ctx.Deprecated("ArrayIterator::__construct(): Using an object as a backing array for ArrayIterator is deprecated, as it allows violating class constraints and invariants", logopt.NoFuncName(true))
						obj := args[0].Value().(*phpobj.ZObject)
						arr := phpv.NewZArray()
						for prop := range obj.IterProps(ctx) {
							if prop.Modifiers.IsPublic() || (!prop.Modifiers.IsPrivate() && !prop.Modifiers.IsProtected()) {
								v := obj.GetPropValue(prop)
								arr.OffsetSet(ctx, prop.VarName.ZVal(), v)
							}
						}
						d.array = arr
					default:
						d.array = phpv.NewZArray()
					}
				} else {
					d.array = phpv.NewZArray()
				}
				if len(args) > 1 && args[1] != nil {
					d.flags = args[1].AsInt(ctx)
				}
				d.iter = d.array.NewIterator()
				o.SetOpaque(RecursiveArrayIteratorClass, d)
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveArrayIteratorData(o)
				if d == nil {
					return nil, nil
				}
				d.iter.Reset(ctx)
				return nil, nil
			}),
		},
		"current": {
			Name: "current",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveArrayIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return d.iter.Current(ctx)
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveArrayIteratorData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.iter.Key(ctx)
			}),
		},
		"next": {
			Name: "next",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveArrayIteratorData(o)
				if d == nil {
					return nil, nil
				}
				d.iter.Next(ctx)
				return nil, nil
			}),
		},
		"valid": {
			Name: "valid",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveArrayIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return phpv.ZBool(d.iter.Valid(ctx)).ZVal(), nil
			}),
		},
		"haschildren": {
			Name: "hasChildren",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveArrayIteratorData(o)
				if d == nil || !d.iter.Valid(ctx) {
					return phpv.ZFalse.ZVal(), nil
				}
				v, err := d.iter.Current(ctx)
				if err != nil {
					return phpv.ZFalse.ZVal(), nil
				}
				if v.GetType() == phpv.ZtArray {
					return phpv.ZTrue.ZVal(), nil
				}
				// Objects are considered children unless CHILD_ARRAYS_ONLY is set
				if v.GetType() == phpv.ZtObject && d.flags&4 == 0 {
					return phpv.ZTrue.ZVal(), nil
				}
				return phpv.ZFalse.ZVal(), nil
			}),
		},
		"getchildren": {
			Name: "getChildren",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveArrayIteratorData(o)
				if d == nil || !d.iter.Valid(ctx) {
					return phpv.ZNULL.ZVal(), nil
				}
				v, err := d.iter.Current(ctx)
				if err != nil {
					return phpv.ZNULL.ZVal(), nil
				}
				switch v.GetType() {
				case phpv.ZtArray:
					child, err := phpobj.NewZObject(ctx, RecursiveArrayIteratorClass, v, d.flags.ZVal())
					if err != nil {
						return phpv.ZNULL.ZVal(), err
					}
					return child.ZVal(), nil
				case phpv.ZtObject:
					if d.flags&4 == 0 {
						// Wrap object in a new RecursiveArrayIterator
						child, err := phpobj.NewZObject(ctx, RecursiveArrayIteratorClass, v, d.flags.ZVal())
						if err != nil {
							return phpv.ZNULL.ZVal(), err
						}
						return child.ZVal(), nil
					}
					return phpv.ZNULL.ZVal(), nil
				default:
					return phpv.ZNULL.ZVal(), nil
				}
			}),
		},
		"count": {
			Name: "count",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveArrayIteratorData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return d.array.Count(ctx).ZVal(), nil
			}),
		},
		"offsetexists": {
			Name: "offsetExists",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveArrayIteratorData(o)
				if d == nil || len(args) < 1 {
					return phpv.ZFalse.ZVal(), nil
				}
				exists, err := d.array.OffsetExists(ctx, args[0])
				if err != nil {
					return nil, err
				}
				return phpv.ZBool(exists).ZVal(), nil
			}),
		},
		"offsetget": {
			Name: "offsetGet",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveArrayIteratorData(o)
				if d == nil || len(args) < 1 {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.array.OffsetGet(ctx, args[0])
			}),
		},
		"offsetset": {
			Name: "offsetSet",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveArrayIteratorData(o)
				if d == nil || len(args) < 2 {
					return nil, nil
				}
				key := args[0]
				value := args[1]
				if key.GetType() == phpv.ZtNull {
					err := d.array.OffsetSet(ctx, nil, value)
					return nil, err
				}
				err := d.array.OffsetSet(ctx, key, value)
				return nil, err
			}),
		},
		"offsetunset": {
			Name: "offsetUnset",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveArrayIteratorData(o)
				if d == nil || len(args) < 1 {
					return nil, nil
				}
				err := d.array.OffsetUnset(ctx, args[0])
				return nil, err
			}),
		},
		"getarraycopy": {
			Name: "getArrayCopy",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveArrayIteratorData(o)
				if d == nil {
					return phpv.NewZArray().ZVal(), nil
				}
				return d.array.Dup().ZVal(), nil
			}),
		},
		"getflags": {
			Name: "getFlags",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveArrayIteratorData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return d.flags.ZVal(), nil
			}),
		},
		"setflags": {
			Name: "setFlags",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveArrayIteratorData(o)
				if d == nil || len(args) < 1 {
					return nil, nil
				}
				d.flags = args[0].AsInt(ctx)
				return nil, nil
			}),
		},
		"seek": {
			Name: "seek",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveArrayIteratorData(o)
				if d == nil || len(args) < 1 {
					return nil, nil
				}
				position := int(args[0].AsInt(ctx))
				d.iter.Reset(ctx)
				for i := 0; i < position; i++ {
					if !d.iter.Valid(ctx) {
						return nil, phpobj.ThrowError(ctx, phpobj.OutOfBoundsException,
							fmt.Sprintf("Seek position %d is out of range", position))
					}
					d.iter.Next(ctx)
				}
				return nil, nil
			}),
		},
		"append": {
			Name: "append",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveArrayIteratorData(o)
				if d == nil || len(args) < 1 {
					return nil, nil
				}
				d.array.OffsetSet(ctx, nil, args[0])
				return nil, nil
			}),
		},
	}
}

// RecursiveIterator interface
var RecursiveIterator = &phpobj.ZClass{
	Type:    phpv.ZClassTypeInterface,
	Name:    "RecursiveIterator",
	Extends: phpobj.Iterator,
}

var RecursiveArrayIteratorClass = &phpobj.ZClass{
	Name:            "RecursiveArrayIterator",
	Implementations: []*phpobj.ZClass{RecursiveIterator, Countable, phpobj.ArrayAccess},
}

// ============================================================================
// RecursiveIteratorIterator
// ============================================================================

const (
	recursiveIteratorLeavesOnly = 0
	recursiveIteratorSelfFirst  = 1
	recursiveIteratorChildFirst = 2
)

type recursiveIteratorIteratorData struct {
	// Stack of iterators at each depth level
	stack           []*phpobj.ZObject
	mode            int
	depth           int
	maxDepth        int // -1 means no limit
	catchGetChild   bool
	endIterCalled   bool // prevents calling endIteration more than once
	// hasNextAtDepth tracks whether there's a next sibling at each depth level
	// (used by RecursiveTreeIterator for prefix generation)
	hasNextAtDepth []bool
}

func (d *recursiveIteratorIteratorData) Clone() any {
	nd := &recursiveIteratorIteratorData{
		stack:         make([]*phpobj.ZObject, len(d.stack)),
		mode:          d.mode,
		depth:         d.depth,
		maxDepth:      d.maxDepth,
		catchGetChild: d.catchGetChild,
	}
	copy(nd.stack, d.stack)
	if d.hasNextAtDepth != nil {
		nd.hasNextAtDepth = make([]bool, len(d.hasNextAtDepth))
		copy(nd.hasNextAtDepth, d.hasNextAtDepth)
	}
	return nd
}

func getRecursiveIteratorIteratorData(o *phpobj.ZObject) *recursiveIteratorIteratorData {
	d := o.GetOpaque(RecursiveIteratorIteratorClass)
	if d == nil {
		return nil
	}
	return d.(*recursiveIteratorIteratorData)
}

func initRecursiveIteratorIterator() {
	RecursiveIteratorIteratorClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						"RecursiveIteratorIterator::__construct() expects at least 1 argument, 0 given")
				}
				if args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "RecursiveIteratorIterator::__construct(): Argument #1 ($iterator) must be of type RecursiveIterator|IteratorAggregate")
				}
				inner, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "RecursiveIteratorIterator::__construct(): Argument #1 ($iterator) must be of type RecursiveIterator|IteratorAggregate")
				}
				mode := recursiveIteratorLeavesOnly
				if len(args) > 1 {
					mode = int(args[1].AsInt(ctx))
				}
				catchChild := false
				if len(args) > 2 {
					flags := int(args[2].AsInt(ctx))
					catchChild = flags&16 != 0 // CATCH_GET_CHILD = 16
				}
				// If it's an IteratorAggregate, get the real iterator
				if inner.GetClass().Implements(phpobj.IteratorAggregate) && !inner.GetClass().Implements(RecursiveIterator) {
					iterResult, err := inner.CallMethod(ctx, "getIterator")
					if err != nil {
						return nil, err
					}
					if iterResult != nil && iterResult.GetType() == phpv.ZtObject {
						if io, ok := iterResult.Value().(*phpobj.ZObject); ok {
							inner = io
						}
					}
				}
				d := &recursiveIteratorIteratorData{
					stack:         []*phpobj.ZObject{inner},
					mode:          mode,
					depth:         0,
					maxDepth:      -1,
					catchGetChild: catchChild,
				}
				o.SetOpaque(RecursiveIteratorIteratorClass, d)
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveIteratorIteratorData(o)
				if d == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, "Object is not initialized")
				}
				if len(d.stack) == 0 {
					return nil, nil
				}
				// Reset to just the root iterator
				root := d.stack[0]
				d.stack = []*phpobj.ZObject{root}
				d.depth = 0
				d.hasNextAtDepth = nil
				d.endIterCalled = false
				_, err := root.CallMethod(ctx, "rewind")
				if err != nil {
					return nil, err
				}
				// Call beginIteration hook
				o.Unwrap().(*phpobj.ZObject).CallMethod(ctx, "beginIteration")
				// Descend into children if needed
				err = recursiveIteratorDescend(ctx, d, o)
				if err != nil {
					return nil, err
				}
				// Call nextElement hook after first element is ready
				v, _ := o.CallMethod(ctx, "valid")
				if v != nil && bool(v.AsBool(ctx)) {
					_, err = o.Unwrap().(*phpobj.ZObject).CallMethod(ctx, "nextElement")
					if err != nil {
						return nil, err
					}
				}
				return nil, nil
			}),
		},
		"current": {
			Name: "current",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveIteratorIteratorData(o)
				if d == nil || len(d.stack) == 0 {
					return phpv.ZFalse.ZVal(), nil
				}
				top := d.stack[len(d.stack)-1]
				return top.CallMethod(ctx, "current")
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveIteratorIteratorData(o)
				if d == nil || len(d.stack) == 0 {
					return phpv.ZNULL.ZVal(), nil
				}
				top := d.stack[len(d.stack)-1]
				return top.CallMethod(ctx, "key")
			}),
		},
		"next": {
			Name: "next",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveIteratorIteratorData(o)
				if d == nil || len(d.stack) == 0 {
					return nil, nil
				}
				err := recursiveIteratorNext(ctx, d, o)
				if err != nil {
					return nil, err
				}
				// Call nextElement hook after next element is ready
				v, _ := o.CallMethod(ctx, "valid")
				if v != nil && bool(v.AsBool(ctx)) {
					_, err = o.Unwrap().(*phpobj.ZObject).CallMethod(ctx, "nextElement")
					if err != nil {
						return nil, err
					}
				}
				return nil, nil
			}),
		},
		"valid": {
			Name: "valid",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveIteratorIteratorData(o)
				if d == nil || len(d.stack) == 0 {
					return phpv.ZFalse.ZVal(), nil
				}
				top := d.stack[len(d.stack)-1]
				v, err := top.CallMethod(ctx, "valid")
				if err != nil {
					return phpv.ZFalse.ZVal(), err
				}
				if !bool(v.AsBool(ctx)) {
					// Call endIteration hook (only once)
					if !d.endIterCalled {
						d.endIterCalled = true
						o.Unwrap().(*phpobj.ZObject).CallMethod(ctx, "endIteration")
					}
				}
				return v, nil
			}),
		},
		"getdepth": {
			Name: "getDepth",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveIteratorIteratorData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return phpv.ZInt(len(d.stack) - 1).ZVal(), nil
			}),
		},
		"getinneriterator": {
			Name: "getInnerIterator",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveIteratorIteratorData(o)
				if d == nil || len(d.stack) == 0 {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.stack[len(d.stack)-1].ZVal(), nil
			}),
		},
		"getsubiterator": {
			Name: "getSubIterator",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveIteratorIteratorData(o)
				if d == nil || len(d.stack) == 0 {
					return phpv.ZNULL.ZVal(), nil
				}
				level := len(d.stack) - 1
				if len(args) > 0 {
					level = int(args[0].AsInt(ctx))
				}
				if level < 0 || level >= len(d.stack) {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.stack[level].ZVal(), nil
			}),
		},
		"setmaxdepth": {
			Name: "setMaxDepth",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveIteratorIteratorData(o)
				if d == nil {
					return nil, nil
				}
				if len(args) == 0 {
					d.maxDepth = -1
				} else {
					depth := int(args[0].AsInt(ctx))
					if depth < 0 {
						return nil, phpobj.ThrowError(ctx, phpobj.OutOfRangeException, "Parameter max_depth must be >= -1")
					}
					d.maxDepth = depth
				}
				return nil, nil
			}),
		},
		"getmaxdepth": {
			Name: "getMaxDepth",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveIteratorIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				if d.maxDepth < 0 {
					return phpv.ZFalse.ZVal(), nil
				}
				return phpv.ZInt(d.maxDepth).ZVal(), nil
			}),
		},
		"beginiteration": {
			Name: "beginIteration",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				// Default implementation does nothing; subclasses can override
				return nil, nil
			}),
		},
		"enditeration": {
			Name: "endIteration",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return nil, nil
			}),
		},
		"beginchildren": {
			Name: "beginChildren",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return nil, nil
			}),
		},
		"endchildren": {
			Name: "endChildren",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return nil, nil
			}),
		},
		"callhaschildren": {
			Name: "callHasChildren",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveIteratorIteratorData(o)
				if d == nil || len(d.stack) == 0 {
					return phpv.ZFalse.ZVal(), nil
				}
				top := d.stack[len(d.stack)-1]
				return top.CallMethod(ctx, "hasChildren")
			}),
		},
		"callgetchildren": {
			Name: "callGetChildren",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveIteratorIteratorData(o)
				if d == nil || len(d.stack) == 0 {
					return phpv.ZNULL.ZVal(), nil
				}
				top := d.stack[len(d.stack)-1]
				return top.CallMethod(ctx, "getChildren")
			}),
		},
		"nextelement": {
			Name: "nextElement",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return nil, nil
			}),
		},
		"getsubpath": {Name: "getSubPath", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			d := getRecursiveIteratorIteratorData(o)
			if d == nil || len(d.stack) == 0 {
				return phpv.ZStr(""), nil
			}
			top := d.stack[len(d.stack)-1]
			result, err := top.CallMethod(ctx, "getSubPath")
			if err != nil {
				return phpv.ZStr(""), nil
			}
			return result, nil
		})},
		"getsubpathname": {Name: "getSubPathname", Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			d := getRecursiveIteratorIteratorData(o)
			if d == nil || len(d.stack) == 0 {
				return phpv.ZStr(""), nil
			}
			top := d.stack[len(d.stack)-1]
			result, err := top.CallMethod(ctx, "getSubPathname")
			if err != nil {
				return phpv.ZStr(""), nil
			}
			return result, nil
		})},
	}
}

// recursiveIteratorCallHasChildren calls hasChildren through the outer object's callHasChildren,
// handling CATCH_GET_CHILD for error suppression.
func recursiveIteratorCallHasChildren(ctx phpv.Context, d *recursiveIteratorIteratorData, outer *phpobj.ZObject, top *phpobj.ZObject) (bool, error) {
	var hasChildren bool
	var err error
	if outer != nil {
		var result *phpv.ZVal
		result, err = outer.Unwrap().(*phpobj.ZObject).CallMethod(ctx, "callHasChildren")
		if err != nil {
			if d.catchGetChild {
				return false, nil
			}
			return false, err
		}
		hasChildren = bool(result.AsBool(ctx))
	} else {
		var result *phpv.ZVal
		result, err = top.CallMethod(ctx, "hasChildren")
		if err != nil {
			if d.catchGetChild {
				return false, nil
			}
			return false, err
		}
		hasChildren = bool(result.AsBool(ctx))
	}
	return hasChildren, nil
}

// recursiveIteratorCallGetChildren calls getChildren through the outer object's callGetChildren,
// handling CATCH_GET_CHILD for error suppression.
func recursiveIteratorCallGetChildren(ctx phpv.Context, d *recursiveIteratorIteratorData, outer *phpobj.ZObject, top *phpobj.ZObject) (*phpobj.ZObject, error) {
	var childResult *phpv.ZVal
	var err error
	if outer != nil {
		childResult, err = outer.Unwrap().(*phpobj.ZObject).CallMethod(ctx, "callGetChildren")
	} else {
		childResult, err = top.CallMethod(ctx, "getChildren")
	}
	if err != nil {
		if d.catchGetChild {
			return nil, nil
		}
		return nil, err
	}
	if childResult == nil || childResult.GetType() != phpv.ZtObject {
		return nil, nil
	}
	child, ok := childResult.Value().(*phpobj.ZObject)
	if !ok {
		return nil, nil
	}
	return child, nil
}

// recursiveIteratorBeginChildren calls beginChildren hook, handling CATCH_GET_CHILD.
func recursiveIteratorBeginChildren(ctx phpv.Context, d *recursiveIteratorIteratorData, outer *phpobj.ZObject) error {
	if outer != nil {
		_, err := outer.Unwrap().(*phpobj.ZObject).CallMethod(ctx, "beginChildren")
		if err != nil {
			if d.catchGetChild {
				return nil
			}
			return err
		}
	}
	return nil
}

// recursiveIteratorEndChildren calls endChildren hook, handling CATCH_GET_CHILD.
func recursiveIteratorEndChildren(ctx phpv.Context, d *recursiveIteratorIteratorData, outer *phpobj.ZObject) error {
	if outer != nil {
		_, err := outer.Unwrap().(*phpobj.ZObject).CallMethod(ctx, "endChildren")
		if err != nil {
			if d.catchGetChild {
				return nil
			}
			return err
		}
	}
	return nil
}

// recursiveIteratorUpdateHasNext checks whether there's a next sibling after
// the current element at the current depth, and stores it in hasNextAtDepth.
func recursiveIteratorUpdateHasNext(ctx phpv.Context, d *recursiveIteratorIteratorData) {
	depth := len(d.stack) - 1
	// Grow slice if needed
	for len(d.hasNextAtDepth) <= depth {
		d.hasNextAtDepth = append(d.hasNextAtDepth, false)
	}
	// For each depth level, check if there's a next element by peeking
	for i := 0; i <= depth; i++ {
		if i < len(d.stack) {
			_ = d.stack[i]
			// Check if there's a next sibling at this level:
			// this is complex - we'll use a simpler heuristic based on the parent level.
			d.hasNextAtDepth[i] = false // will be set per-element during iteration
		}
	}
}

// recursiveIteratorDescend tries to descend into children of the current element.
// The outer parameter is the RecursiveIteratorIterator object for calling hooks.
func recursiveIteratorDescend(ctx phpv.Context, d *recursiveIteratorIteratorData, outer *phpobj.ZObject) error {
	for {
		top := d.stack[len(d.stack)-1]
		v, err := top.CallMethod(ctx, "valid")
		if err != nil || !bool(v.AsBool(ctx)) {
			return err
		}

		// In SELF_FIRST mode, stop here - the current node should be yielded first.
		// Descending into children happens on next().
		if d.mode == recursiveIteratorSelfFirst && len(d.stack) > 1 {
			return nil
		}

		// Check maxDepth limit
		currentDepth := len(d.stack) - 1
		if d.maxDepth >= 0 && currentDepth >= d.maxDepth {
			return nil // Max depth reached
		}

		// Check if current has children
		hasChildren, err := recursiveIteratorCallHasChildren(ctx, d, outer, top)
		if err != nil {
			return err
		}
		if !hasChildren {
			return nil // Leaf node
		}

		// In SELF_FIRST mode at root level, stop here to yield the parent first
		if d.mode == recursiveIteratorSelfFirst {
			return nil
		}

		// Get children iterator
		child, err := recursiveIteratorCallGetChildren(ctx, d, outer, top)
		if err != nil {
			return err
		}
		if child == nil {
			return nil
		}
		// Rewind the child
		_, err = child.CallMethod(ctx, "rewind")
		if err != nil {
			return err
		}
		d.stack = append(d.stack, child)
		d.depth++
		// Call beginChildren hook on outer
		if err := recursiveIteratorBeginChildren(ctx, d, outer); err != nil {
			return err
		}
	}
}

// recursiveIteratorNext advances the recursive iterator
func recursiveIteratorNext(ctx phpv.Context, d *recursiveIteratorIteratorData, outer *phpobj.ZObject) error {
	if len(d.stack) == 0 {
		return nil
	}

	top := d.stack[len(d.stack)-1]

	// In SELF_FIRST mode, try to descend into children first before advancing
	if d.mode == recursiveIteratorSelfFirst {
		currentDepth := len(d.stack) - 1
		canDescend := d.maxDepth < 0 || currentDepth < d.maxDepth

		if canDescend {
			hasChildren, err := recursiveIteratorCallHasChildren(ctx, d, outer, top)
			if err != nil {
				return err
			}
			if hasChildren {
				child, err := recursiveIteratorCallGetChildren(ctx, d, outer, top)
				if err != nil {
					return err
				}
				if child != nil {
					_, err = child.CallMethod(ctx, "rewind")
					if err == nil {
						d.stack = append(d.stack, child)
						d.depth++
						if err := recursiveIteratorBeginChildren(ctx, d, outer); err != nil {
							return err
						}
						// Check if child is valid
						v, _ := child.CallMethod(ctx, "valid")
						if v != nil && bool(v.AsBool(ctx)) {
							return nil // Stay at the child's first element
						}
						// Child is empty, pop it back
						d.stack = d.stack[:len(d.stack)-1]
						d.depth--
					}
				}
			}
		}
	}

	_, err := top.CallMethod(ctx, "next")
	if err != nil {
		return err
	}

	// Check if current level is valid
	v, err := top.CallMethod(ctx, "valid")
	if err != nil {
		return err
	}
	if bool(v.AsBool(ctx)) {
		// Try to descend into children
		err = recursiveIteratorDescend(ctx, d, outer)
		if err != nil {
			return err
		}
		// Check if we ended up at a valid position after descent
		if len(d.stack) > 0 {
			newTop := d.stack[len(d.stack)-1]
			vv, _ := newTop.CallMethod(ctx, "valid")
			if vv != nil && bool(vv.AsBool(ctx)) {
				return nil // Valid position after descent
			}
		}
		// Descent resulted in invalid position (e.g. all children filtered out),
		// fall through to the "go back up" logic
	}

	// Current level exhausted, go back up
	for len(d.stack) > 1 {
		// Call endChildren hook on outer
		if err := recursiveIteratorEndChildren(ctx, d, outer); err != nil {
			return err
		}
		if len(d.stack) <= 1 {
			break // endChildren may have modified the stack
		}
		d.stack = d.stack[:len(d.stack)-1]
		if d.depth > 0 {
			d.depth--
		}
		if len(d.stack) == 0 {
			break
		}
		top = d.stack[len(d.stack)-1]

		// In CHILD_FIRST mode, we need to yield the parent after children
		if d.mode == recursiveIteratorChildFirst {
			// Check if current position is valid
			v, err = top.CallMethod(ctx, "valid")
			if err != nil {
				return err
			}
			if bool(v.AsBool(ctx)) {
				return nil // Yield the parent
			}
		}

		_, err = top.CallMethod(ctx, "next")
		if err != nil {
			return err
		}
		v, err = top.CallMethod(ctx, "valid")
		if err != nil {
			return err
		}
		if bool(v.AsBool(ctx)) {
			return recursiveIteratorDescend(ctx, d, outer)
		}
	}
	return nil
}

var RecursiveIteratorIteratorClass = &phpobj.ZClass{
	Name:            "RecursiveIteratorIterator",
	Implementations: []*phpobj.ZClass{OuterIterator},
	Const: map[phpv.ZString]*phpv.ZClassConst{
		"LEAVES_ONLY":   {Value: phpv.ZInt(recursiveIteratorLeavesOnly)},
		"SELF_FIRST":    {Value: phpv.ZInt(recursiveIteratorSelfFirst)},
		"CHILD_FIRST":   {Value: phpv.ZInt(recursiveIteratorChildFirst)},
		"CATCH_GET_CHILD": {Value: phpv.ZInt(16)},
	},
}

// ============================================================================
// NoRewindIterator
// ============================================================================

type noRewindIteratorData struct {
	inner *phpobj.ZObject
}

func (d *noRewindIteratorData) Clone() any {
	return &noRewindIteratorData{inner: d.inner}
}

func getNoRewindIteratorData(o *phpobj.ZObject) *noRewindIteratorData {
	d := o.GetOpaque(NoRewindIteratorClass)
	if d == nil {
		return nil
	}
	return d.(*noRewindIteratorData)
}

func initNoRewindIterator() {
	NoRewindIteratorClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "NoRewindIterator::__construct() expects exactly 1 argument, 0 given")
				}
				if args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "NoRewindIterator::__construct(): Argument #1 ($iterator) must be of type Iterator")
				}
				inner, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "NoRewindIterator::__construct(): Argument #1 ($iterator) must be of type Iterator")
				}
				o.SetOpaque(NoRewindIteratorClass, &noRewindIteratorData{inner: inner})
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				// No-op: this is the whole point of NoRewindIterator
				return nil, nil
			}),
		},
		"current": {
			Name: "current",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getNoRewindIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return d.inner.CallMethod(ctx, "current")
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getNoRewindIteratorData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.inner.CallMethod(ctx, "key")
			}),
		},
		"next": {
			Name: "next",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getNoRewindIteratorData(o)
				if d == nil {
					return nil, nil
				}
				_, err := d.inner.CallMethod(ctx, "next")
				return nil, err
			}),
		},
		"valid": {
			Name: "valid",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getNoRewindIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return d.inner.CallMethod(ctx, "valid")
			}),
		},
		"getinneriterator": {
			Name: "getInnerIterator",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getNoRewindIteratorData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.inner.ZVal(), nil
			}),
		},
	}
}

var NoRewindIteratorClass = &phpobj.ZClass{
	Name:            "NoRewindIterator",
	Implementations: []*phpobj.ZClass{OuterIterator},
}

// ============================================================================
// EmptyIterator
// ============================================================================

func initEmptyIterator() {
	EmptyIteratorClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return nil, nil
			}),
		},
		"current": {
			Name: "current",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return nil, phpobj.ThrowError(ctx, phpobj.BadMethodCallException, "Accessing the value of an EmptyIterator")
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return nil, phpobj.ThrowError(ctx, phpobj.BadMethodCallException, "Accessing the key of an EmptyIterator")
			}),
		},
		"next": {
			Name: "next",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return nil, nil
			}),
		},
		"valid": {
			Name: "valid",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.ZFalse.ZVal(), nil
			}),
		},
	}
}

var EmptyIteratorClass = &phpobj.ZClass{
	Name:            "EmptyIterator",
	Implementations: []*phpobj.ZClass{phpobj.Iterator},
}

// ============================================================================
// CallbackFilterIterator
// ============================================================================

type callbackFilterIteratorData struct {
	inner    *phpobj.ZObject
	callback phpv.Callable
}

func (d *callbackFilterIteratorData) Clone() any {
	return &callbackFilterIteratorData{inner: d.inner, callback: d.callback}
}

func getCallbackFilterIteratorData(o *phpobj.ZObject) *callbackFilterIteratorData {
	d := o.GetOpaque(CallbackFilterIteratorClass)
	if d == nil {
		return nil
	}
	return d.(*callbackFilterIteratorData)
}

func initCallbackFilterIterator() {
	CallbackFilterIteratorClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) < 2 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("CallbackFilterIterator::__construct() expects exactly 2 arguments, %d given", len(args)))
				}
				if args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "CallbackFilterIterator::__construct(): Argument #1 ($iterator) must be of type Iterator")
				}
				inner, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "CallbackFilterIterator::__construct(): Argument #1 ($iterator) must be of type Iterator")
				}
				// Resolve the callback (handles arrays like [$obj, 'method'], strings, closures, etc.)
				// If the value is already a Callable (e.g. from getChildren wrapping), use directly
				var callback phpv.Callable
				var err error
				if cb, ok := args[1].Value().(phpv.Callable); ok {
					callback = cb
				} else {
					callback, err = core.SpawnCallable(ctx, args[1])
				}
				if err != nil {
					// Wrap the error in a proper TypeError with function context
					errMsg := err.Error()
					if strings.Contains(errMsg, "array callback must have exactly two") {
						return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
							"CallbackFilterIterator::__construct(): Argument #2 ($callback) must be a valid callback, array callback must have exactly two members")
					}
					if args[1].GetType() == phpv.ZtArray || args[1].GetType() == phpv.ZtString {
						return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
							"CallbackFilterIterator::__construct(): Argument #2 ($callback) must be a valid callback, no array or string given")
					}
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
						"CallbackFilterIterator::__construct(): Argument #2 ($callback) must be a valid callback, no array or string given")
				}
				o.SetOpaque(CallbackFilterIteratorClass, &callbackFilterIteratorData{
					inner:    inner,
					callback: callback,
				})
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getCallbackFilterIteratorData(o)
				if d == nil {
					return nil, nil
				}
				_, err := d.inner.CallMethod(ctx, "rewind")
				if err != nil {
					return nil, err
				}
				// Skip to first accepted element
				for {
					v, err := d.inner.CallMethod(ctx, "valid")
					if err != nil || !bool(v.AsBool(ctx)) {
						break
					}
					accepted, err := callbackFilterAccept(ctx, d)
					if err != nil {
						return nil, err
					}
					if accepted {
						break
					}
					_, err = d.inner.CallMethod(ctx, "next")
					if err != nil {
						return nil, err
					}
				}
				return nil, nil
			}),
		},
		"current": {
			Name: "current",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getCallbackFilterIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return d.inner.CallMethod(ctx, "current")
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getCallbackFilterIteratorData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.inner.CallMethod(ctx, "key")
			}),
		},
		"next": {
			Name: "next",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getCallbackFilterIteratorData(o)
				if d == nil {
					return nil, nil
				}
				for {
					_, err := d.inner.CallMethod(ctx, "next")
					if err != nil {
						return nil, err
					}
					v, err := d.inner.CallMethod(ctx, "valid")
					if err != nil || !bool(v.AsBool(ctx)) {
						break
					}
					accepted, err := callbackFilterAccept(ctx, d)
					if err != nil {
						return nil, err
					}
					if accepted {
						break
					}
				}
				return nil, nil
			}),
		},
		"valid": {
			Name: "valid",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getCallbackFilterIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return d.inner.CallMethod(ctx, "valid")
			}),
		},
		"getinneriterator": {
			Name: "getInnerIterator",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getCallbackFilterIteratorData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.inner.ZVal(), nil
			}),
		},
	}
}

func callbackFilterAccept(ctx phpv.Context, d *callbackFilterIteratorData) (bool, error) {
	current, err := d.inner.CallMethod(ctx, "current")
	if err != nil {
		return false, err
	}
	key, err := d.inner.CallMethod(ctx, "key")
	if err != nil {
		return false, err
	}
	result, err := ctx.CallZVal(ctx, d.callback, []*phpv.ZVal{current, key, d.inner.ZVal()}, nil)
	if err != nil {
		return false, err
	}
	return result != nil && bool(result.AsBool(ctx)), nil
}

var CallbackFilterIteratorClass = &phpobj.ZClass{
	Name:            "CallbackFilterIterator",
	Implementations: []*phpobj.ZClass{OuterIterator},
}

// ============================================================================
// MultipleIterator
// ============================================================================

const (
	multipleIteratorMitNeedAny = 0
	multipleIteratorMitNeedAll = 1
	multipleIteratorMitKeysNumeric  = 0
	multipleIteratorMitKeysAssoc    = 2
)

type multipleIteratorEntry struct {
	iterator *phpobj.ZObject
	info     *phpv.ZVal // associated info (key for ASSOC mode), nil = no info
}

type multipleIteratorData struct {
	entries []multipleIteratorEntry
	flags   int
}

func (d *multipleIteratorData) Clone() any {
	nd := &multipleIteratorData{
		entries: make([]multipleIteratorEntry, len(d.entries)),
		flags:   d.flags,
	}
	copy(nd.entries, d.entries)
	return nd
}

func getMultipleIteratorData(o *phpobj.ZObject) *multipleIteratorData {
	d := o.GetOpaque(MultipleIteratorClass)
	if d == nil {
		return nil
	}
	return d.(*multipleIteratorData)
}

func initMultipleIterator() {
	MultipleIteratorClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				flags := multipleIteratorMitNeedAll
				if len(args) > 0 {
					flags = int(args[0].AsInt(ctx))
				}
				o.SetOpaque(MultipleIteratorClass, &multipleIteratorData{
					flags: flags,
				})
				return nil, nil
			}),
		},
		"attachiterator": {
			Name: "attachIterator",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getMultipleIteratorData(o)
				if d == nil {
					return nil, nil
				}
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "MultipleIterator::attachIterator(): Argument #1 ($iterator) must be of type Iterator")
				}
				inner, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "MultipleIterator::attachIterator(): Argument #1 ($iterator) must be of type Iterator")
				}
				var info *phpv.ZVal
				if len(args) > 1 && args[1] != nil && args[1].GetType() != phpv.ZtNull {
					// Validate info type: must be string or int
					switch args[1].GetType() {
					case phpv.ZtString, phpv.ZtInt:
						info = args[1]
					case phpv.ZtObject:
						return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
							fmt.Sprintf("MultipleIterator::attachIterator(): Argument #2 ($info) must be of type string|int|null, %s given", args[1].Value().(*phpobj.ZObject).GetClass().GetName()))
					default:
						return nil, phpobj.ThrowError(ctx, phpobj.TypeError,
							fmt.Sprintf("MultipleIterator::attachIterator(): Argument #2 ($info) must be of type string|int|null, %s given", args[1].GetType().TypeName()))
					}
					// Check for duplicate info (but not for the same iterator being re-attached)
					for _, e := range d.entries {
						if e.iterator != inner && e.info != nil && info != nil {
							if string(e.info.AsString(ctx)) == string(info.AsString(ctx)) {
								return nil, phpobj.ThrowError(ctx, phpobj.InvalidArgumentException, "Key duplication error")
							}
						}
					}
				}
				// Replace existing entry for the same iterator, or append
				found := false
				for i, e := range d.entries {
					if e.iterator == inner {
						d.entries[i].info = info
						found = true
						break
					}
				}
				if !found {
					d.entries = append(d.entries, multipleIteratorEntry{iterator: inner, info: info})
				}
				return nil, nil
			}),
		},
		"detachiterator": {
			Name: "detachIterator",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getMultipleIteratorData(o)
				if d == nil || len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, nil
				}
				inner, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, nil
				}
				for i, e := range d.entries {
					if e.iterator == inner {
						d.entries = append(d.entries[:i], d.entries[i+1:]...)
						break
					}
				}
				return nil, nil
			}),
		},
		"containsiterator": {
			Name: "containsIterator",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getMultipleIteratorData(o)
				if d == nil || len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return phpv.ZFalse.ZVal(), nil
				}
				inner, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return phpv.ZFalse.ZVal(), nil
				}
				for _, e := range d.entries {
					if e.iterator == inner {
						return phpv.ZTrue.ZVal(), nil
					}
				}
				return phpv.ZFalse.ZVal(), nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getMultipleIteratorData(o)
				if d == nil {
					return nil, nil
				}
				for _, e := range d.entries {
					_, err := e.iterator.CallMethod(ctx, "rewind")
					if err != nil {
						return nil, err
					}
				}
				return nil, nil
			}),
		},
		"current": {
			Name: "current",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getMultipleIteratorData(o)
				if d == nil || len(d.entries) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Called current() on an invalid iterator")
				}
				// Check if any sub-iterator is invalid in MIT_NEED_ALL mode
				if d.flags&multipleIteratorMitNeedAll != 0 {
					for _, e := range d.entries {
						v, err := e.iterator.CallMethod(ctx, "valid")
						if err != nil || !bool(v.AsBool(ctx)) {
							return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Called current() with non valid sub iterator")
						}
					}
				}
				result := phpv.NewZArray()
				useAssoc := d.flags&multipleIteratorMitKeysAssoc != 0
				for i, e := range d.entries {
					v, _ := e.iterator.CallMethod(ctx, "valid")
					if v != nil && bool(v.AsBool(ctx)) {
						cur, err := e.iterator.CallMethod(ctx, "current")
						if err != nil {
							return nil, err
						}
						if useAssoc && e.info != nil {
							result.OffsetSet(ctx, e.info.Value(), cur)
						} else if useAssoc && e.info == nil {
							return nil, phpobj.ThrowError(ctx, phpobj.InvalidArgumentException, "Sub-Iterator is associated with NULL")
						} else {
							result.OffsetSet(ctx, phpv.ZInt(i), cur)
						}
					} else {
						// Invalid sub-iterator in MIT_NEED_ANY mode: use NULL
						if useAssoc && e.info != nil {
							result.OffsetSet(ctx, e.info.Value(), phpv.ZNULL.ZVal())
						} else if useAssoc && e.info == nil {
							return nil, phpobj.ThrowError(ctx, phpobj.InvalidArgumentException, "Sub-Iterator is associated with NULL")
						} else {
							result.OffsetSet(ctx, phpv.ZInt(i), phpv.ZNULL.ZVal())
						}
					}
				}
				return result.ZVal(), nil
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getMultipleIteratorData(o)
				if d == nil || len(d.entries) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Called key() on an invalid iterator")
				}
				// Check if any sub-iterator is invalid in MIT_NEED_ALL mode
				if d.flags&multipleIteratorMitNeedAll != 0 {
					for _, e := range d.entries {
						v, err := e.iterator.CallMethod(ctx, "valid")
						if err != nil || !bool(v.AsBool(ctx)) {
							return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Called key() with non valid sub iterator")
						}
					}
				}
				result := phpv.NewZArray()
				useAssoc := d.flags&multipleIteratorMitKeysAssoc != 0
				for i, e := range d.entries {
					v, _ := e.iterator.CallMethod(ctx, "valid")
					if v != nil && bool(v.AsBool(ctx)) {
						k, err := e.iterator.CallMethod(ctx, "key")
						if err != nil {
							return nil, err
						}
						if useAssoc && e.info != nil {
							result.OffsetSet(ctx, e.info.Value(), k)
						} else if useAssoc && e.info == nil {
							return nil, phpobj.ThrowError(ctx, phpobj.InvalidArgumentException, "Sub-Iterator is associated with NULL")
						} else {
							result.OffsetSet(ctx, phpv.ZInt(i), k)
						}
					} else {
						if useAssoc && e.info != nil {
							result.OffsetSet(ctx, e.info.Value(), phpv.ZNULL.ZVal())
						} else if useAssoc && e.info == nil {
							return nil, phpobj.ThrowError(ctx, phpobj.InvalidArgumentException, "Sub-Iterator is associated with NULL")
						} else {
							result.OffsetSet(ctx, phpv.ZInt(i), phpv.ZNULL.ZVal())
						}
					}
				}
				return result.ZVal(), nil
			}),
		},
		"next": {
			Name: "next",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getMultipleIteratorData(o)
				if d == nil {
					return nil, nil
				}
				for _, e := range d.entries {
					_, err := e.iterator.CallMethod(ctx, "next")
					if err != nil {
						return nil, err
					}
				}
				return nil, nil
			}),
		},
		"valid": {
			Name: "valid",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getMultipleIteratorData(o)
				if d == nil || len(d.entries) == 0 {
					return phpv.ZFalse.ZVal(), nil
				}
				if d.flags&multipleIteratorMitNeedAll != 0 {
					// All must be valid
					for _, e := range d.entries {
						v, err := e.iterator.CallMethod(ctx, "valid")
						if err != nil || !bool(v.AsBool(ctx)) {
							return phpv.ZFalse.ZVal(), err
						}
					}
					return phpv.ZTrue.ZVal(), nil
				}
				// Any must be valid
				for _, e := range d.entries {
					v, err := e.iterator.CallMethod(ctx, "valid")
					if err == nil && bool(v.AsBool(ctx)) {
						return phpv.ZTrue.ZVal(), nil
					}
				}
				return phpv.ZFalse.ZVal(), nil
			}),
		},
		"countiterators": {
			Name: "countIterators",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getMultipleIteratorData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return phpv.ZInt(len(d.entries)).ZVal(), nil
			}),
		},
		"getflags": {
			Name: "getFlags",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getMultipleIteratorData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return phpv.ZInt(d.flags).ZVal(), nil
			}),
		},
		"setflags": {
			Name: "setFlags",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getMultipleIteratorData(o)
				if d == nil {
					return nil, nil
				}
				if len(args) > 0 {
					d.flags = int(args[0].AsInt(ctx))
				}
				return nil, nil
			}),
		},
	}
}

var MultipleIteratorClass = &phpobj.ZClass{
	Name:            "MultipleIterator",
	Implementations: []*phpobj.ZClass{phpobj.Iterator},
	Const: map[phpv.ZString]*phpv.ZClassConst{
		"MIT_NEED_ANY":     {Value: phpv.ZInt(multipleIteratorMitNeedAny)},
		"MIT_NEED_ALL":     {Value: phpv.ZInt(multipleIteratorMitNeedAll)},
		"MIT_KEYS_NUMERIC": {Value: phpv.ZInt(multipleIteratorMitKeysNumeric)},
		"MIT_KEYS_ASSOC":   {Value: phpv.ZInt(multipleIteratorMitKeysAssoc)},
	},
}

// ============================================================================
// FilterIterator (abstract class)
// ============================================================================

type filterIteratorData struct {
	inner *phpobj.ZObject
}

func (d *filterIteratorData) Clone() any {
	return &filterIteratorData{inner: d.inner}
}

func getFilterIteratorData(o *phpobj.ZObject) *filterIteratorData {
	d := o.GetOpaque(FilterIteratorClass)
	if d == nil {
		return nil
	}
	return d.(*filterIteratorData)
}

func filterIteratorSkipToAccepted(ctx phpv.Context, o *phpobj.ZObject, d *filterIteratorData) error {
	// Unwrap to get the runtime class (e.g. ParentIterator) rather than the
	// narrowed FilterIterator class, so that accept() resolves to the override.
	realObj := o.Unwrap().(*phpobj.ZObject)
	for {
		v, err := d.inner.CallMethod(ctx, "valid")
		if err != nil || !bool(v.AsBool(ctx)) {
			return err
		}
		// PHP calls current() and key() on the inner iterator before accept()
		// to fetch/cache the current element
		if _, err := d.inner.CallMethod(ctx, "current"); err != nil {
			return err
		}
		if _, err := d.inner.CallMethod(ctx, "key"); err != nil {
			return err
		}
		// Call accept() on the FilterIterator subclass (not the inner iterator)
		accepted, err := realObj.CallMethod(ctx, "accept")
		if err != nil {
			return err
		}
		if bool(accepted.AsBool(ctx)) {
			return nil
		}
		_, err = d.inner.CallMethod(ctx, "next")
		if err != nil {
			return err
		}
	}
}

func initFilterIterator() {
	FilterIteratorClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "FilterIterator::__construct() expects exactly 1 argument, 0 given")
				}
				if args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "FilterIterator::__construct(): Argument #1 ($iterator) must be of type Iterator")
				}
				inner, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "FilterIterator::__construct(): Argument #1 ($iterator) must be of type Iterator")
				}
				o.SetOpaque(FilterIteratorClass, &filterIteratorData{inner: inner})
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getFilterIteratorData(o)
				if d == nil {
					return nil, nil
				}
				_, err := d.inner.CallMethod(ctx, "rewind")
				if err != nil {
					return nil, err
				}
				return nil, filterIteratorSkipToAccepted(ctx, o, d)
			}),
		},
		"current": {
			Name: "current",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getFilterIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return d.inner.CallMethod(ctx, "current")
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getFilterIteratorData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.inner.CallMethod(ctx, "key")
			}),
		},
		"next": {
			Name: "next",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getFilterIteratorData(o)
				if d == nil {
					return nil, nil
				}
				_, err := d.inner.CallMethod(ctx, "next")
				if err != nil {
					return nil, err
				}
				return nil, filterIteratorSkipToAccepted(ctx, o, d)
			}),
		},
		"valid": {
			Name: "valid",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getFilterIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return d.inner.CallMethod(ctx, "valid")
			}),
		},
		"getinneriterator": {
			Name: "getInnerIterator",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getFilterIteratorData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.inner.ZVal(), nil
			}),
		},
		"accept": {
			Name:      "accept",
			Modifiers: phpv.ZAttrAbstract | phpv.ZAttrPublic,
			Empty:     true,
		},
	}
}

var FilterIteratorClass = &phpobj.ZClass{
	Name:            "FilterIterator",
	Extends:         IteratorIteratorClass,
	Implementations: []*phpobj.ZClass{OuterIterator},
}

// ============================================================================
// RecursiveFilterIterator
// ============================================================================

func initRecursiveFilterIterator() {
	// Copy methods from FilterIterator
	RecursiveFilterIteratorClass.Methods = make(map[phpv.ZString]*phpv.ZClassMethod)
	for k, v := range FilterIteratorClass.Methods {
		RecursiveFilterIteratorClass.Methods[k] = v
	}
	// Add recursive methods
	RecursiveFilterIteratorClass.Methods["haschildren"] = &phpv.ZClassMethod{
		Name: "hasChildren",
		Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			d := getFilterIteratorData(o)
			if d == nil {
				return phpv.ZFalse.ZVal(), nil
			}
			return d.inner.CallMethod(ctx, "hasChildren")
		}),
	}
	RecursiveFilterIteratorClass.Methods["getchildren"] = &phpv.ZClassMethod{
		Name: "getChildren",
		Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			d := getFilterIteratorData(o)
			if d == nil {
				return phpv.ZNULL.ZVal(), nil
			}
			children, err := d.inner.CallMethod(ctx, "getChildren")
			if err != nil {
				return nil, err
			}
			if children == nil || children.GetType() != phpv.ZtObject {
				return phpv.ZNULL.ZVal(), nil
			}
			childObj := children.Value().(*phpobj.ZObject)
			result, err := phpobj.NewZObject(ctx, RecursiveFilterIteratorClass, childObj.ZVal())
			if err != nil {
				return nil, err
			}
			return result.ZVal(), nil
		}),
	}
}

var RecursiveFilterIteratorClass = &phpobj.ZClass{
	Name:            "RecursiveFilterIterator",
	Extends:         FilterIteratorClass,
	Implementations: []*phpobj.ZClass{RecursiveIterator, OuterIterator},
}

// ============================================================================
// RecursiveCachingIterator
// ============================================================================

func initRecursiveCachingIterator() {
	// Copy methods from CachingIterator
	RecursiveCachingIteratorClass.Methods = make(map[phpv.ZString]*phpv.ZClassMethod)
	for k, v := range CachingIteratorClass.Methods {
		RecursiveCachingIteratorClass.Methods[k] = v
	}
	// Add recursive methods
	RecursiveCachingIteratorClass.Methods["haschildren"] = &phpv.ZClassMethod{
		Name: "hasChildren",
		Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			d := getCachingIteratorData(o)
			if d == nil || d.currentVal == nil {
				return phpv.ZFalse.ZVal(), nil
			}
			return d.inner.CallMethod(ctx, "hasChildren")
		}),
	}
	RecursiveCachingIteratorClass.Methods["getchildren"] = &phpv.ZClassMethod{
		Name: "getChildren",
		Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			d := getCachingIteratorData(o)
			if d == nil {
				return phpv.ZNULL.ZVal(), nil
			}
			children, err := d.inner.CallMethod(ctx, "getChildren")
			if err != nil {
				return nil, err
			}
			if children == nil || children.GetType() != phpv.ZtObject {
				return phpv.ZNULL.ZVal(), nil
			}
			childObj := children.Value().(*phpobj.ZObject)
			result, err := phpobj.NewZObject(ctx, RecursiveCachingIteratorClass, childObj.ZVal())
			if err != nil {
				return nil, err
			}
			return result.ZVal(), nil
		}),
	}
}

var RecursiveCachingIteratorClass = &phpobj.ZClass{
	Name:            "RecursiveCachingIterator",
	Extends:         CachingIteratorClass,
	Implementations: []*phpobj.ZClass{RecursiveIterator, Countable},
}

// ============================================================================
// RecursiveRegexIterator
// ============================================================================

func initRecursiveRegexIterator() {
	// Copy methods from RegexIterator
	RecursiveRegexIteratorClass.Methods = make(map[phpv.ZString]*phpv.ZClassMethod)
	for k, v := range RegexIteratorClass.Methods {
		RecursiveRegexIteratorClass.Methods[k] = v
	}
	// Add recursive methods
	RecursiveRegexIteratorClass.Methods["haschildren"] = &phpv.ZClassMethod{
		Name: "hasChildren",
		Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			d := getRegexIteratorData(o)
			if d == nil {
				return phpv.ZFalse.ZVal(), nil
			}
			return d.inner.CallMethod(ctx, "hasChildren")
		}),
	}
	RecursiveRegexIteratorClass.Methods["getchildren"] = &phpv.ZClassMethod{
		Name: "getChildren",
		Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			d := getRegexIteratorData(o)
			if d == nil {
				return phpv.ZNULL.ZVal(), nil
			}
			children, err := d.inner.CallMethod(ctx, "getChildren")
			if err != nil {
				return nil, err
			}
			return children, nil
		}),
	}
}

var RecursiveRegexIteratorClass = &phpobj.ZClass{
	Name:            "RecursiveRegexIterator",
	Extends:         RegexIteratorClass,
	Implementations: []*phpobj.ZClass{RecursiveIterator},
}

// ============================================================================
// RecursiveCallbackFilterIterator
// ============================================================================

func initRecursiveCallbackFilterIterator() {
	// Copy methods from CallbackFilterIterator
	RecursiveCallbackFilterIteratorClass.Methods = make(map[phpv.ZString]*phpv.ZClassMethod)
	for k, v := range CallbackFilterIteratorClass.Methods {
		RecursiveCallbackFilterIteratorClass.Methods[k] = v
	}
	// Add recursive methods
	RecursiveCallbackFilterIteratorClass.Methods["haschildren"] = &phpv.ZClassMethod{
		Name: "hasChildren",
		Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			d := getCallbackFilterIteratorData(o)
			if d == nil {
				return phpv.ZFalse.ZVal(), nil
			}
			return d.inner.CallMethod(ctx, "hasChildren")
		}),
	}
	RecursiveCallbackFilterIteratorClass.Methods["getchildren"] = &phpv.ZClassMethod{
		Name: "getChildren",
		Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			d := getCallbackFilterIteratorData(o)
			if d == nil {
				return phpv.ZNULL.ZVal(), nil
			}
			children, err := d.inner.CallMethod(ctx, "getChildren")
			if err != nil {
				return nil, err
			}
			if children == nil || children.GetType() != phpv.ZtObject {
				return phpv.ZNULL.ZVal(), nil
			}
			// Wrap children in a new RecursiveCallbackFilterIterator with same callback
			childObj := children.Value().(*phpobj.ZObject)
			// Create callback ZVal - use phpv.NewZVal to properly wrap the Callable
			callbackVal := phpv.NewZVal(d.callback)
			result, err := phpobj.NewZObject(ctx, RecursiveCallbackFilterIteratorClass, childObj.ZVal(), callbackVal)
			if err != nil {
				return nil, err
			}
			return result.ZVal(), nil
		}),
	}
}

var RecursiveCallbackFilterIteratorClass = &phpobj.ZClass{
	Name:            "RecursiveCallbackFilterIterator",
	Extends:         CallbackFilterIteratorClass,
	Implementations: []*phpobj.ZClass{RecursiveIterator},
}

// ============================================================================
// ParentIterator (extends RecursiveFilterIterator, accept() checks hasChildren)
// ============================================================================

func initParentIterator() {
	// Copy methods from RecursiveFilterIterator
	ParentIteratorClass.Methods = make(map[phpv.ZString]*phpv.ZClassMethod)
	for k, v := range RecursiveFilterIteratorClass.Methods {
		ParentIteratorClass.Methods[k] = v
	}
	// Override __construct to use ParentIterator name in error messages
	ParentIteratorClass.Methods["__construct"] = &phpv.ZClassMethod{
		Name: "__construct",
		Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			if len(args) == 0 {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ParentIterator::__construct() expects exactly 1 argument, 0 given")
			}
			if args[0] == nil || args[0].GetType() != phpv.ZtObject {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ParentIterator::__construct(): Argument #1 ($iterator) must be of type RecursiveIterator")
			}
			inner, ok := args[0].Value().(*phpobj.ZObject)
			if !ok {
				return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "ParentIterator::__construct(): Argument #1 ($iterator) must be of type RecursiveIterator")
			}
			o.SetOpaque(FilterIteratorClass, &filterIteratorData{inner: inner})
			return nil, nil
		}),
	}
	// Override accept to return hasChildren()
	ParentIteratorClass.Methods["accept"] = &phpv.ZClassMethod{
		Name: "accept",
		Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
			d := getFilterIteratorData(o)
			if d == nil {
				return phpv.ZFalse.ZVal(), nil
			}
			return d.inner.CallMethod(ctx, "hasChildren")
		}),
	}
}

var ParentIteratorClass = &phpobj.ZClass{
	Name:            "ParentIterator",
	Extends:         RecursiveFilterIteratorClass,
	Implementations: []*phpobj.ZClass{RecursiveIterator, OuterIterator},
}
