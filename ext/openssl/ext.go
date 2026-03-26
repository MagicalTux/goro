package openssl

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

func init() {
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "openssl",
		Version: core.VERSION,
		Classes: []*phpobj.ZClass{
			OpenSSLAsymmetricKey,
		},
		Functions: map[string]*phpctx.ExtFunction{
			"openssl_encrypt":             {Func: fncOpensslEncrypt, Args: []*phpctx.ExtFunctionArg{}},
			"openssl_decrypt":             {Func: fncOpensslDecrypt, Args: []*phpctx.ExtFunctionArg{}},
			"openssl_cipher_iv_length":    {Func: fncOpensslCipherIvLength, Args: []*phpctx.ExtFunctionArg{}},
			"openssl_cipher_key_length":   {Func: fncOpensslCipherKeyLength, Args: []*phpctx.ExtFunctionArg{}},
			"openssl_get_cipher_methods":  {Func: fncOpensslGetCipherMethods, Args: []*phpctx.ExtFunctionArg{}},
			"openssl_digest":              {Func: fncOpensslDigest, Args: []*phpctx.ExtFunctionArg{}},
			"openssl_get_md_methods":      {Func: fncOpensslGetMdMethods, Args: []*phpctx.ExtFunctionArg{}},
			"openssl_random_pseudo_bytes": {Func: fncOpensslRandomPseudoBytes, Args: []*phpctx.ExtFunctionArg{}},
			"openssl_sign":                {Func: fncOpensslSign, Args: []*phpctx.ExtFunctionArg{}},
			"openssl_verify":              {Func: fncOpensslVerify, Args: []*phpctx.ExtFunctionArg{}},
			"openssl_pkey_new":            {Func: fncOpensslPkeyNew, Args: []*phpctx.ExtFunctionArg{}},
			"openssl_pkey_export":         {Func: fncOpensslPkeyExport, Args: []*phpctx.ExtFunctionArg{}},
			"openssl_pkey_get_details":    {Func: fncOpensslPkeyGetDetails, Args: []*phpctx.ExtFunctionArg{}},
			"openssl_pkey_get_public":     {Func: fncOpensslPkeyGetPublic, Args: []*phpctx.ExtFunctionArg{}},
			"openssl_pkey_get_private":    {Func: fncOpensslPkeyGetPrivate, Args: []*phpctx.ExtFunctionArg{}},
			"openssl_error_string":        {Func: fncOpensslErrorString, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{
			// Padding options
			"OPENSSL_RAW_DATA":   phpv.ZInt(OPENSSL_RAW_DATA),
			"OPENSSL_ZERO_PADDING": phpv.ZInt(OPENSSL_ZERO_PADDING),
			"OPENSSL_DONT_ZERO_PAD_KEY": phpv.ZInt(OPENSSL_DONT_ZERO_PAD_KEY),

			// Key types
			"OPENSSL_KEYTYPE_RSA": phpv.ZInt(OPENSSL_KEYTYPE_RSA),
			"OPENSSL_KEYTYPE_EC":  phpv.ZInt(OPENSSL_KEYTYPE_EC),

			// Signature algorithms
			"OPENSSL_ALGO_SHA1":   phpv.ZInt(OPENSSL_ALGO_SHA1),
			"OPENSSL_ALGO_SHA224": phpv.ZInt(OPENSSL_ALGO_SHA224),
			"OPENSSL_ALGO_SHA256": phpv.ZInt(OPENSSL_ALGO_SHA256),
			"OPENSSL_ALGO_SHA384": phpv.ZInt(OPENSSL_ALGO_SHA384),
			"OPENSSL_ALGO_SHA512": phpv.ZInt(OPENSSL_ALGO_SHA512),

			// Version
			"OPENSSL_VERSION_TEXT":   phpv.ZString("OpenSSL 3.0.0 (goro)"),
			"OPENSSL_VERSION_NUMBER": phpv.ZInt(0x30000000),
		},
	})
}

// OPENSSL options flags
const (
	OPENSSL_RAW_DATA        = 1
	OPENSSL_ZERO_PADDING    = 2
	OPENSSL_DONT_ZERO_PAD_KEY = 4
)

// Key types
const (
	OPENSSL_KEYTYPE_RSA = 0
	OPENSSL_KEYTYPE_EC  = 3
)

// Signature algorithms
const (
	OPENSSL_ALGO_SHA1   = 1
	OPENSSL_ALGO_SHA224 = 6
	OPENSSL_ALGO_SHA256 = 7
	OPENSSL_ALGO_SHA384 = 8
	OPENSSL_ALGO_SHA512 = 9
)
