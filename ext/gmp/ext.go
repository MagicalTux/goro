package gmp

import "github.com/MagicalTux/gophp/core"

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	core.RegisterExt(&core.Ext{
		Name: "gmp",
		Classes: []*core.ZClass{
			GMP,
		},
		Functions: map[string]*core.ExtFunction{
			"gmp_abs":    &core.ExtFunction{Func: gmpAbs, Args: []*core.ExtFunctionArg{}},
			"gmp_add":    &core.ExtFunction{Func: gmpAdd, Args: []*core.ExtFunctionArg{}},
			"gmp_cmp":    &core.ExtFunction{Func: gmpCmp, Args: []*core.ExtFunctionArg{}},
			"gmp_init":   &core.ExtFunction{Func: gmpInit, Args: []*core.ExtFunctionArg{}},
			"gmp_intval": &core.ExtFunction{Func: gmpIntval, Args: []*core.ExtFunctionArg{}},
			"gmp_strval": &core.ExtFunction{Func: gmpStrval, Args: []*core.ExtFunctionArg{}},
		},
		Constants: map[core.ZString]*core.ZVal{},
	})
}
