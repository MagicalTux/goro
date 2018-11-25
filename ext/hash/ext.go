package hash

import "github.com/MagicalTux/goro/core"

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	core.RegisterExt(&core.Ext{
		Name:    "hash",
		Version: core.VERSION,
		Classes: []*core.ZClass{
			HashContext,
		},
		Functions: map[string]*core.ExtFunction{
			"hash":        &core.ExtFunction{Func: fncHash, Args: []*core.ExtFunctionArg{}},
			"hash_algos":  &core.ExtFunction{Func: fncHashAlgos, Args: []*core.ExtFunctionArg{}},
			"hash_equals": &core.ExtFunction{Func: fncHashEquals, Args: []*core.ExtFunctionArg{}},
			"hash_final":  &core.ExtFunction{Func: fncHashFinal, Args: []*core.ExtFunctionArg{}},
			"hash_hkdf":   &core.ExtFunction{Func: fncHashHkdf, Args: []*core.ExtFunctionArg{}},
			"hash_hmac":   &core.ExtFunction{Func: fncHashHmac, Args: []*core.ExtFunctionArg{}},
			"hash_init":   &core.ExtFunction{Func: fncHashInit, Args: []*core.ExtFunctionArg{}},
			"hash_pbkdf2": &core.ExtFunction{Func: fncHashPbkdf2, Args: []*core.ExtFunctionArg{}},
			"hash_update": &core.ExtFunction{Func: fncHashUpdate, Args: []*core.ExtFunctionArg{}},
		},
		Constants: map[core.ZString]*core.ZVal{
			"HASH_HMAC": core.ZInt(1).ZVal(),
		},
	})
}
