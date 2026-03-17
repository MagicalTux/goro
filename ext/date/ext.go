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
		Classes: []*phpobj.ZClass{DateTimeInterface, DateTime, DateTimeImmutable, DateInterval, DateTimeZone, DatePeriod},
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
		Constants: map[phpv.ZString]phpv.Val{
			// Date format constants
			"DATE_ATOM":         phpv.ZString("Y-m-d\\TH:i:sP"),
			"DATE_COOKIE":       phpv.ZString("l, d-M-Y H:i:s T"),
			"DATE_ISO8601":      phpv.ZString("Y-m-d\\TH:i:sO"),
			"DATE_ISO8601_EXPANDED": phpv.ZString("X-m-d\\TH:i:sP"),
			"DATE_RFC822":       phpv.ZString("D, d M y H:i:s O"),
			"DATE_RFC850":       phpv.ZString("l, d-M-y H:i:s T"),
			"DATE_RFC1036":      phpv.ZString("D, d M y H:i:s O"),
			"DATE_RFC1123":      phpv.ZString("D, d M Y H:i:s O"),
			"DATE_RFC7231":      phpv.ZString("D, d M Y H:i:s \\G\\M\\T"),
			"DATE_RFC2822":      phpv.ZString("D, d M Y H:i:s O"),
			"DATE_RFC3339":      phpv.ZString("Y-m-d\\TH:i:sP"),
			"DATE_RFC3339_EXTENDED": phpv.ZString("Y-m-d\\TH:i:s.vP"),
			"DATE_RSS":          phpv.ZString("D, d M Y H:i:s O"),
			"DATE_W3C":          phpv.ZString("Y-m-d\\TH:i:sP"),
			// Day of week constants
			"SUNFUNCS_RET_TIMESTAMP": phpv.ZInt(0),
			"SUNFUNCS_RET_STRING":    phpv.ZInt(1),
			"SUNFUNCS_RET_DOUBLE":    phpv.ZInt(2),
		},
	})
}
