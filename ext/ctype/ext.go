package ctype

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
)

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "ctype",
		Version: core.VERSION,
		Classes: []phpv.ZClass{},
		// Note: ExtFunctionArg is currently unused
		Functions: map[string]*phpctx.ExtFunction{
			"ctype_alnum":  {Func: ctypeAlnum, Args: []*phpctx.ExtFunctionArg{}},
			"ctype_alpha":  {Func: ctypeAlpha, Args: []*phpctx.ExtFunctionArg{}},
			"ctype_cntrl":  {Func: ctypeCntrl, Args: []*phpctx.ExtFunctionArg{}},
			"ctype_digit":  {Func: ctypeDigit, Args: []*phpctx.ExtFunctionArg{}},
			"ctype_graph":  {Func: ctypeGraph, Args: []*phpctx.ExtFunctionArg{}},
			"ctype_lower":  {Func: ctypeLower, Args: []*phpctx.ExtFunctionArg{}},
			"ctype_print":  {Func: ctypePrint, Args: []*phpctx.ExtFunctionArg{}},
			"ctype_punct":  {Func: ctypePunct, Args: []*phpctx.ExtFunctionArg{}},
			"ctype_space":  {Func: ctypeSpace, Args: []*phpctx.ExtFunctionArg{}},
			"ctype_upper":  {Func: ctypeUpper, Args: []*phpctx.ExtFunctionArg{}},
			"ctype_xdigit": {Func: ctypeXdigit, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{},
	})
}
