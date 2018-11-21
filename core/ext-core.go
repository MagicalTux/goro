package core

import (
	"math"
	"runtime"
)

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	RegisterExt(&Ext{
		Name: "Core",
		Classes: []*ZClass{
			stdClass,
		},
		Functions: map[string]*ExtFunction{
			"count":           &ExtFunction{Func: fncCount, Args: []*ExtFunctionArg{}},
			"define":          &ExtFunction{Func: fncDefine, Args: []*ExtFunctionArg{}},
			"defined":         &ExtFunction{Func: fncDefined, Args: []*ExtFunctionArg{}},
			"echo":            &ExtFunction{Func: stdFuncEcho, Args: []*ExtFunctionArg{}},
			"error_reporting": &ExtFunction{Func: fncErrorReporting, Args: []*ExtFunctionArg{}},
			"func_get_arg":    &ExtFunction{Func: fncFuncGetArg, Args: []*ExtFunctionArg{}},
			"func_get_args":   &ExtFunction{Func: fncFuncGetArgs, Args: []*ExtFunctionArg{}},
			"func_num_args":   &ExtFunction{Func: fncFuncNumArgs, Args: []*ExtFunctionArg{}},
			"include":         &ExtFunction{Func: fncInclude, Args: []*ExtFunctionArg{}},
			"include_once":    &ExtFunction{Func: fncIncludeOnce, Args: []*ExtFunctionArg{}},
			"phpversion":      &ExtFunction{Func: stdFuncPhpVersion, Args: []*ExtFunctionArg{}},
			"print":           &ExtFunction{Func: fncPrint, Args: []*ExtFunctionArg{}},
			"require":         &ExtFunction{Func: fncRequire, Args: []*ExtFunctionArg{}},
			"require_once":    &ExtFunction{Func: fncRequireOnce, Args: []*ExtFunctionArg{}},
			"strlen":          &ExtFunction{Func: fncStrlen, Args: []*ExtFunctionArg{}},
			"zend_version":    &ExtFunction{Func: stdFuncZendVersion, Args: []*ExtFunctionArg{}},
		},
		Constants: map[ZString]*ZVal{
			"DEFAULT_INCLUDE_PATH":         ZString(".:").ZVal(),
			"FALSE":                        ZBool(false).ZVal(),
			"NULL":                         ZNull{}.ZVal(),
			"PHP_EOL":                      ZString("\n").ZVal(),
			"PHP_EXTRA_VERSION":            ZString("").ZVal(),
			"PHP_FD_SETSIZE":               ZInt(1024).ZVal(),
			"PHP_FLOAT_DIG":                ZInt(15).ZVal(),
			"PHP_FLOAT_EPSILON":            ZFloat(2.220446049250313e-16).ZVal(),
			"PHP_FLOAT_MAX":                ZFloat(math.MaxFloat64).ZVal(),
			"PHP_FLOAT_MIN":                ZFloat(math.SmallestNonzeroFloat64).ZVal(),
			"PHP_INT_MAX":                  ZInt(math.MaxInt64).ZVal(),
			"PHP_INT_MIN":                  ZInt(math.MinInt64).ZVal(),
			"PHP_INT_SIZE":                 ZInt(8).ZVal(),
			"PHP_MAJOR_VERSION":            ZInt(7).ZVal(),
			"PHP_MINOR_VERSION":            ZInt(3).ZVal(),
			"PHP_OS":                       ZString(runtime.GOOS).ZVal(),
			"PHP_OS_FAMILY":                ZString(runtime.GOOS).ZVal(),
			"PHP_OUTPUT_HANDLER_CLEAN":     ZInt(BufferClean).ZVal(),
			"PHP_OUTPUT_HANDLER_CLEANABLE": ZInt(BufferCleanable).ZVal(),
			"PHP_OUTPUT_HANDLER_CONT":      ZInt(BufferWrite).ZVal(),
			"PHP_OUTPUT_HANDLER_END":       ZInt(BufferFinal).ZVal(),
			"PHP_OUTPUT_HANDLER_FINAL":     ZInt(BufferFinal).ZVal(),
			"PHP_OUTPUT_HANDLER_FLUSH":     ZInt(BufferFlush).ZVal(),
			"PHP_OUTPUT_HANDLER_FLUSHABLE": ZInt(BufferFlushable).ZVal(),
			"PHP_OUTPUT_HANDLER_REMOVABLE": ZInt(BufferRemovable).ZVal(),
			"PHP_OUTPUT_HANDLER_START":     ZInt(BufferStart).ZVal(),
			"PHP_OUTPUT_HANDLER_STDFLAGS":  ZInt(BufferCleanable | BufferFlushable | BufferRemovable).ZVal(),
			"PHP_OUTPUT_HANDLER_WRITE":     ZInt(BufferWrite).ZVal(),
			"PHP_RELEASE_VERSION":          ZInt(0).ZVal(),
			"PHP_VERSION":                  ZString(VERSION).ZVal(),
			"PHP_VERSION_ID":               ZInt(70300).ZVal(),
			"PHP_ZTS":                      ZInt(1).ZVal(),
			"TRUE":                         ZBool(true).ZVal(),
			"ZEND_THREAD_SAFE":             ZBool(true).ZVal(),
		},
	})
}
