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
		Functions: map[string]*phpctx.ExtFunction{
			"preg_quote":   {Func: pregQuote, Args: []*phpctx.ExtFunctionArg{}},
			"preg_replace": {Func: pregReplace, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{},
	})
}
