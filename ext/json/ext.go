package json

import (
	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpctx"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "json",
		Version: core.VERSION,
		Classes: []*phpobj.ZClass{
			JsonSerializable,
			JsonException,
		},
		// Note: ExtFunctionArg is currently unused
		Functions: map[string]*phpctx.ExtFunction{
			"json_decode":         {Func: fncJsonDecode, MinArgs: 1, MaxArgs: 4},
			"json_encode":         {Func: fncJsonEncode, MinArgs: 1, MaxArgs: 3},
			"json_last_error":     {Func: fncJsonLastError, ZeroArgs: true},
			"json_last_error_msg": {Func: fncJsonLastErrorMsg, ZeroArgs: true},
			"json_validate":       {Func: fncJsonValidate, MinArgs: 1, MaxArgs: 3},
		},
		Constants: map[phpv.ZString]phpv.Val{
			"JSON_BIGINT_AS_STRING":            JSON_BIGINT_AS_STRING,
			"JSON_ERROR_CTRL_CHAR":             JSON_ERROR_CTRL_CHAR,
			"JSON_ERROR_DEPTH":                 JSON_ERROR_DEPTH,
			"JSON_ERROR_INF_OR_NAN":            JSON_ERROR_INF_OR_NAN,
			"JSON_ERROR_INVALID_PROPERTY_NAME": JSON_ERROR_INVALID_PROPERTY_NAME,
			"JSON_ERROR_NONE":                  JSON_ERROR_NONE,
			"JSON_ERROR_RECURSION":             JSON_ERROR_RECURSION,
			"JSON_ERROR_STATE_MISMATCH":        JSON_ERROR_STATE_MISMATCH,
			"JSON_ERROR_SYNTAX":                JSON_ERROR_SYNTAX,
			"JSON_ERROR_UNSUPPORTED_TYPE":      JSON_ERROR_UNSUPPORTED_TYPE,
			"JSON_ERROR_NON_BACKED_ENUM":       JSON_ERROR_NON_BACKED_ENUM,
			"JSON_ERROR_UTF16":                 JSON_ERROR_UTF16,
			"JSON_ERROR_UTF8":                  JSON_ERROR_UTF8,
			"JSON_FORCE_OBJECT":                JSON_FORCE_OBJECT,
			"JSON_HEX_AMP":                     JSON_HEX_AMP,
			"JSON_HEX_APOS":                    JSON_HEX_APOS,
			"JSON_HEX_QUOT":                    JSON_HEX_QUOT,
			"JSON_HEX_TAG":                     JSON_HEX_TAG,
			"JSON_INVALID_UTF8_IGNORE":         JSON_INVALID_UTF8_IGNORE,
			"JSON_INVALID_UTF8_SUBSTITUTE":     JSON_INVALID_UTF8_SUBSTITUTE,
			"JSON_THROW_ON_ERROR":              phpv.ZInt(ThrowOnError),
			"JSON_NUMERIC_CHECK":               JSON_NUMERIC_CHECK,
			"JSON_OBJECT_AS_ARRAY":             JSON_OBJECT_AS_ARRAY,
			"JSON_PARTIAL_OUTPUT_ON_ERROR":     JSON_PARTIAL_OUTPUT_ON_ERROR,
			"JSON_PRESERVE_ZERO_FRACTION":      JSON_PRESERVE_ZERO_FRACTION,
			"JSON_PRETTY_PRINT":                JSON_PRETTY_PRINT,
			"JSON_UNESCAPED_LINE_TERMINATORS":  JSON_UNESCAPED_LINE_TERMINATORS,
			"JSON_UNESCAPED_SLASHES":           JSON_UNESCAPED_SLASHES,
			"JSON_UNESCAPED_UNICODE":           JSON_UNESCAPED_UNICODE,
		},
	})
}
