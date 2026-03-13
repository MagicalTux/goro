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
			"date_default_timezone_get": {Func: fncDateDefaultTimezoneGet, Args: []*phpctx.ExtFunctionArg{}},
			"date_default_timezone_set": {Func: fncDateDefaultTimezoneSet, Args: []*phpctx.ExtFunctionArg{}},
			"date_diff":                 {Func: fncDateDiff, Args: []*phpctx.ExtFunctionArg{}},
			"strftime":                  {Func: fncStrftime, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{},
	})
}
