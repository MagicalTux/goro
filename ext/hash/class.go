package hash

import (
	"fmt"

	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
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
				// Index 0: algo name
				arr.OffsetSet(ctx, phpv.ZInt(0).ZVal(), phpv.ZString(hcd.algo).ZVal())
				// Index 1: options (0 = normal, 1 = HMAC)
				options := phpv.ZInt(0)
				if hcd.isHmac {
					options = phpv.ZInt(1)
				}
				arr.OffsetSet(ctx, phpv.ZInt(1).ZVal(), options.ZVal())
				// Index 2: HMAC key (string, empty if not HMAC)
				arr.OffsetSet(ctx, phpv.ZInt(2).ZVal(), phpv.ZString(hcd.hmacKey).ZVal())
				// Index 3: written data (raw bytes for replay)
				arr.OffsetSet(ctx, phpv.ZInt(3).ZVal(), phpv.ZString(hcd.writtenData).ZVal())
				// Index 4: seed (uint32)
				arr.OffsetSet(ctx, phpv.ZInt(4).ZVal(), phpv.ZInt(int64(int32(hcd.seed))).ZVal())
				// Index 5: seed64 (stored as two int32s to preserve value)
				arr.OffsetSet(ctx, phpv.ZInt(5).ZVal(), phpv.ZInt(int64(hcd.seed64)).ZVal())
				// Index 6: secret (raw bytes, empty if not used)
				arr.OffsetSet(ctx, phpv.ZInt(6).ZVal(), phpv.ZString(hcd.secret).ZVal())

				return arr.ZVal(), nil
			}),
		},
		"__unserialize": {
			Name:      "__unserialize",
			Modifiers: phpv.ZAttrPublic,
			Method: phpobj.NativeMethod(func(ctx phpv.Context, o *phpobj.ZObject, args []*phpv.ZVal) (*phpv.ZVal, error) {
				if len(args) == 0 {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "HashContext unserialization failed")
				}
				arr := args[0].AsArray(ctx)
				if arr == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "HashContext unserialization failed")
				}

				// Extract fields
				algoVal, _ := arr.OffsetGet(ctx, phpv.ZInt(0))
				optionsVal, _ := arr.OffsetGet(ctx, phpv.ZInt(1))
				keyVal, _ := arr.OffsetGet(ctx, phpv.ZInt(2))
				writtenDataVal, _ := arr.OffsetGet(ctx, phpv.ZInt(3))
				seedVal, _ := arr.OffsetGet(ctx, phpv.ZInt(4))
				seed64Val, _ := arr.OffsetGet(ctx, phpv.ZInt(5))
				secretVal, _ := arr.OffsetGet(ctx, phpv.ZInt(6))

				if algoVal == nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "HashContext unserialization failed")
				}

				algo := phpv.ZString(algoVal.AsString(ctx))

				isHmac := false
				if optionsVal != nil {
					isHmac = optionsVal.AsInt(ctx) == 1
				}

				var hmacKey []byte
				if keyVal != nil && keyVal.GetType() == phpv.ZtString {
					hmacKey = []byte(keyVal.Value().(phpv.ZString))
				}

				var writtenData []byte
				if writtenDataVal != nil && writtenDataVal.GetType() == phpv.ZtString {
					writtenData = []byte(writtenDataVal.Value().(phpv.ZString))
				}

				var seed uint32
				if seedVal != nil {
					seed = uint32(int32(seedVal.AsInt(ctx)))
				}

				var seed64 uint64
				if seed64Val != nil {
					seed64 = uint64(seed64Val.AsInt(ctx))
				}

				var secret []byte
				if secretVal != nil && secretVal.GetType() == phpv.ZtString {
					secret = []byte(secretVal.Value().(phpv.ZString))
					if len(secret) == 0 {
						secret = nil
					}
				}

				hcd, err := recreateHashContext(algo, isHmac, hmacKey, seed, seed64, secret, writtenData)
				if err != nil {
					return nil, phpobj.ThrowError(ctx, phpobj.Exception, "HashContext unserialization failed: unknown algorithm")
				}

				o.SetOpaque(HashContext, hcd)
				return nil, nil
			}),
		},
	}
}
