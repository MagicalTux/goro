package ctype

import "github.com/MagicalTux/gophp/core"

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	core.RegisterExt(&core.Ext{
		Name: "ctype",
		Functions: map[string]*core.ExtFunction{
			"ctype_alnum":  &core.ExtFunction{Func: ctypeAlnum, Args: []*core.ExtFunctionArg{}},  // ctype.go:28
			"ctype_alpha":  &core.ExtFunction{Func: ctypeAlpha, Args: []*core.ExtFunctionArg{}},  // ctype.go:38
			"ctype_cntrl":  &core.ExtFunction{Func: ctypeCntrl, Args: []*core.ExtFunctionArg{}},  // ctype.go:48
			"ctype_digit":  &core.ExtFunction{Func: ctypeDigit, Args: []*core.ExtFunctionArg{}},  // ctype.go:58
			"ctype_graph":  &core.ExtFunction{Func: ctypeGraph, Args: []*core.ExtFunctionArg{}},  // ctype.go:68
			"ctype_lower":  &core.ExtFunction{Func: ctypeLower, Args: []*core.ExtFunctionArg{}},  // ctype.go:78
			"ctype_print":  &core.ExtFunction{Func: ctypePrint, Args: []*core.ExtFunctionArg{}},  // ctype.go:88
			"ctype_punct":  &core.ExtFunction{Func: ctypePunct, Args: []*core.ExtFunctionArg{}},  // ctype.go:98
			"ctype_space":  &core.ExtFunction{Func: ctypeSpace, Args: []*core.ExtFunctionArg{}},  // ctype.go:108
			"ctype_upper":  &core.ExtFunction{Func: ctypeUpper, Args: []*core.ExtFunctionArg{}},  // ctype.go:118
			"ctype_xdigit": &core.ExtFunction{Func: ctypeXdigit, Args: []*core.ExtFunctionArg{}}, // ctype.go:128
		},
		Constants: map[core.ZString]*core.ZVal{},
	})
}
