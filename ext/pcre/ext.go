package pcre

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "pcre",
		Version: core.VERSION,
		Classes: []*phpobj.ZClass{},
		// Note: ExtFunctionArg is currently unused
		Functions: map[string]*phpctx.ExtFunction{
			"preg_match":                  {Func: pregMatch, Args: []*phpctx.ExtFunctionArg{}},
			"preg_match_all":              {Func: pregMatchAll, Args: []*phpctx.ExtFunctionArg{}},
			"preg_quote":                  {Func: pregQuote, Args: []*phpctx.ExtFunctionArg{}},
			"preg_replace":                {Func: pregReplace, Args: []*phpctx.ExtFunctionArg{}},
			"preg_replace_callback":       {Func: pregReplaceCallback, Args: []*phpctx.ExtFunctionArg{}},
			"preg_replace_callback_array": {Func: pregReplaceCallbackArray, Args: []*phpctx.ExtFunctionArg{}},
			"preg_split":                  {Func: pregSplit, Args: []*phpctx.ExtFunctionArg{}},
			"preg_grep":                   {Func: pregGrep, Args: []*phpctx.ExtFunctionArg{}},
			"preg_filter":                 {Func: pregFilter, Args: []*phpctx.ExtFunctionArg{}},
			"preg_last_error":             {Func: pregLastError, Args: []*phpctx.ExtFunctionArg{}},
			"preg_last_error_msg":         {Func: pregLastErrorMsg, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{
			"PREG_GREP_INVERT":          PREG_GREP_INVERT,
			"PREG_OFFSET_CAPTURE":       PREG_OFFSET_CAPTURE,
			"PREG_PATTERN_ORDER":        PREG_PATTERN_ORDER,
			"PREG_REPLACE_EVAL":         PREG_REPLACE_EVAL,
			"PREG_SET_ORDER":            PREG_SET_ORDER,
			"PREG_SPLIT_DELIM_CAPTURE":  PREG_SPLIT_DELIM_CAPTURE,
			"PREG_SPLIT_NO_EMPTY":       PREG_SPLIT_NO_EMPTY,
			"PREG_SPLIT_OFFSET_CAPTURE": PREG_SPLIT_OFFSET_CAPTURE,
			"PREG_UNMATCHED_AS_NULL":    PREG_UNMATCHED_AS_NULL,
			"PREG_NO_ERROR":              PREG_NO_ERROR,
			"PREG_INTERNAL_ERROR":        PREG_INTERNAL_ERROR,
			"PREG_BACKTRACK_LIMIT_ERROR": PREG_BACKTRACK_LIMIT_ERROR,
			"PREG_RECURSION_LIMIT_ERROR": PREG_RECURSION_LIMIT_ERROR,
			"PREG_BAD_UTF8_ERROR":        PREG_BAD_UTF8_ERROR,
			"PREG_BAD_UTF8_OFFSET_ERROR": PREG_BAD_UTF8_OFFSET_ERROR,
			"PREG_JIT_STACKLIMIT_ERROR":  PREG_JIT_STACKLIMIT_ERROR,
		},
	})
}
