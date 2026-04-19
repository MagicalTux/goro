package example

import (
	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpctx"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "example",
		Version: core.VERSION,
		Classes: []*phpobj.ZClass{},
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
