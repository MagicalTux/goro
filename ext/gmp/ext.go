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
			"gmp_abs":    {Func: gmpAbs, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_add":    {Func: gmpAdd, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_clrbit": {Func: gmpClrbit, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_cmp":    {Func: gmpCmp, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_init":   {Func: gmpInit, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_intval": {Func: gmpIntval, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_neg":    {Func: gmpNeg, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_setbit": {Func: gmpSetbit, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_strval": {Func: gmpStrval, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_sub":    {Func: gmpSub, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{},
	})
}
