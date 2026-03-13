package spl

import (
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// arrayIteratorData holds the internal state for an ArrayIterator instance
type arrayIteratorData struct {
	array *phpv.ZArray
	iter  phpv.ZIterator
}

func (d *arrayIteratorData) Clone() any {
	return &arrayIteratorData{
		array: d.array.Dup(),
		iter:  nil, // reset iterator on clone
	}
}

func getArrayIteratorData(o *phpobj.ZObject) *arrayIteratorData {
	d := o.GetOpaque(ArrayIteratorClass)
	if d == nil {
		return nil
	}
	return d.(*arrayIteratorData)
}

func initArrayIterator() {
	ArrayIteratorClass.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := &arrayIteratorData{}
				if len(args) > 0 && args[0] != nil && args[0].GetType() == phpv.ZtArray {
					d.array = args[0].Value().(*phpv.ZArray).Dup()
				} else {
					d.array = phpv.NewZArray()
				}
				d.iter = d.array.NewIterator()
				o.SetOpaque(ArrayIteratorClass, d)
				return nil, nil
			}),
		},
		"rewind": {
			Name: "rewind",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayIteratorData(o)
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
				d := getArrayIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return d.iter.Current(ctx)
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayIteratorData(o)
				if d == nil {
					return phpv.ZNULL.ZVal(), nil
				}
				return d.iter.Key(ctx)
			}),
		},
		"next": {
			Name: "next",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayIteratorData(o)
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
				d := getArrayIteratorData(o)
				if d == nil {
					return phpv.ZFalse.ZVal(), nil
				}
				return phpv.ZBool(d.iter.Valid(ctx)).ZVal(), nil
			}),
		},
		"count": {
			Name: "count",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getArrayIteratorData(o)
				if d == nil {
					return phpv.ZInt(0).ZVal(), nil
				}
				return d.array.Count(ctx).ZVal(), nil
			}),
		},
	}
}

var ArrayIteratorClass = &phpobj.ZClass{
	Name:            "ArrayIterator",
	Implementations: []*phpobj.ZClass{phpobj.Iterator, Countable},
}
