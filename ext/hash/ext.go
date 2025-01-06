package hash

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "hash",
		Version: core.VERSION,
		Classes: []*phpobj.ZClass{
			HashContext,
		},
		// Note: ExtFunctionArg is currently unused
		Functions: map[string]*phpctx.ExtFunction{
			"hash":        {Func: fncHash, Args: []*phpctx.ExtFunctionArg{}},
			"hash_algos":  {Func: fncHashAlgos, Args: []*phpctx.ExtFunctionArg{}},
			"hash_equals": {Func: fncHashEquals, Args: []*phpctx.ExtFunctionArg{}},
			"hash_final":  {Func: fncHashFinal, Args: []*phpctx.ExtFunctionArg{}},
			"hash_hkdf":   {Func: fncHashHkdf, Args: []*phpctx.ExtFunctionArg{}},
			"hash_hmac":   {Func: fncHashHmac, Args: []*phpctx.ExtFunctionArg{}},
			"hash_init":   {Func: fncHashInit, Args: []*phpctx.ExtFunctionArg{}},
			"hash_pbkdf2": {Func: fncHashPbkdf2, Args: []*phpctx.ExtFunctionArg{}},
			"hash_update": {Func: fncHashUpdate, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{
			"HASH_HMAC": HASH_HMAC,
		},
	})
}
