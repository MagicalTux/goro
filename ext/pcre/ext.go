package pcre

import "github.com/MagicalTux/goro/core"

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	core.RegisterExt(&core.Ext{
		Name:    "pcre",
		Version: core.VERSION,
		Classes: []*core.ZClass{},
		Functions: map[string]*core.ExtFunction{
			"preg_quote":   &core.ExtFunction{Func: pregQuote, Args: []*core.ExtFunctionArg{}},
			"preg_replace": &core.ExtFunction{Func: pregReplace, Args: []*core.ExtFunctionArg{}},
		},
		Constants: map[core.ZString]*core.ZVal{},
	})
}
