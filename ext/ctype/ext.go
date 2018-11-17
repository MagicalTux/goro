package ctype

import "github.com/MagicalTux/gophp/core"

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	core.RegisterExt(&core.Ext{
		Name: "ctype",
		Functions: map[string]*core.ExtFunction{
			"ctype_alnum":  &core.ExtFunction{Func: ctypeAlnum, Args: []*core.ExtFunctionArg{}},
			"ctype_alpha":  &core.ExtFunction{Func: ctypeAlpha, Args: []*core.ExtFunctionArg{}},
			"ctype_cntrl":  &core.ExtFunction{Func: ctypeCntrl, Args: []*core.ExtFunctionArg{}},
			"ctype_digit":  &core.ExtFunction{Func: ctypeDigit, Args: []*core.ExtFunctionArg{}},
			"ctype_graph":  &core.ExtFunction{Func: ctypeGraph, Args: []*core.ExtFunctionArg{}},
			"ctype_lower":  &core.ExtFunction{Func: ctypeLower, Args: []*core.ExtFunctionArg{}},
			"ctype_print":  &core.ExtFunction{Func: ctypePrint, Args: []*core.ExtFunctionArg{}},
			"ctype_punct":  &core.ExtFunction{Func: ctypePunct, Args: []*core.ExtFunctionArg{}},
			"ctype_space":  &core.ExtFunction{Func: ctypeSpace, Args: []*core.ExtFunctionArg{}},
			"ctype_upper":  &core.ExtFunction{Func: ctypeUpper, Args: []*core.ExtFunctionArg{}},
			"ctype_xdigit": &core.ExtFunction{Func: ctypeXdigit, Args: []*core.ExtFunctionArg{}},
		},
		Constants: map[core.ZString]*core.ZVal{},
	})
}
