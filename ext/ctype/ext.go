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
		Functions: map[string]*phpctx.ExtFunction{
			"ctype_alnum":  &phpctx.ExtFunction{Func: ctypeAlnum, Args: []*phpctx.ExtFunctionArg{}},
			"ctype_alpha":  &phpctx.ExtFunction{Func: ctypeAlpha, Args: []*phpctx.ExtFunctionArg{}},
			"ctype_cntrl":  &phpctx.ExtFunction{Func: ctypeCntrl, Args: []*phpctx.ExtFunctionArg{}},
			"ctype_digit":  &phpctx.ExtFunction{Func: ctypeDigit, Args: []*phpctx.ExtFunctionArg{}},
			"ctype_graph":  &phpctx.ExtFunction{Func: ctypeGraph, Args: []*phpctx.ExtFunctionArg{}},
			"ctype_lower":  &phpctx.ExtFunction{Func: ctypeLower, Args: []*phpctx.ExtFunctionArg{}},
			"ctype_print":  &phpctx.ExtFunction{Func: ctypePrint, Args: []*phpctx.ExtFunctionArg{}},
			"ctype_punct":  &phpctx.ExtFunction{Func: ctypePunct, Args: []*phpctx.ExtFunctionArg{}},
			"ctype_space":  &phpctx.ExtFunction{Func: ctypeSpace, Args: []*phpctx.ExtFunctionArg{}},
			"ctype_upper":  &phpctx.ExtFunction{Func: ctypeUpper, Args: []*phpctx.ExtFunctionArg{}},
			"ctype_xdigit": &phpctx.ExtFunction{Func: ctypeXdigit, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{},
	})
}
