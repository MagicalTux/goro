package mbstring

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
)

func init() {
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "mbstring",
		Version: core.VERSION,
		Functions: map[string]*phpctx.ExtFunction{
			"mb_check_encoding":      {Func: fncMbCheckEncoding, Args: []*phpctx.ExtFunctionArg{}},
			"mb_convert_encoding":    {Func: fncMbConvertEncoding, Args: []*phpctx.ExtFunctionArg{}},
			"mb_detect_encoding":     {Func: fncMbDetectEncoding, Args: []*phpctx.ExtFunctionArg{}},
			"mb_internal_encoding":   {Func: fncMbInternalEncoding, Args: []*phpctx.ExtFunctionArg{}},
			"mb_strlen":              {Func: fncMbStrlen, Args: []*phpctx.ExtFunctionArg{}},
			"mb_strpos":              {Func: fncMbStrpos, Args: []*phpctx.ExtFunctionArg{}},
			"mb_strrpos":             {Func: fncMbStrrpos, Args: []*phpctx.ExtFunctionArg{}},
			"mb_substr":              {Func: fncMbSubstr, Args: []*phpctx.ExtFunctionArg{}},
			"mb_strtolower":          {Func: fncMbStrtolower, Args: []*phpctx.ExtFunctionArg{}},
			"mb_strtoupper":          {Func: fncMbStrtoupper, Args: []*phpctx.ExtFunctionArg{}},
			"mb_substitute_character": {Func: fncMbSubstituteCharacter, Args: []*phpctx.ExtFunctionArg{}},
			"mb_substr_count":        {Func: fncMbSubstrCount, Args: []*phpctx.ExtFunctionArg{}},
			"mb_detect_order":        {Func: fncMbDetectOrder, Args: []*phpctx.ExtFunctionArg{}},
			"mb_language":            {Func: fncMbLanguage, Args: []*phpctx.ExtFunctionArg{}},
			"mb_str_split":           {Func: fncMbStrSplit, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{
			"MB_CASE_UPPER": phpv.ZInt(0),
			"MB_CASE_LOWER": phpv.ZInt(1),
			"MB_CASE_TITLE": phpv.ZInt(2),
		},
	})
}
