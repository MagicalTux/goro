package spl

import (
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// RecursiveTreeIterator constants
const (
	rtiBypassCurrent = 4
	rtiBypassKey     = 8
	rtiPrefixLeft    = 0
	rtiPrefixMidHasNext = 1
	rtiPrefixMidLast    = 2
	rtiPrefixEndHasNext = 3
	rtiPrefixEndLast    = 4
	rtiPrefixRight      = 5
)

type recursiveTreeIteratorData struct {
	flags   int
	prefix  [6]string
}

func (d *recursiveTreeIteratorData) Clone() any {
	nd := &recursiveTreeIteratorData{
		flags:  d.flags,
		prefix: d.prefix,
	}
	return nd
}

func getRecursiveTreeIteratorData(o *phpobj.ZObject) *recursiveTreeIteratorData {
	d := o.GetOpaque(RecursiveTreeIteratorClass)
	if d == nil {
		return nil
	}
	return d.(*recursiveTreeIteratorData)
}

var RecursiveTreeIteratorClass = &phpobj.ZClass{
	Name:            "RecursiveTreeIterator",
	Extends:         RecursiveIteratorIteratorClass,
	Implementations: []*phpobj.ZClass{OuterIterator},
	Const: map[phpv.ZString]*phpv.ZClassConst{
		"BYPASS_CURRENT": {Value: phpv.ZInt(rtiBypassCurrent)},
		"BYPASS_KEY":     {Value: phpv.ZInt(rtiBypassKey)},
		"PREFIX_LEFT":       {Value: phpv.ZInt(rtiPrefixLeft)},
		"PREFIX_MID_HAS_NEXT": {Value: phpv.ZInt(rtiPrefixMidHasNext)},
		"PREFIX_MID_LAST":    {Value: phpv.ZInt(rtiPrefixMidLast)},
		"PREFIX_END_HAS_NEXT": {Value: phpv.ZInt(rtiPrefixEndHasNext)},
		"PREFIX_END_LAST":    {Value: phpv.ZInt(rtiPrefixEndLast)},
		"PREFIX_RIGHT":       {Value: phpv.ZInt(rtiPrefixRight)},
	},
}

func initRecursiveTreeIterator() {
	// Start by copying all parent methods from RecursiveIteratorIterator
	RecursiveTreeIteratorClass.Methods = make(map[phpv.ZString]*phpv.ZClassMethod)
	for k, v := range RecursiveIteratorIteratorClass.Methods {
		RecursiveTreeIteratorClass.Methods[k] = v
	}

	// Override specific methods
	overrides := map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name: "__construct",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				// Call parent constructor
				if len(args) == 0 || args[0] == nil || args[0].GetType() != phpv.ZtObject {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "RecursiveTreeIterator::__construct(): Argument #1 ($iterator) must be of type RecursiveIterator|IteratorAggregate")
				}
				inner, ok := args[0].Value().(*phpobj.ZObject)
				if !ok {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "RecursiveTreeIterator::__construct(): Argument #1 ($iterator) must be of type RecursiveIterator|IteratorAggregate")
				}

				flags := rtiBypassKey
				if len(args) > 1 {
					flags = int(args[1].AsInt(ctx))
				}

				mode := recursiveIteratorSelfFirst
				if len(args) > 3 {
					mode = int(args[3].AsInt(ctx))
				}

				// Set up parent RecursiveIteratorIterator data
				d := &recursiveIteratorIteratorData{
					stack: []*phpobj.ZObject{inner},
					mode:  mode,
					depth: 0,
				}
				o.SetOpaque(RecursiveIteratorIteratorClass, d)

				// Set up tree iterator data
				td := &recursiveTreeIteratorData{
					flags: flags,
					prefix: [6]string{
						"",    // PREFIX_LEFT
						"| ",  // PREFIX_MID_HAS_NEXT
						"  ",  // PREFIX_MID_LAST
						"|-",  // PREFIX_END_HAS_NEXT
						"\\-", // PREFIX_END_LAST
						"",    // PREFIX_RIGHT
					},
				}
				o.SetOpaque(RecursiveTreeIteratorClass, td)
				return nil, nil
			}),
		},
		"getprefix": {
			Name: "getPrefix",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				td := getRecursiveTreeIteratorData(o)
				if td == nil {
					return phpv.ZStr(""), nil
				}
				d := getRecursiveIteratorIteratorData(o)
				if d == nil {
					return phpv.ZStr(""), nil
				}

				prefix := td.prefix[rtiPrefixLeft]
				depth := len(d.stack) - 1
				for i := 0; i < depth; i++ {
					prefix += td.prefix[rtiPrefixMidHasNext]
				}
				if depth >= 0 {
					prefix += td.prefix[rtiPrefixEndHasNext]
				}
				prefix += td.prefix[rtiPrefixRight]
				return phpv.ZStr(prefix), nil
			}),
		},
		"setprefixpart": {
			Name: "setPrefixPart",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				td := getRecursiveTreeIteratorData(o)
				if td == nil {
					return nil, nil
				}
				if len(args) < 2 {
					return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "RecursiveTreeIterator::setPrefixPart() expects exactly 2 arguments")
				}
				part := int(args[0].AsInt(ctx))
				if part < 0 || part > 5 {
					return nil, phpobj.ThrowError(ctx, phpobj.OutOfRangeException, "Prefix part index out of range")
				}
				td.prefix[part] = string(args[1].AsString(ctx))
				return nil, nil
			}),
		},
		"getentry": {
			Name: "getEntry",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				d := getRecursiveIteratorIteratorData(o)
				if d == nil || len(d.stack) == 0 {
					return phpv.ZStr(""), nil
				}
				top := d.stack[len(d.stack)-1]
				v, err := top.CallMethod(ctx, "current")
				if err != nil {
					return phpv.ZStr(""), nil
				}
				return v.AsString(ctx).ZVal(), nil
			}),
		},
		"getpostfix": {
			Name: "getPostfix",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return phpv.ZStr(""), nil
			}),
		},
		"current": {
			Name: "current",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				td := getRecursiveTreeIteratorData(o)
				d := getRecursiveIteratorIteratorData(o)
				if d == nil || len(d.stack) == 0 {
					return phpv.ZFalse.ZVal(), nil
				}

				if td != nil && td.flags&rtiBypassCurrent != 0 {
					top := d.stack[len(d.stack)-1]
					return top.CallMethod(ctx, "current")
				}

				// Build prefix + entry + postfix
				prefix, _ := o.CallMethod(ctx, "getPrefix")
				entry, _ := o.CallMethod(ctx, "getEntry")
				postfix, _ := o.CallMethod(ctx, "getPostfix")

				result := string(prefix.AsString(ctx)) + string(entry.AsString(ctx)) + string(postfix.AsString(ctx))
				return phpv.ZStr(result), nil
			}),
		},
		"key": {
			Name: "key",
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				td := getRecursiveTreeIteratorData(o)
				d := getRecursiveIteratorIteratorData(o)
				if d == nil || len(d.stack) == 0 {
					return phpv.ZNULL.ZVal(), nil
				}

				top := d.stack[len(d.stack)-1]
				key, err := top.CallMethod(ctx, "key")
				if err != nil {
					return phpv.ZNULL.ZVal(), nil
				}

				if td != nil && td.flags&rtiBypassKey != 0 {
					return key, nil
				}

				// Prepend prefix to key
				prefix, _ := o.CallMethod(ctx, "getPrefix")
				return phpv.ZStr(string(prefix.AsString(ctx)) + string(key.AsString(ctx))), nil
			}),
		},
	}
	for k, v := range overrides {
		RecursiveTreeIteratorClass.Methods[k] = v
	}
}
