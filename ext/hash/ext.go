package hash

import (
	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpctx"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
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
			"hash": {Func: fncHash, Args: []*phpctx.ExtFunctionArg{
				{ArgName: "algo"},
				{ArgName: "data"},
				{ArgName: "binary", Optional: true},
				{ArgName: "options", Optional: true},
			}},
			"hash_algos":      {Func: fncHashAlgos, Args: []*phpctx.ExtFunctionArg{}},
			"hash_copy":       {Func: fncHashCopy, Args: []*phpctx.ExtFunctionArg{}},
			"hash_equals": {Func: fncHashEquals, Args: []*phpctx.ExtFunctionArg{
				{ArgName: "known_string", Sensitive: true},
				{ArgName: "user_string", Sensitive: true},
			}},
			"hash_file":       {Func: fncHashFile, Args: []*phpctx.ExtFunctionArg{}},
			"hash_final":      {Func: fncHashFinal, Args: []*phpctx.ExtFunctionArg{}},
			"hash_hkdf":       {Func: fncHashHkdf, Args: []*phpctx.ExtFunctionArg{}},
			"hash_hmac": {Func: fncHashHmac, Args: []*phpctx.ExtFunctionArg{
				{ArgName: "algo"},
				{ArgName: "data"},
				{ArgName: "key", Sensitive: true},
			}},
			"hash_hmac_algos": {Func: fncHashHmacAlgos, Args: []*phpctx.ExtFunctionArg{}},
			"hash_hmac_file":  {Func: fncHashHmacFile, Args: []*phpctx.ExtFunctionArg{}},
			"hash_init": {Func: fncHashInit, Args: []*phpctx.ExtFunctionArg{
				{ArgName: "algo"},
				{ArgName: "flags", Optional: true},
				{ArgName: "key", Optional: true},
				{ArgName: "options", Optional: true},
			}},
			"hash_pbkdf2":        {Func: fncHashPbkdf2, Args: []*phpctx.ExtFunctionArg{}},
			"hash_update":        {Func: fncHashUpdate, Args: []*phpctx.ExtFunctionArg{}},
			"hash_update_file":   {Func: fncHashUpdateFile, Args: []*phpctx.ExtFunctionArg{}},
			"hash_update_stream": {Func: fncHashUpdateStream, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{
			"HASH_HMAC": HASH_HMAC,
		},
	})
}
