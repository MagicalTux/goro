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
		Constants: map[phpv.ZString]*phpv.ZVal{
			"JSON_BIGINT_AS_STRING":            phpv.ZInt(BigintAsString).ZVal(),
			"JSON_ERROR_CTRL_CHAR":             phpv.ZInt(ErrCtrlChar).ZVal(),
			"JSON_ERROR_DEPTH":                 phpv.ZInt(ErrDepth).ZVal(),
			"JSON_ERROR_INF_OR_NAN":            phpv.ZInt(ErrInfOrNan).ZVal(),
			"JSON_ERROR_INVALID_PROPERTY_NAME": phpv.ZInt(ErrInvalidPropName).ZVal(),
			"JSON_ERROR_NONE":                  phpv.ZInt(ErrNone).ZVal(),
			"JSON_ERROR_RECURSION":             phpv.ZInt(ErrRecursion).ZVal(),
			"JSON_ERROR_STATE_MISMATCH":        phpv.ZInt(ErrStateMismatch).ZVal(),
			"JSON_ERROR_SYNTAX":                phpv.ZInt(ErrSyntax).ZVal(),
			"JSON_ERROR_UNSUPPORTED_TYPE":      phpv.ZInt(ErrUnsupportedType).ZVal(),
			"JSON_ERROR_UTF16":                 phpv.ZInt(ErrUtf16).ZVal(),
			"JSON_ERROR_UTF8":                  phpv.ZInt(ErrUtf8).ZVal(),
			"JSON_FORCE_OBJECT":                phpv.ZInt(ForceObject).ZVal(),
			"JSON_HEX_AMP":                     phpv.ZInt(HexAmp).ZVal(),
			"JSON_HEX_APOS":                    phpv.ZInt(HexApos).ZVal(),
			"JSON_HEX_QUOT":                    phpv.ZInt(HexQuot).ZVal(),
			"JSON_HEX_TAG":                     phpv.ZInt(HexTag).ZVal(),
			"JSON_INVALID_UTF8_IGNORE":         phpv.ZInt(InvalidUtf8Ignore).ZVal(),
			"JSON_INVALID_UTF8_SUBSTITUTE":     phpv.ZInt(InvalidUtf8Substitute).ZVal(),
			"JSON_NUMERIC_CHECK":               phpv.ZInt(NumericCheck).ZVal(),
			"JSON_OBJECT_AS_ARRAY":             phpv.ZInt(ObjectAsArray).ZVal(),
			"JSON_PARTIAL_OUTPUT_ON_ERROR":     phpv.ZInt(PartialOutputOnError).ZVal(),
			"JSON_PRESERVE_ZERO_FRACTION":      phpv.ZInt(PreserveZeroFraction).ZVal(),
			"JSON_PRETTY_PRINT":                phpv.ZInt(PrettyPrint).ZVal(),
			"JSON_UNESCAPED_LINE_TERMINATORS":  phpv.ZInt(UnescapedEOL).ZVal(),
			"JSON_UNESCAPED_SLASHES":           phpv.ZInt(UnescapedSlashes).ZVal(),
			"JSON_UNESCAPED_UNICODE":           phpv.ZInt(UnescapedUnicode).ZVal(),
		},
	})
}
