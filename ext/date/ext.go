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
		Classes: []*phpobj.ZClass{DateTimeInterface, DateTime, DateTimeImmutable, DateInterval, DateTimeZone},
		Functions: map[string]*phpctx.ExtFunction{
			"checkdate":                   {Func: fncCheckdate, Args: []*phpctx.ExtFunctionArg{}},
			"date":                        {Func: fncDate, Args: []*phpctx.ExtFunctionArg{}},
			"date_create":                 {Func: fncDateCreate, Args: []*phpctx.ExtFunctionArg{}},
			"date_create_immutable":       {Func: fncDateCreateImmutable, Args: []*phpctx.ExtFunctionArg{}},
			"date_date_set":               {Func: fncDateDateSet, Args: []*phpctx.ExtFunctionArg{}},
			"date_default_timezone_get":   {Func: fncDateDefaultTimezoneGet, Args: []*phpctx.ExtFunctionArg{}},
			"date_default_timezone_set":   {Func: fncDateDefaultTimezoneSet, Args: []*phpctx.ExtFunctionArg{}},
			"date_diff":                   {Func: fncDateDiff, Args: []*phpctx.ExtFunctionArg{}},
			"date_format":                 {Func: fncDateFormat, Args: []*phpctx.ExtFunctionArg{}},
			"date_modify":                 {Func: fncDateModify, Args: []*phpctx.ExtFunctionArg{}},
			"date_offset_get":             {Func: fncDateOffsetGet, Args: []*phpctx.ExtFunctionArg{}},
			"date_time_set":               {Func: fncDateTimeSet, Args: []*phpctx.ExtFunctionArg{}},
			"date_timestamp_get":          {Func: fncDateTimestampGet, Args: []*phpctx.ExtFunctionArg{}},
			"date_timestamp_set":          {Func: fncDateTimestampSet, Args: []*phpctx.ExtFunctionArg{}},
			"date_timezone_get":           {Func: fncDateTimezoneGet, Args: []*phpctx.ExtFunctionArg{}},
			"date_timezone_set":           {Func: fncDateTimezoneSet, Args: []*phpctx.ExtFunctionArg{}},
			"getdate":                     {Func: fncGetdate, Args: []*phpctx.ExtFunctionArg{}},
			"gmdate":                      {Func: fncGmdate, Args: []*phpctx.ExtFunctionArg{}},
			"gmmktime":                    {Func: fncGmmktime, Args: []*phpctx.ExtFunctionArg{}},
			"gmstrftime":                  {Func: fncGmstrftime, Args: []*phpctx.ExtFunctionArg{}},
			"idate":                       {Func: fncIdate, Args: []*phpctx.ExtFunctionArg{}},
			"localtime":                   {Func: fncLocaltime, Args: []*phpctx.ExtFunctionArg{}},
			"mktime":                      {Func: fncMktime, Args: []*phpctx.ExtFunctionArg{}},
			"strftime":                    {Func: fncStrftime, Args: []*phpctx.ExtFunctionArg{}},
			"strtotime":                   {Func: fncStrtotime, Args: []*phpctx.ExtFunctionArg{}},
			"timezone_abbreviations_list": {Func: fncTimezoneAbbreviationsList, Args: []*phpctx.ExtFunctionArg{}},
			"timezone_identifiers_list":   {Func: fncTimezoneIdentifiersList, Args: []*phpctx.ExtFunctionArg{}},
			"timezone_name_get":           {Func: fncTimezoneNameGet, Args: []*phpctx.ExtFunctionArg{}},
			"timezone_offset_get":         {Func: fncTimezoneOffsetGet, Args: []*phpctx.ExtFunctionArg{}},
			"timezone_open":               {Func: fncTimezoneOpen, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{},
	})
}
