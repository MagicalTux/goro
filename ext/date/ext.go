package date

import "github.com/MagicalTux/goro/core"

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	core.RegisterExt(&core.Ext{
		Name:    "date",
		Version: core.VERSION,
		Classes: []*core.ZClass{},
		Functions: map[string]*core.ExtFunction{
			"strftime": &core.ExtFunction{Func: fncStrftime, Args: []*core.ExtFunctionArg{}},
		},
		Constants: map[core.ZString]*core.ZVal{},
	})
}
