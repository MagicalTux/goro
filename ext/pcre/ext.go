package pcre

import "github.com/MagicalTux/gophp/core"

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	core.RegisterExt(&core.Ext{
		Name: "pcre",
		Functions: map[string]*core.ExtFunction{
			"preg_quote":   &core.ExtFunction{Func: pregQuote, Args: []*core.ExtFunctionArg{}},
			"preg_replace": &core.ExtFunction{Func: pregReplace, Args: []*core.ExtFunctionArg{}},
		},
		Constants: map[core.ZString]*core.ZVal{},
	})
}
