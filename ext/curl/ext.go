package curl

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// CURLOPT constants — PHP values
const (
	CURLOPT_URL             = 10002
	CURLOPT_RETURNTRANSFER  = 19913
	CURLOPT_POST            = 47
	CURLOPT_POSTFIELDS      = 10015
	CURLOPT_HTTPHEADER      = 10023
	CURLOPT_USERAGENT       = 10018
	CURLOPT_TIMEOUT         = 13
	CURLOPT_CONNECTTIMEOUT  = 78
	CURLOPT_FOLLOWLOCATION  = 52
	CURLOPT_MAXREDIRS       = 68
	CURLOPT_SSL_VERIFYPEER  = 64
	CURLOPT_SSL_VERIFYHOST  = 81
	CURLOPT_CUSTOMREQUEST   = 10036
	CURLOPT_NOBODY          = 44
	CURLOPT_HEADER          = 42
	CURLOPT_ENCODING        = 10102
	CURLOPT_HEADERFUNCTION  = 20079
	CURLOPT_WRITEFUNCTION   = 20011
	CURLOPT_HTTPAUTH        = 107
	CURLOPT_USERPWD         = 10005
)

// CURLINFO constants
const (
	CURLINFO_HTTP_CODE    = 2097154
	CURLINFO_CONTENT_TYPE = 1048594
	CURLINFO_EFFECTIVE_URL = 1048577
	CURLINFO_TOTAL_TIME   = 3145731
)

// CURLE error codes
const (
	CURLE_OK                    = 0
	CURLE_UNSUPPORTED_PROTOCOL  = 1
	CURLE_URL_MALFORMAT         = 3
	CURLE_COULDNT_RESOLVE_HOST  = 6
	CURLE_COULDNT_CONNECT       = 7
	CURLE_HTTP_RETURNED_ERROR   = 22
	CURLE_OPERATION_TIMEDOUT    = 28
	CURLE_SSL_CONNECT_ERROR     = 35
)

func init() {
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "curl",
		Version: core.VERSION,
		Classes: []*phpobj.ZClass{
			CurlHandle,
		},
		Functions: map[string]*phpctx.ExtFunction{
			"curl_init":        {Func: fncCurlInit, Args: []*phpctx.ExtFunctionArg{}},
			"curl_setopt":      {Func: fncCurlSetopt, Args: []*phpctx.ExtFunctionArg{}},
			"curl_setopt_array": {Func: fncCurlSetoptArray, Args: []*phpctx.ExtFunctionArg{}},
			"curl_exec":        {Func: fncCurlExec, Args: []*phpctx.ExtFunctionArg{}},
			"curl_getinfo":     {Func: fncCurlGetinfo, Args: []*phpctx.ExtFunctionArg{}},
			"curl_errno":       {Func: fncCurlErrno, Args: []*phpctx.ExtFunctionArg{}},
			"curl_error":       {Func: fncCurlError, Args: []*phpctx.ExtFunctionArg{}},
			"curl_close":       {Func: fncCurlClose, Args: []*phpctx.ExtFunctionArg{}},
			"curl_reset":       {Func: fncCurlReset, Args: []*phpctx.ExtFunctionArg{}},
			"curl_version":     {Func: fncCurlVersion, Args: []*phpctx.ExtFunctionArg{}},
			"curl_strerror":    {Func: fncCurlStrerror, Args: []*phpctx.ExtFunctionArg{}},
			"curl_escape":      {Func: fncCurlEscape, Args: []*phpctx.ExtFunctionArg{}},
			"curl_unescape":    {Func: fncCurlUnescape, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{
			// CURLOPT constants
			"CURLOPT_URL":            phpv.ZInt(CURLOPT_URL),
			"CURLOPT_RETURNTRANSFER": phpv.ZInt(CURLOPT_RETURNTRANSFER),
			"CURLOPT_POST":           phpv.ZInt(CURLOPT_POST),
			"CURLOPT_POSTFIELDS":     phpv.ZInt(CURLOPT_POSTFIELDS),
			"CURLOPT_HTTPHEADER":     phpv.ZInt(CURLOPT_HTTPHEADER),
			"CURLOPT_USERAGENT":      phpv.ZInt(CURLOPT_USERAGENT),
			"CURLOPT_TIMEOUT":        phpv.ZInt(CURLOPT_TIMEOUT),
			"CURLOPT_CONNECTTIMEOUT": phpv.ZInt(CURLOPT_CONNECTTIMEOUT),
			"CURLOPT_FOLLOWLOCATION": phpv.ZInt(CURLOPT_FOLLOWLOCATION),
			"CURLOPT_MAXREDIRS":      phpv.ZInt(CURLOPT_MAXREDIRS),
			"CURLOPT_SSL_VERIFYPEER": phpv.ZInt(CURLOPT_SSL_VERIFYPEER),
			"CURLOPT_SSL_VERIFYHOST": phpv.ZInt(CURLOPT_SSL_VERIFYHOST),
			"CURLOPT_CUSTOMREQUEST":  phpv.ZInt(CURLOPT_CUSTOMREQUEST),
			"CURLOPT_NOBODY":         phpv.ZInt(CURLOPT_NOBODY),
			"CURLOPT_HEADER":         phpv.ZInt(CURLOPT_HEADER),
			"CURLOPT_ENCODING":       phpv.ZInt(CURLOPT_ENCODING),
			"CURLOPT_HEADERFUNCTION": phpv.ZInt(CURLOPT_HEADERFUNCTION),
			"CURLOPT_WRITEFUNCTION":  phpv.ZInt(CURLOPT_WRITEFUNCTION),
			"CURLOPT_HTTPAUTH":       phpv.ZInt(CURLOPT_HTTPAUTH),
			"CURLOPT_USERPWD":        phpv.ZInt(CURLOPT_USERPWD),

			// CURLINFO constants
			"CURLINFO_HTTP_CODE":     phpv.ZInt(CURLINFO_HTTP_CODE),
			"CURLINFO_CONTENT_TYPE":  phpv.ZInt(CURLINFO_CONTENT_TYPE),
			"CURLINFO_EFFECTIVE_URL": phpv.ZInt(CURLINFO_EFFECTIVE_URL),
			"CURLINFO_TOTAL_TIME":    phpv.ZInt(CURLINFO_TOTAL_TIME),

			// CURLE error codes
			"CURLE_OK":                   phpv.ZInt(CURLE_OK),
			"CURLE_UNSUPPORTED_PROTOCOL": phpv.ZInt(CURLE_UNSUPPORTED_PROTOCOL),
			"CURLE_URL_MALFORMAT":        phpv.ZInt(CURLE_URL_MALFORMAT),
			"CURLE_COULDNT_RESOLVE_HOST": phpv.ZInt(CURLE_COULDNT_RESOLVE_HOST),
			"CURLE_COULDNT_CONNECT":      phpv.ZInt(CURLE_COULDNT_CONNECT),
			"CURLE_HTTP_RETURNED_ERROR":  phpv.ZInt(CURLE_HTTP_RETURNED_ERROR),
			"CURLE_OPERATION_TIMEDOUT":   phpv.ZInt(CURLE_OPERATION_TIMEDOUT),
			"CURLE_SSL_CONNECT_ERROR":    phpv.ZInt(CURLE_SSL_CONNECT_ERROR),
		},
	})
}
