package json

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	core.RegisterExt(&core.Ext{
		Name:    "json",
		Version: core.VERSION,
		Classes: []*core.ZClass{
			JsonSerializable,
		},
		Functions: map[string]*core.ExtFunction{
			"json_decode": &core.ExtFunction{Func: fncJsonDecode, Args: []*core.ExtFunctionArg{}},
			"json_encode": &core.ExtFunction{Func: fncJsonEncode, Args: []*core.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{
			"JSON_BIGINT_AS_STRING":            phpv.ZInt(BigintAsString),
			"JSON_ERROR_CTRL_CHAR":             phpv.ZInt(ErrCtrlChar),
			"JSON_ERROR_DEPTH":                 phpv.ZInt(ErrDepth),
			"JSON_ERROR_INF_OR_NAN":            phpv.ZInt(ErrInfOrNan),
			"JSON_ERROR_INVALID_PROPERTY_NAME": phpv.ZInt(ErrInvalidPropName),
			"JSON_ERROR_NONE":                  phpv.ZInt(ErrNone),
			"JSON_ERROR_RECURSION":             phpv.ZInt(ErrRecursion),
			"JSON_ERROR_STATE_MISMATCH":        phpv.ZInt(ErrStateMismatch),
			"JSON_ERROR_SYNTAX":                phpv.ZInt(ErrSyntax),
			"JSON_ERROR_UNSUPPORTED_TYPE":      phpv.ZInt(ErrUnsupportedType),
			"JSON_ERROR_UTF16":                 phpv.ZInt(ErrUtf16),
			"JSON_ERROR_UTF8":                  phpv.ZInt(ErrUtf8),
			"JSON_FORCE_OBJECT":                phpv.ZInt(ForceObject),
			"JSON_HEX_AMP":                     phpv.ZInt(HexAmp),
			"JSON_HEX_APOS":                    phpv.ZInt(HexApos),
			"JSON_HEX_QUOT":                    phpv.ZInt(HexQuot),
			"JSON_HEX_TAG":                     phpv.ZInt(HexTag),
			"JSON_INVALID_UTF8_IGNORE":         phpv.ZInt(InvalidUtf8Ignore),
			"JSON_INVALID_UTF8_SUBSTITUTE":     phpv.ZInt(InvalidUtf8Substitute),
			"JSON_NUMERIC_CHECK":               phpv.ZInt(NumericCheck),
			"JSON_OBJECT_AS_ARRAY":             phpv.ZInt(ObjectAsArray),
			"JSON_PARTIAL_OUTPUT_ON_ERROR":     phpv.ZInt(PartialOutputOnError),
			"JSON_PRESERVE_ZERO_FRACTION":      phpv.ZInt(PreserveZeroFraction),
			"JSON_PRETTY_PRINT":                phpv.ZInt(PrettyPrint),
			"JSON_UNESCAPED_LINE_TERMINATORS":  phpv.ZInt(UnescapedEOL),
			"JSON_UNESCAPED_SLASHES":           phpv.ZInt(UnescapedSlashes),
			"JSON_UNESCAPED_UNICODE":           phpv.ZInt(UnescapedUnicode),
		},
	})
}
