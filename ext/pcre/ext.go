package pcre

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
)

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "pcre",
		Version: core.VERSION,
		Classes: []phpv.ZClass{},
		// Note: ExtFunctionArg is currently unused
		Functions: map[string]*phpctx.ExtFunction{
			"preg_match":   {Func: pregMatch, Args: []*phpctx.ExtFunctionArg{}},
			"preg_quote":   {Func: pregQuote, Args: []*phpctx.ExtFunctionArg{}},
			"preg_replace": {Func: pregReplace, Args: []*phpctx.ExtFunctionArg{}},
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
		},
	})
}
