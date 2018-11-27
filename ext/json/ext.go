package json

import "github.com/MagicalTux/goro/core"

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
		Constants: map[core.ZString]*core.ZVal{
			"JSON_BIGINT_AS_STRING":            core.ZInt(BigintAsString).ZVal(),
			"JSON_ERROR_CTRL_CHAR":             core.ZInt(ErrCtrlChar).ZVal(),
			"JSON_ERROR_DEPTH":                 core.ZInt(ErrDepth).ZVal(),
			"JSON_ERROR_INF_OR_NAN":            core.ZInt(ErrInfOrNan).ZVal(),
			"JSON_ERROR_INVALID_PROPERTY_NAME": core.ZInt(ErrInvalidPropName).ZVal(),
			"JSON_ERROR_NONE":                  core.ZInt(ErrNone).ZVal(),
			"JSON_ERROR_RECURSION":             core.ZInt(ErrRecursion).ZVal(),
			"JSON_ERROR_STATE_MISMATCH":        core.ZInt(ErrStateMismatch).ZVal(),
			"JSON_ERROR_SYNTAX":                core.ZInt(ErrSyntax).ZVal(),
			"JSON_ERROR_UNSUPPORTED_TYPE":      core.ZInt(ErrUnsupportedType).ZVal(),
			"JSON_ERROR_UTF16":                 core.ZInt(ErrUtf16).ZVal(),
			"JSON_ERROR_UTF8":                  core.ZInt(ErrUtf8).ZVal(),
			"JSON_FORCE_OBJECT":                core.ZInt(ForceObject).ZVal(),
			"JSON_HEX_AMP":                     core.ZInt(HexAmp).ZVal(),
			"JSON_HEX_APOS":                    core.ZInt(HexApos).ZVal(),
			"JSON_HEX_QUOT":                    core.ZInt(HexQuot).ZVal(),
			"JSON_HEX_TAG":                     core.ZInt(HexTag).ZVal(),
			"JSON_INVALID_UTF8_IGNORE":         core.ZInt(InvalidUtf8Ignore).ZVal(),
			"JSON_INVALID_UTF8_SUBSTITUTE":     core.ZInt(InvalidUtf8Substitute).ZVal(),
			"JSON_NUMERIC_CHECK":               core.ZInt(NumericCheck).ZVal(),
			"JSON_OBJECT_AS_ARRAY":             core.ZInt(ObjectAsArray).ZVal(),
			"JSON_PARTIAL_OUTPUT_ON_ERROR":     core.ZInt(PartialOutputOnError).ZVal(),
			"JSON_PRESERVE_ZERO_FRACTION":      core.ZInt(PreserveZeroFraction).ZVal(),
			"JSON_PRETTY_PRINT":                core.ZInt(PrettyPrint).ZVal(),
			"JSON_UNESCAPED_LINE_TERMINATORS":  core.ZInt(UnescapedEOL).ZVal(),
			"JSON_UNESCAPED_SLASHES":           core.ZInt(UnescapedSlashes).ZVal(),
			"JSON_UNESCAPED_UNICODE":           core.ZInt(UnescapedUnicode).ZVal(),
		},
	})
}
