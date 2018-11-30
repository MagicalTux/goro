package hash

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
)

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "hash",
		Version: core.VERSION,
		Classes: []phpv.ZClass{
			HashContext,
		},
		Functions: map[string]*phpctx.ExtFunction{
			"hash":        &phpctx.ExtFunction{Func: fncHash, Args: []*phpctx.ExtFunctionArg{}},
			"hash_algos":  &phpctx.ExtFunction{Func: fncHashAlgos, Args: []*phpctx.ExtFunctionArg{}},
			"hash_equals": &phpctx.ExtFunction{Func: fncHashEquals, Args: []*phpctx.ExtFunctionArg{}},
			"hash_final":  &phpctx.ExtFunction{Func: fncHashFinal, Args: []*phpctx.ExtFunctionArg{}},
			"hash_hkdf":   &phpctx.ExtFunction{Func: fncHashHkdf, Args: []*phpctx.ExtFunctionArg{}},
			"hash_hmac":   &phpctx.ExtFunction{Func: fncHashHmac, Args: []*phpctx.ExtFunctionArg{}},
			"hash_init":   &phpctx.ExtFunction{Func: fncHashInit, Args: []*phpctx.ExtFunctionArg{}},
			"hash_pbkdf2": &phpctx.ExtFunction{Func: fncHashPbkdf2, Args: []*phpctx.ExtFunctionArg{}},
			"hash_update": &phpctx.ExtFunction{Func: fncHashUpdate, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{
			"HASH_HMAC": phpv.ZInt(1),
		},
	})
}
