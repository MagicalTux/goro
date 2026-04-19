package bz2

import (
	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpctx"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "bz2",
		Version: core.VERSION,
		Classes: []*phpobj.ZClass{},
		// Note: ExtFunctionArg is currently unused
		Functions: map[string]*phpctx.ExtFunction{
			"bzdecompress": {Func: fncBzDecompress, Args: []*phpctx.ExtFunctionArg{}},
			"bzcompress":   {Func: fncBzCompress, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{},
	})
}
