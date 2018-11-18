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
			"phpversion":   &ExtFunction{Func: stdFuncPhpVersion, Args: []*ExtFunctionArg{}},
			"zend_version": &ExtFunction{Func: stdFuncZendVersion, Args: []*ExtFunctionArg{}},
		},
		Constants: map[ZString]*ZVal{
			"DEFAULT_INCLUDE_PATH": ZString(".:").ZVal(),
			"FALSE":                ZBool(false).ZVal(),
			"NULL":                 ZNull{}.ZVal(),
			"PHP_EOL":              ZString("\n").ZVal(),
			"PHP_EXTRA_VERSION":    ZString("").ZVal(),
			"PHP_FD_SETSIZE":       ZInt(1024).ZVal(),
			"PHP_FLOAT_DIG":        ZInt(15).ZVal(),
			"PHP_FLOAT_EPSILON":    ZFloat(2.220446049250313e-16).ZVal(),
			"PHP_FLOAT_MAX":        ZFloat(math.MaxFloat64).ZVal(),
			"PHP_FLOAT_MIN":        ZFloat(2.2250738585072E-308).ZVal(),
			"PHP_INT_MAX":          ZInt(math.MaxInt64).ZVal(),
			"PHP_INT_MIN":          ZInt(math.MinInt64).ZVal(),
			"PHP_INT_SIZE":         ZInt(8).ZVal(),
			"PHP_MAJOR_VERSION":    ZInt(7).ZVal(),
			"PHP_MINOR_VERSION":    ZInt(3).ZVal(),
			"PHP_OS":               ZString(runtime.GOOS).ZVal(),
			"PHP_OS_FAMILY":        ZString(runtime.GOOS).ZVal(),
			"PHP_RELEASE_VERSION":  ZInt(0).ZVal(),
			"PHP_VERSION":          ZString(VERSION).ZVal(),
			"PHP_VERSION_ID":       ZInt(70300).ZVal(),
			"PHP_ZTS":              ZInt(1).ZVal(),
			"TRUE":                 ZBool(true).ZVal(),
			"ZEND_THREAD_SAFE":     ZBool(true).ZVal(),
		},
	})
}
