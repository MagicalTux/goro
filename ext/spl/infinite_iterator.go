package spl

import (
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// infiniteIteratorData holds the internal state for an InfiniteIterator instance.
// InfiniteIterator extends IteratorIterator in PHP and inherits its caching behavior.
type infiniteIteratorData struct {
	inner       *phpobj.ZObject
	cachedVal   *phpv.ZVal
	cachedKey   *phpv.ZVal
	cachedValid bool
}

func getInfiniteIteratorData(o *phpobj.ZObject) *infiniteIteratorData {
	d := o.GetOpaque(InfiniteIteratorClass)
	if d == nil {
		return nil
	}
	return d.(*infiniteIteratorData)
}

func infiniteIteratorFetchCache(ctx phpv.Context, d *infiniteIteratorData) error {
	v, err := d.inner.CallMethod(ctx, "valid")
	if err != nil {
		d.cachedValid = false
		d.cachedVal = nil
		d.cachedKey = nil
		return err
	}
	d.cachedValid = v != nil && bool(v.AsBool(ctx))
	if d.cachedValid {
		d.cachedVal, err = d.inner.CallMethod(ctx, "current")
		if err != nil {
			return err
		}
		d.cachedKey, err = d.inner.CallMethod(ctx, "key")
		if err != nil {
			return err
		}
	} else {
		d.cachedVal = nil
		d.cachedKey = nil
	}
	return nil
}

func initInfiniteIterator() {
	InfiniteIteratorClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "InfiniteIterator::__construct(): Argument #1 ($iterator) must be of type Iterator")
				}
				inner, ok := args[0].Value().(*phpobj.ZObject)
				if !ok || !inner.GetClass().Implements(phpobj.Iterator) {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "InfiniteIterator::__construct(): Argument #1 ($iterator) must be of type Iterator")
				}
				o.SetOpaque(InfiniteIteratorClass, &infiniteIteratorData{inner: inner})
				// Also set IteratorIterator opaque so parent methods work
				o.SetOpaque(IteratorIteratorClass, &iteratorIteratorData{inner: inner})
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getInfiniteIteratorData(o)
				if d == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, "The object is in an invalid state as the parent constructor was not called")
				}
				_, err := d.inner.CallMethod(ctx, "rewind")
				if err != nil {
					return nil, err
				}
				return nil, infiniteIteratorFetchCache(ctx, d)
			}),
		},
		"current": {
			Name: "current",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getInfiniteIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				if d.cachedVal != nil {
					return d.cachedVal, nil
				}
				return phpv.ZFalse.ZVal(), nil
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getInfiniteIteratorData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				if d.cachedKey != nil {
					return d.cachedKey, nil
				}
				return phpv.ZNULL.ZVal(), nil
			}),
		},
		"next": {
			Name: "next",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getInfiniteIteratorData(o)
				if d == nil {
					return nil, nil
				}
				// Advance the inner iterator
				_, err := d.inner.CallMethod(ctx, "next")
				if err != nil {
					return nil, err
				}
				// Check if inner is still valid
				v, err := d.inner.CallMethod(ctx, "valid")
				if err != nil {
					return nil, err
				}
				isValid := v != nil && bool(v.AsBool(ctx))
				if !isValid {
					// Inner exhausted, rewind it
					_, err = d.inner.CallMethod(ctx, "rewind")
					if err != nil {
						return nil, err
					}
					// After rewind, re-check valid and fetch cache
					return nil, infiniteIteratorFetchCache(ctx, d)
				}
				// Inner is valid - cache current/key without re-checking valid
				d.cachedValid = true
				d.cachedVal, err = d.inner.CallMethod(ctx, "current")
				if err != nil {
					return nil, err
				}
				d.cachedKey, err = d.inner.CallMethod(ctx, "key")
				if err != nil {
					return nil, err
				}
				return nil, nil
			}),
		},
		"valid": {
			Name: "valid",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getInfiniteIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return phpv.ZBool(d.cachedValid).ZVal(), nil
			}),
		},
		"getinneriterator": {
			Name: "getInnerIterator",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getInfiniteIteratorData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.inner.ZVal(), nil
			}),
		},
	}
}

var InfiniteIteratorClass = &phpobj.ZClass{
	Name:            "InfiniteIterator",
	Implementations: []*phpobj.ZClass{OuterIterator},
}
