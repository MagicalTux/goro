package spl

import (
	"regexp"
	"strings"
	"unicode/utf8"

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
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
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
					return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "LimitIterator::__construct(): Argument #2 ($offset) must be non-negative")
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
				// Skip to offset
				for d.pos < d.offset {
					v, err := d.inner.CallMethod(ctx, "valid")
					if err != nil {
						return nil, err
					}
					if !bool(v.AsBool(ctx)) {
						break
					}
					_, err = d.inner.CallMethod(ctx, "next")
					if err != nil {
						return nil, err
					}
					d.pos++
				}
				return nil, nil
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
	}
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
}

func (d *cachingIteratorData) Clone() any {
	return &cachingIteratorData{
		inner:      d.inner,
		flags:      d.flags,
		currentVal: d.currentVal,
		currentKey: d.currentKey,
		hasNext:    d.hasNext,
		started:    d.started,
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
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
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
				o.SetOpaque(CachingIteratorClass, &cachingIteratorData{
					inner:   inner,
					flags:   flags,
					hasNext: false,
					started: false,
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
				// Fetch first element
				v, err := d.inner.CallMethod(ctx, "valid")
				if err != nil {
					return nil, err
				}
				if bool(v.AsBool(ctx)) {
					d.currentVal, _ = d.inner.CallMethod(ctx, "current")
					d.currentKey, _ = d.inner.CallMethod(ctx, "key")
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
					d.flags = int(args[0].AsInt(ctx))
				}
				return nil, nil
			}),
		},
		"__tostring": {
			Name: "__toString",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getCachingIteratorData(o)
				if d == nil || d.currentVal == nil {
					return phpv.ZString("").ZVal(), nil
				}
				if d.flags&cachingIteratorToStringUseKey != 0 && d.currentKey != nil {
					return phpv.ZString(d.currentKey.AsString(ctx)).ZVal(), nil
				}
				if d.flags&cachingIteratorToStringUseCurrent != 0 && d.currentVal != nil {
					return phpv.ZString(d.currentVal.AsString(ctx)).ZVal(), nil
				}
				return phpv.ZString(d.currentVal.AsString(ctx)).ZVal(), nil
			}),
		},
	}
}

var CachingIteratorClass = &phpobj.ZClass{
	Name:            "CachingIterator",
	Implementations: []*phpobj.ZClass{OuterIterator, Countable},
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
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "AppendIterator::append(): Argument #1 ($iterator) must be of type Iterator")
				}
				inner, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "AppendIterator::append(): Argument #1 ($iterator) must be of type Iterator")
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
	inner   *phpobj.ZObject
	pattern phpv.ZString
	mode    int
	flags   int
}

