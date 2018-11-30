package gmp

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
)

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "gmp",
		Version: core.VERSION,
		Classes: []phpv.ZClass{
			GMP,
		},
		Functions: map[string]*phpctx.ExtFunction{
			"gmp_abs":    &phpctx.ExtFunction{Func: gmpAbs, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_add":    &phpctx.ExtFunction{Func: gmpAdd, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_clrbit": &phpctx.ExtFunction{Func: gmpClrbit, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_cmp":    &phpctx.ExtFunction{Func: gmpCmp, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_init":   &phpctx.ExtFunction{Func: gmpInit, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_intval": &phpctx.ExtFunction{Func: gmpIntval, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_neg":    &phpctx.ExtFunction{Func: gmpNeg, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_setbit": &phpctx.ExtFunction{Func: gmpSetbit, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_strval": &phpctx.ExtFunction{Func: gmpStrval, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_sub":    &phpctx.ExtFunction{Func: gmpSub, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{},
	})
}
