package pcre

import "github.com/MagicalTux/gophp/core"

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	core.RegisterExt(&core.Ext{
		Name: "pcre",
		Functions: map[string]*core.ExtFunction{
			"preg_replace": &core.ExtFunction{Func: pregReplace, Args: []*core.ExtFunctionArg{}}, // replace.go:6
		},
		Constants: map[core.ZString]*core.ZVal{},
	})
}
