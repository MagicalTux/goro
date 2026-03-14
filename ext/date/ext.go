package date

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "date",
		Version: core.VERSION,
		Classes: []*phpobj.ZClass{DateTimeInterface, DateTime, DateTimeImmutable, DateInterval},
		Functions: map[string]*phpctx.ExtFunction{
			"checkdate":                 {Func: fncCheckdate, Args: []*phpctx.ExtFunctionArg{}},
			"date":                      {Func: fncDate, Args: []*phpctx.ExtFunctionArg{}},
			"date_default_timezone_get": {Func: fncDateDefaultTimezoneGet, Args: []*phpctx.ExtFunctionArg{}},
			"date_default_timezone_set": {Func: fncDateDefaultTimezoneSet, Args: []*phpctx.ExtFunctionArg{}},
			"date_diff":                 {Func: fncDateDiff, Args: []*phpctx.ExtFunctionArg{}},
			"getdate":                   {Func: fncGetdate, Args: []*phpctx.ExtFunctionArg{}},
			"gmdate":                    {Func: fncGmdate, Args: []*phpctx.ExtFunctionArg{}},
			"gmmktime":                  {Func: fncGmmktime, Args: []*phpctx.ExtFunctionArg{}},
			"idate":                     {Func: fncIdate, Args: []*phpctx.ExtFunctionArg{}},
			"localtime":                 {Func: fncLocaltime, Args: []*phpctx.ExtFunctionArg{}},
			"mktime":                    {Func: fncMktime, Args: []*phpctx.ExtFunctionArg{}},
			"strftime":                  {Func: fncStrftime, Args: []*phpctx.ExtFunctionArg{}},
			"strtotime":                 {Func: fncStrtotime, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{},
	})
}
