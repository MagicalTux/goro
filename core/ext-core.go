package core

import (
	"math"
	"runtime"

	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpv"
)

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "Core",
		Version: VERSION,
		Classes: []phpv.ZClass{},
		Functions: map[string]*phpctx.ExtFunction{
			"count":             &phpctx.ExtFunction{Func: fncCount, Args: []*phpctx.ExtFunctionArg{}},
			"define":            &phpctx.ExtFunction{Func: fncDefine, Args: []*phpctx.ExtFunctionArg{}},
			"defined":           &phpctx.ExtFunction{Func: fncDefined, Args: []*phpctx.ExtFunctionArg{}},
			"echo":              &phpctx.ExtFunction{Func: stdFuncEcho, Args: []*phpctx.ExtFunctionArg{}},
			"error_reporting":   &phpctx.ExtFunction{Func: fncErrorReporting, Args: []*phpctx.ExtFunctionArg{}},
			"func_get_arg":      &phpctx.ExtFunction{Func: fncFuncGetArg, Args: []*phpctx.ExtFunctionArg{}},
			"func_get_args":     &phpctx.ExtFunction{Func: fncFuncGetArgs, Args: []*phpctx.ExtFunctionArg{}},
			"func_num_args":     &phpctx.ExtFunction{Func: fncFuncNumArgs, Args: []*phpctx.ExtFunctionArg{}},
			"gc_collect_cycles": &phpctx.ExtFunction{Func: stdFuncGcCollectCycles, Args: []*phpctx.ExtFunctionArg{}},
			"gc_disable":        &phpctx.ExtFunction{Func: stdFuncGcDisable, Args: []*phpctx.ExtFunctionArg{}},
			"gc_enable":         &phpctx.ExtFunction{Func: stdFuncGcEnable, Args: []*phpctx.ExtFunctionArg{}},
			"gc_enabled":        &phpctx.ExtFunction{Func: stdFuncGcEnabled, Args: []*phpctx.ExtFunctionArg{}},
			"gc_mem_caches":     &phpctx.ExtFunction{Func: stdFuncGcMemCaches, Args: []*phpctx.ExtFunctionArg{}},
			"include":           &phpctx.ExtFunction{Func: fncInclude, Args: []*phpctx.ExtFunctionArg{}},
			"include_once":      &phpctx.ExtFunction{Func: fncIncludeOnce, Args: []*phpctx.ExtFunctionArg{}},
			"phpversion":        &phpctx.ExtFunction{Func: stdFuncPhpVersion, Args: []*phpctx.ExtFunctionArg{}},
			"print":             &phpctx.ExtFunction{Func: fncPrint, Args: []*phpctx.ExtFunctionArg{}},
			"require":           &phpctx.ExtFunction{Func: fncRequire, Args: []*phpctx.ExtFunctionArg{}},
			"require_once":      &phpctx.ExtFunction{Func: fncRequireOnce, Args: []*phpctx.ExtFunctionArg{}},
			"strcmp":            &phpctx.ExtFunction{Func: fncStrcmp, Args: []*phpctx.ExtFunctionArg{}},
			"strlen":            &phpctx.ExtFunction{Func: fncStrlen, Args: []*phpctx.ExtFunctionArg{}},
			"zend_version":      &phpctx.ExtFunction{Func: stdFuncZendVersion, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{
			"DEFAULT_INCLUDE_PATH":     phpv.ZString(".:"),
			"DIRECTORY_SEPARATOR":      phpv.ZString("/"),
			"E_ALL":                    phpv.ZInt(phpv.E_ALL),
			"E_COMPILphpv.E_ERROR":     phpv.ZInt(phpv.E_COMPILE_ERROR),
			"E_COMPILphpv.E_WARNING":   phpv.ZInt(phpv.E_COMPILE_WARNING),
			"E_CORphpv.E_ERROR":        phpv.ZInt(phpv.E_CORE_ERROR),
			"E_CORphpv.E_WARNING":      phpv.ZInt(phpv.E_CORE_WARNING),
			"E_DEPRECATED":             phpv.ZInt(phpv.E_DEPRECATED),
			"E_ERROR":                  phpv.ZInt(phpv.E_ERROR),
			"E_NOTICE":                 phpv.ZInt(phpv.E_NOTICE),
			"E_PARSE":                  phpv.ZInt(phpv.E_PARSE),
			"E_RECOVERABLphpv.E_ERROR": phpv.ZInt(phpv.E_RECOVERABLE_ERROR),
			"E_STRICT":                 phpv.ZInt(phpv.E_STRICT),
			"E_USER_DEPRECATED":        phpv.ZInt(phpv.E_USER_DEPRECATED),
			"E_USER_ERROR":             phpv.ZInt(phpv.E_USER_ERROR),
			"E_USER_NOTICE":            phpv.ZInt(phpv.E_USER_NOTICE),
			"E_USER_WARNING":           phpv.ZInt(phpv.E_USER_WARNING),
			"E_WARNING":                phpv.ZInt(phpv.E_WARNING),
			"FALSE":                    phpv.ZBool(false),
			"NULL":                     phpv.ZNull{},
			"PHP_EOL":                  phpv.ZString("\n"),
			"PHP_EXTRA_VERSION":        phpv.ZString(""),
			"PHP_FD_SETSIZE":           phpv.ZInt(1024),
			"PHP_FLOAT_DIG":            phpv.ZInt(15),
			"PHP_FLOAT_EPSILON":        phpv.ZFloat(2.220446049250313e-16),
			"PHP_FLOAT_MAX":            phpv.ZFloat(math.MaxFloat64),
			"PHP_FLOAT_MIN":            phpv.ZFloat(math.SmallestNonzeroFloat64),
			"PHP_INT_MAX":              phpv.ZInt(math.MaxInt64),
			"PHP_INT_MIN":              phpv.ZInt(math.MinInt64),
			"PHP_INT_SIZE":             phpv.ZInt(8),
			"PHP_MAJOR_VERSION":        phpv.ZInt(7),
			"PHP_MAXPATHLEN":           phpv.ZInt(4096),
			"PHP_MINOR_VERSION":        phpv.ZInt(3),
			"PHP_OS":                   phpv.ZString(runtime.GOOS),
			"PHP_OS_FAMILY":            phpv.ZString(runtime.GOOS),
			"PHP_RELEASE_VERSION":      phpv.ZInt(0),
			"PHP_VERSION":              phpv.ZString(VERSION),
			"PHP_VERSION_ID":           phpv.ZInt(70300),
			"PHP_ZTS":                  phpv.ZInt(1),
			"TRUE":                     phpv.ZBool(true),
			"ZEND_THREAD_SAFE":         phpv.ZBool(true),
		},
	})
}
