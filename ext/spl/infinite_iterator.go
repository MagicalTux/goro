package spl

import (
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// infiniteIteratorData holds the internal state for an InfiniteIterator instance
type infiniteIteratorData struct {
	inner *phpobj.ZObject
}

func getInfiniteIteratorData(o *phpobj.ZObject) *infiniteIteratorData {
	d := o.GetOpaque(InfiniteIteratorClass)
	if d == nil {
		return nil
	}
	return d.(*infiniteIteratorData)
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
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getInfiniteIteratorData(o)
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
				d := getInfiniteIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return d.inner.CallMethod(ctx, "current")
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getInfiniteIteratorData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.inner.CallMethod(ctx, "key")
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
				// If inner is no longer valid, rewind it
				v, err := d.inner.CallMethod(ctx, "valid")
				if err != nil {
					return nil, err
				}
				if v == nil || !bool(v.AsBool(ctx)) {
					_, err = d.inner.CallMethod(ctx, "rewind")
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
				d := getInfiniteIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				// InfiniteIterator delegates valid() to the inner iterator
				return d.inner.CallMethod(ctx, "valid")
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