func (d *regexIteratorData) Clone() any {
	return &regexIteratorData{
		inner:   d.inner,
		pattern: d.pattern,
		mode:    d.mode,
		flags:   d.flags,
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
				return nil, err
			}),
		},
		"current": {
			Name: "current",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRegexIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
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
					if d.accept(ctx) {
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
				return phpv.ZBool(d.accept(ctx)).ZVal(), nil
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
func (d *regexIteratorData) accept(ctx phpv.Context) bool {
	val, err := d.inner.CallMethod(ctx, "current")
	if err != nil || val == nil {
		return false
	}
	subject := string(val.AsString(ctx))

	re, err := parsePCREPattern(string(d.pattern))
	if err != nil || re == nil {
		return false
	}
	return re.MatchString(subject)
}

var RegexIteratorClass = &phpobj.ZClass{
	Name:            "RegexIterator",
	Extends:         nil, // Would extend FilterIterator in full impl
	Implementations: []*phpobj.ZClass{OuterIterator},
	Const: map[phpv.ZString]*phpv.ZClassConst{
		"MATCH":        {Value: phpv.ZInt(regexIteratorMatch)},
		"GET_MATCH":    {Value: phpv.ZInt(regexIteratorGetMatch)},
		"ALL_MATCHES":  {Value: phpv.ZInt(regexIteratorAllMatches)},
		"SPLIT":        {Value: phpv.ZInt(regexIteratorSplit)},
		"REPLACE":      {Value: phpv.ZInt(regexIteratorReplace)},
		"USE_KEY":      {Value: phpv.ZInt(regexIteratorUseKey)},
	},
}

// ============================================================================
// RecursiveArrayIterator
// ============================================================================

type recursiveArrayIteratorData struct {
	array *phpv.ZArray
	iter  phpv.ZIterator
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
	RecursiveArrayIteratorClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := &recursiveArrayIteratorData{}
				if len(args) > 0 && args[0] != nil && args[0].GetType() == phpv.ZtArray {
					d.array = args[0].Value().(*phpv.ZArray).Dup()
				} else {
					d.array = phpv.NewZArray()
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
				return phpv.ZBool(v.GetType() == phpv.ZtArray).ZVal(), nil
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
				if v.GetType() != phpv.ZtArray {
					return phpv.ZNULL.ZVal(), nil
				}
				child, err := phpobj.NewZObject(ctx, RecursiveArrayIteratorClass, v)
				if err != nil {
					return phpv.ZNULL.ZVal(), err
				}
				return child.ZVal(), nil
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
	Implementations: []*phpobj.ZClass{RecursiveIterator, Countable},
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
	stack []*phpobj.ZObject
	mode  int
	depth int
}

func (d *recursiveIteratorIteratorData) Clone() any {
	nd := &recursiveIteratorIteratorData{
		stack: make([]*phpobj.ZObject, len(d.stack)),
		mode:  d.mode,
		depth: d.depth,
	}
	copy(nd.stack, d.stack)
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
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
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
				d := &recursiveIteratorIteratorData{
					stack: []*phpobj.ZObject{inner},
					mode:  mode,
					depth: 0,
				}
				o.SetOpaque(RecursiveIteratorIteratorClass, d)
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveIteratorIteratorData(o)
				if d == nil || len(d.stack) == 0 {
					return nil, nil
				}
				// Reset to just the root iterator
				root := d.stack[0]
				d.stack = []*phpobj.ZObject{root}
				d.depth = 0
				_, err := root.CallMethod(ctx, "rewind")
				if err != nil {
					return nil, err
				}
				// Descend into children if needed
				err = recursiveIteratorDescend(ctx, d)
				return nil, err
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
				err := recursiveIteratorNext(ctx, d)
				return nil, err
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
	}
}

// recursiveIteratorDescend tries to descend into children of the current element
// for LEAVES_ONLY and SELF_FIRST modes.
func recursiveIteratorDescend(ctx phpv.Context, d *recursiveIteratorIteratorData) error {
	for {
		top := d.stack[len(d.stack)-1]
		v, err := top.CallMethod(ctx, "valid")
		if err != nil || !bool(v.AsBool(ctx)) {
			return err
		}

		// Check if current has children
		hasChildrenResult, err := top.CallMethod(ctx, "hasChildren")
		if err != nil {
			return nil // Not a RecursiveIterator, stop descending
		}
		if !bool(hasChildrenResult.AsBool(ctx)) {
			return nil // Leaf node
		}

		// Get children iterator
		childResult, err := top.CallMethod(ctx, "getChildren")
		if err != nil {
			return nil
		}
		if childResult == nil || childResult.GetType() != phpv.ZtObject {
			return nil
		}
		child, ok := childResult.Value().(*phpobj.ZObject)
		if !ok {
			return nil
		}
		// Rewind the child
		_, err = child.CallMethod(ctx, "rewind")
		if err != nil {
			return err
		}
		d.stack = append(d.stack, child)
		d.depth++
	}
}

// recursiveIteratorNext advances the recursive iterator
func recursiveIteratorNext(ctx phpv.Context, d *recursiveIteratorIteratorData) error {
	if len(d.stack) == 0 {
		return nil
	}

	top := d.stack[len(d.stack)-1]
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
		return recursiveIteratorDescend(ctx, d)
	}

	// Current level exhausted, go back up
	for len(d.stack) > 1 {
		d.stack = d.stack[:len(d.stack)-1]
		d.depth--
		top = d.stack[len(d.stack)-1]
		_, err = top.CallMethod(ctx, "next")
		if err != nil {
			return err
		}
		v, err = top.CallMethod(ctx, "valid")
		if err != nil {
			return err
		}
		if bool(v.AsBool(ctx)) {
			return recursiveIteratorDescend(ctx, d)
		}
	}
	return nil
}

var RecursiveIteratorIteratorClass = &phpobj.ZClass{
	Name:            "RecursiveIteratorIterator",
	Implementations: []*phpobj.ZClass{OuterIterator},
	Const: map[phpv.ZString]*phpv.ZClassConst{
		"LEAVES_ONLY": {Value: phpv.ZInt(recursiveIteratorLeavesOnly)},
		"SELF_FIRST":  {Value: phpv.ZInt(recursiveIteratorSelfFirst)},
		"CHILD_FIRST": {Value: phpv.ZInt(recursiveIteratorChildFirst)},
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
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
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
				return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Accessing current() on an EmptyIterator")
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return nil, phpobj.ThrowError(ctx, phpobj.RuntimeException, "Accessing key() on an EmptyIterator")
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
	callback *phpv.ZVal
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
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "CallbackFilterIterator::__construct() expects exactly 2 arguments")
				}
				if args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "CallbackFilterIterator::__construct(): Argument #1 ($iterator) must be of type Iterator")
				}
				inner, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "CallbackFilterIterator::__construct(): Argument #1 ($iterator) must be of type Iterator")
				}
				o.SetOpaque(CallbackFilterIteratorClass, &callbackFilterIteratorData{
					inner:    inner,
					callback: args[1],
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
					if callbackFilterAccept(ctx, d) {
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
					if callbackFilterAccept(ctx, d) {
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

func callbackFilterAccept(ctx phpv.Context, d *callbackFilterIteratorData) bool {
	current, err := d.inner.CallMethod(ctx, "current")
	if err != nil {
		return false
	}
	key, err := d.inner.CallMethod(ctx, "key")
	if err != nil {
		return false
	}
	result, err := ctx.CallZVal(ctx, d.callback.Value().(phpv.Callable), []*phpv.ZVal{current, key, d.inner.ZVal()}, nil)
	if err != nil {
		return false
	}
	return result != nil && bool(result.AsBool(ctx))
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

type multipleIteratorData struct {
	iterators []*phpobj.ZObject
	flags     int
}

func (d *multipleIteratorData) Clone() any {
	nd := &multipleIteratorData{
		iterators: make([]*phpobj.ZObject, len(d.iterators)),
		flags:     d.flags,
	}
	copy(nd.iterators, d.iterators)
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
				d.iterators = append(d.iterators, inner)
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getMultipleIteratorData(o)
				if d == nil {
					return nil, nil
				}
				for _, it := range d.iterators {
					_, err := it.CallMethod(ctx, "rewind")
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
				if d == nil || len(d.iterators) == 0 {
					return phpv.ZFalse.ZVal(), nil
				}
				result := phpv.NewZArray()
				for i, it := range d.iterators {
					v, err := it.CallMethod(ctx, "current")
					if err != nil {
						return nil, err
					}
					result.OffsetSet(ctx, phpv.ZInt(i), v)
				}
				return result.ZVal(), nil
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getMultipleIteratorData(o)
				if d == nil || len(d.iterators) == 0 {
					return phpv.ZFalse.ZVal(), nil
				}
				result := phpv.NewZArray()
				for i, it := range d.iterators {
					k, err := it.CallMethod(ctx, "key")
					if err != nil {
						return nil, err
					}
					result.OffsetSet(ctx, phpv.ZInt(i), k)
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
				for _, it := range d.iterators {
					_, err := it.CallMethod(ctx, "next")
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
				if d == nil || len(d.iterators) == 0 {
					return phpv.ZFalse.ZVal(), nil
				}
				if d.flags&multipleIteratorMitNeedAll != 0 {
					// All must be valid
					for _, it := range d.iterators {
						v, err := it.CallMethod(ctx, "valid")
						if err != nil || !bool(v.AsBool(ctx)) {
							return phpv.ZFalse.ZVal(), err
						}
					}
					return phpv.ZTrue.ZVal(), nil
				}
				// Any must be valid
				for _, it := range d.iterators {
					v, err := it.CallMethod(ctx, "valid")
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
				return phpv.ZInt(len(d.iterators)).ZVal(), nil
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
