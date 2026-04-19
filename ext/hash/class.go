package hash

import (
	"fmt"

	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// > class HashContext
var HashContext = &phpobj.ZClass{
	Name: "HashContext",
}

func init() {
	HashContext.Methods = map[phpv.ZString]*phpv.ZClassMethod{
		"__construct": {
			Name:      "__construct",
			Modifiers: phpv.ZAttrPrivate,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				return nil, phpobj.ThrowError(ctx, phpobj.Error, "Call to private HashContext::__construct() from global scope")
			}),
		},
		"__clone": {
			Name:      "__clone",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				opaque := o.GetOpaque(HashContext)
				if opaque == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot clone a finalized HashContext")
				}
				hcd, ok := opaque.(*hashContextData)
				if !ok || hcd.finalized {
					return nil, phpobj.ThrowError(ctx, phpobj.Error, "Cannot clone a finalized HashContext")
				}
				return nil, nil
			}),
		},
		"__debuginfo": {
			Name:      "__debugInfo",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				opaque := o.GetOpaque(HashContext)
				arr := phpv.NewZArray()
				if opaque != nil {
					if hcd, ok := opaque.(*hashContextData); ok {
						arr.OffsetSet(ctx, phpv.ZString("algo").ZVal(), phpv.ZString(hcd.algo).ZVal())
					}
				}
				return arr.ZVal(), nil
			}),
		},
		"__serialize": {
			Name:      "__serialize",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				opaque := o.GetOpaque(HashContext)
				if opaque == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Serialization of 'HashContext' is not allowed")
				}
				hcd, ok := opaque.(*hashContextData)
				if !ok {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Serialization of 'HashContext' is not allowed")
				}
				if hcd.finalized {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, fmt.Sprintf("HashContext for algorithm %q cannot be serialized", string(hcd.algo)))
				}
				if hcd.isHmac {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "HashContext with HASH_HMAC option cannot be serialized")
				}

				arr := phpv.NewZArray()
				// PHP-compatible 5-element format
				arr.OffsetSet(ctx, phpv.ZInt(0).ZVal(), phpv.ZString(hcd.algo).ZVal())
				arr.OffsetSet(ctx, phpv.ZInt(1).ZVal(), phpv.ZInt(0).ZVal())

				stateArr, _ := serializeHashState(hcd.Hash)
				// Fallback for custom hash implementations: use replay data
				if stateArr == nil || stateArr.Count(ctx) == 0 {
					stateArr = phpv.NewZArray()
					stateArr.OffsetSet(nil, phpv.ZInt(0).ZVal(), phpv.ZString(hcd.writtenData).ZVal())
					stateArr.OffsetSet(nil, phpv.ZInt(1).ZVal(), phpv.ZInt(int64(len(hcd.writtenData))).ZVal())
				}
				arr.OffsetSet(ctx, phpv.ZInt(2).ZVal(), stateArr.ZVal())
				arr.OffsetSet(ctx, phpv.ZInt(3).ZVal(), phpv.ZInt(phpSerializeMagic).ZVal())
				arr.OffsetSet(ctx, phpv.ZInt(4).ZVal(), phpv.NewZArray().ZVal())

				return arr.ZVal(), nil
			}),
		},
		"__unserialize": {
			Name:      "__unserialize",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				existing := o.GetOpaque(HashContext)
				if existing != nil {
					if hcd, ok := existing.(*hashContextData); ok && !hcd.finalized {
						return nil, phpobj.ThrowError(ctx, phpobj.Exception, "HashContext::__unserialize called on initialized object")
					}
				}

				if len(args) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Incomplete or ill-formed serialization data")
				}
				arr := args[0].AsArray(ctx)
				if arr == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Incomplete or ill-formed serialization data")
				}

				algoVal, _ := arr.OffsetGet(ctx, phpv.ZInt(0))
				flagsVal, _ := arr.OffsetGet(ctx, phpv.ZInt(1))
				stateVal, _ := arr.OffsetGet(ctx, phpv.ZInt(2))
				magicVal, _ := arr.OffsetGet(ctx, phpv.ZInt(3))
				hmacVal, _ := arr.OffsetGet(ctx, phpv.ZInt(4))

				if algoVal == nil || flagsVal == nil || stateVal == nil || magicVal == nil || hmacVal == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Incomplete or ill-formed serialization data")
				}

				if algoVal.GetType() != phpv.ZtString {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Incomplete or ill-formed serialization data")
				}
				algo := phpv.ZString(algoVal.Value().(phpv.ZString)).ToLower()

				if flagsVal.GetType() != phpv.ZtInt {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Incomplete or ill-formed serialization data")
				}
				flags := flagsVal.Value().(phpv.ZInt)

				if flags == 1 {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "HashContext with HASH_HMAC option cannot be serialized")
				}

				if stateVal.GetType() != phpv.ZtArray {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, fmt.Sprintf("Incomplete or ill-formed serialization data (%q code %d)", string(algo), -1))
				}

				magic := magicVal.AsInt(ctx)

				algoLower := algo.ToLower()
				if _, ok := algos[algoLower]; !ok {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "Unknown hash algorithm")
				}

				if magic != phpSerializeMagic {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, fmt.Sprintf("Incomplete or ill-formed serialization data (%q code %d)", string(algo), -1))
				}

				stateArr := stateVal.AsArray(ctx)
				hcd, err := unserializeFromPHPState(ctx, algo, stateArr)
				if err != nil {
					code := int64(-1)
					switch err {
					case errSpecMismatch:
						code = phpHashUnserializeSpecMismatch
					case errInvalidSize:
						code = phpHashUnserializeInvalidSize
					}
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, fmt.Sprintf("Incomplete or ill-formed serialization data (%q code %d)", string(algo), code))
				}

				_ = hmacVal
				o.SetOpaque(HashContext, hcd)
				return nil, nil
			}),
		},
	}
}
