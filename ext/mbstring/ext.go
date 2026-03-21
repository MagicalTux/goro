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
			"mb_check_encoding":       {Func: fncMbCheckEncoding, Args: []*phpctx.ExtFunctionArg{}},
			"mb_convert_encoding":     {Func: fncMbConvertEncoding, Args: []*phpctx.ExtFunctionArg{}},
			"mb_convert_case":         {Func: fncMbConvertCase, Args: []*phpctx.ExtFunctionArg{}},
			"mb_convert_variables":    {Func: fncMbConvertVariables, Args: []*phpctx.ExtFunctionArg{}},
			"mb_detect_encoding":      {Func: fncMbDetectEncoding, Args: []*phpctx.ExtFunctionArg{}},
			"mb_detect_order":         {Func: fncMbDetectOrder, Args: []*phpctx.ExtFunctionArg{}},
			"mb_encoding_aliases":     {Func: fncMbEncodingAliases, Args: []*phpctx.ExtFunctionArg{}},
			"mb_get_info":             {Func: fncMbGetInfo, Args: []*phpctx.ExtFunctionArg{}},
			"mb_http_input":           {Func: fncMbHttpInput, Args: []*phpctx.ExtFunctionArg{}},
			"mb_http_output":          {Func: fncMbHttpOutput, Args: []*phpctx.ExtFunctionArg{}},
			"mb_internal_encoding":    {Func: fncMbInternalEncoding, Args: []*phpctx.ExtFunctionArg{}},
			"mb_language":             {Func: fncMbLanguage, Args: []*phpctx.ExtFunctionArg{}},
			"mb_list_encodings":       {Func: fncMbListEncodings, Args: []*phpctx.ExtFunctionArg{}},
			"mb_ord":                  {Func: fncMbOrd, Args: []*phpctx.ExtFunctionArg{}},
			"mb_chr":                  {Func: fncMbChr, Args: []*phpctx.ExtFunctionArg{}},
			"mb_output_handler":       {Func: fncMbOutputHandler, Args: []*phpctx.ExtFunctionArg{}},
			"mb_parse_str":            {Func: fncMbParseStr, Args: []*phpctx.ExtFunctionArg{}},
			"mb_preferred_mime_name":  {Func: fncMbPreferredMimeName, Args: []*phpctx.ExtFunctionArg{}},
			"mb_scrub":                {Func: fncMbScrub, Args: []*phpctx.ExtFunctionArg{}},
			"mb_str_pad":              {Func: fncMbStrPad, Args: []*phpctx.ExtFunctionArg{}},
			"mb_str_split":            {Func: fncMbStrSplit, Args: []*phpctx.ExtFunctionArg{}},
			"mb_strcut":               {Func: fncMbStrcut, Args: []*phpctx.ExtFunctionArg{}},
			"mb_strimwidth":           {Func: fncMbStrimwidth, Args: []*phpctx.ExtFunctionArg{}},
			"mb_strwidth":             {Func: fncMbStrwidth, Args: []*phpctx.ExtFunctionArg{}},
			"mb_stripos":              {Func: fncMbStripos, Args: []*phpctx.ExtFunctionArg{}},
			"mb_stristr":              {Func: fncMbStristr, Args: []*phpctx.ExtFunctionArg{}},
			"mb_strlen":               {Func: fncMbStrlen, Args: []*phpctx.ExtFunctionArg{}},
			"mb_strpos":               {Func: fncMbStrpos, Args: []*phpctx.ExtFunctionArg{}},
			"mb_strrchr":              {Func: fncMbStrrchr, Args: []*phpctx.ExtFunctionArg{}},
			"mb_strrichr":             {Func: fncMbStrrichr, Args: []*phpctx.ExtFunctionArg{}},
			"mb_strripos":             {Func: fncMbStrripos, Args: []*phpctx.ExtFunctionArg{}},
			"mb_strrpos":              {Func: fncMbStrrpos, Args: []*phpctx.ExtFunctionArg{}},
			"mb_strstr":               {Func: fncMbStrstr, Args: []*phpctx.ExtFunctionArg{}},
			"mb_strtolower":           {Func: fncMbStrtolower, Args: []*phpctx.ExtFunctionArg{}},
			"mb_strtoupper":           {Func: fncMbStrtoupper, Args: []*phpctx.ExtFunctionArg{}},
			"mb_substitute_character": {Func: fncMbSubstituteCharacter, Args: []*phpctx.ExtFunctionArg{}},
			"mb_substr":               {Func: fncMbSubstr, Args: []*phpctx.ExtFunctionArg{}},
			"mb_substr_count":         {Func: fncMbSubstrCount, Args: []*phpctx.ExtFunctionArg{}},
			"mb_trim":                 {Func: fncMbTrim, Args: []*phpctx.ExtFunctionArg{}},
			"mb_ltrim":                {Func: fncMbLtrim, Args: []*phpctx.ExtFunctionArg{}},
			"mb_rtrim":                {Func: fncMbRtrim, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{
			"MB_CASE_UPPER":           phpv.ZInt(0),
			"MB_CASE_LOWER":           phpv.ZInt(1),
			"MB_CASE_TITLE":           phpv.ZInt(2),
			"MB_CASE_FOLD":            phpv.ZInt(0),
			"MB_CASE_UPPER_SIMPLE":    phpv.ZInt(0),
			"MB_CASE_LOWER_SIMPLE":    phpv.ZInt(1),
			"MB_CASE_FOLD_SIMPLE":     phpv.ZInt(0),
			"MB_ONIGURUMA_VERSION":    phpv.ZString("6.9.9"),
		},
	})
}
