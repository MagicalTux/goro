package example

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
)

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "example",
		Version: core.VERSION,
		Classes: []phpv.ZClass{},
		// Note: ExtFunctionArg is currently unused
		Functions: map[string]*phpctx.ExtFunction{
			"ey":    {Func: fncHey, Args: []*phpctx.ExtFunctionArg{}}, // alias
			"hello": {Func: fncHello, Args: []*phpctx.ExtFunctionArg{}},
			"hey":   {Func: fncHey, Args: []*phpctx.ExtFunctionArg{}},
			"wah":   {Func: fncWah, Args: []*phpctx.ExtFunctionArg{}},
			"yo":    {Func: fncHey, Args: []*phpctx.ExtFunctionArg{}}, // alias
		},
		Constants: map[phpv.ZString]phpv.Val{},
	})
}
