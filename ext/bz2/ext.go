package bz2

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	core.RegisterExt(&core.Ext{
		Name:    "bz2",
		Version: core.VERSION,
		Classes: []*core.ZClass{},
		Functions: map[string]*core.ExtFunction{
			"bzdecompress": &core.ExtFunction{Func: fncBzDecompress, Args: []*core.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]*phpv.ZVal{},
	})
}
