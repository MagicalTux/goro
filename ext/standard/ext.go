package standard

import (
	"math"

	"github.com/MagicalTux/gophp/core"
)

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	core.RegisterExt(&core.Ext{
		Name: "standard",
		Functions: map[string]*core.ExtFunction{
			"get_magic_quotes_gpc":     &core.ExtFunction{Func: getMagicQuotesGpc, Args: []*core.ExtFunctionArg{}},      // deprecated.go:5
			"get_magic_quotes_runtime": &core.ExtFunction{Func: getMagicQuotesRuntime, Args: []*core.ExtFunctionArg{}},  // deprecated.go:10
			"constant":                 &core.ExtFunction{Func: constant, Args: []*core.ExtFunctionArg{}},               // misc.go:12
			"eval":                     &core.ExtFunction{Func: stdFuncEval, Args: []*core.ExtFunctionArg{}},            // misc.go:23
			"hrtime":                   &core.ExtFunction{Func: stdFuncHrTime, Args: []*core.ExtFunctionArg{}},          // misc.go:43
			"sleep":                    &core.ExtFunction{Func: stdFuncSleep, Args: []*core.ExtFunctionArg{}},           // misc.go:65
			"usleep":                   &core.ExtFunction{Func: stdFuncUsleep, Args: []*core.ExtFunctionArg{}},          // misc.go:78
			"exit":                     &core.ExtFunction{Func: exit, Args: []*core.ExtFunctionArg{}},                   // misc.go:91
			"var_dump":                 &core.ExtFunction{Func: stdFuncVarDump, Args: []*core.ExtFunctionArg{}},         // vardump.go:9
			"str_replace":              &core.ExtFunction{Func: stdStrReplace, Args: []*core.ExtFunctionArg{}},          // strings.go:9
			"echo":                     &core.ExtFunction{Func: stdFuncEcho, Args: []*core.ExtFunctionArg{}},            // echo.go:5
			"gc_collect_cycles":        &core.ExtFunction{Func: stdFuncGcCollectCycles, Args: []*core.ExtFunctionArg{}}, // gc.go:5
			"gc_disable":               &core.ExtFunction{Func: stdFuncGcDisable, Args: []*core.ExtFunctionArg{}},       // gc.go:11
			"gc_enable":                &core.ExtFunction{Func: stdFuncGcEnable, Args: []*core.ExtFunctionArg{}},        // gc.go:16
			"gc_enabled":               &core.ExtFunction{Func: stdFuncGcEnabled, Args: []*core.ExtFunctionArg{}},       // gc.go:21
			"gc_mem_caches":            &core.ExtFunction{Func: stdFuncGcMemCaches, Args: []*core.ExtFunctionArg{}},     // gc.go:26
			"abs":                      &core.ExtFunction{Func: mathAbs, Args: []*core.ExtFunctionArg{}},                // math.go:36
			"acos":                     &core.ExtFunction{Func: mathAcos, Args: []*core.ExtFunctionArg{}},               // math.go:64
			"acosh":                    &core.ExtFunction{Func: mathACosh, Args: []*core.ExtFunctionArg{}},              // math.go:75
			"asin":                     &core.ExtFunction{Func: mathAsin, Args: []*core.ExtFunctionArg{}},               // math.go:86
			"asinh":                    &core.ExtFunction{Func: mathAsinh, Args: []*core.ExtFunctionArg{}},              // math.go:97
			"atan2":                    &core.ExtFunction{Func: mathAtan2, Args: []*core.ExtFunctionArg{}},              // math.go:108
			"atan":                     &core.ExtFunction{Func: mathAtan, Args: []*core.ExtFunctionArg{}},               // math.go:119
			"atanh":                    &core.ExtFunction{Func: mathAtanh, Args: []*core.ExtFunctionArg{}},              // math.go:130
			"cos":                      &core.ExtFunction{Func: mathCos, Args: []*core.ExtFunctionArg{}},                // math.go:141
			"cosh":                     &core.ExtFunction{Func: mathCosh, Args: []*core.ExtFunctionArg{}},               // math.go:152
			"deg2rad":                  &core.ExtFunction{Func: mathDeg2rad, Args: []*core.ExtFunctionArg{}},            // math.go:163
			"exp":                      &core.ExtFunction{Func: mathExp, Args: []*core.ExtFunctionArg{}},                // math.go:174
			"expm1":                    &core.ExtFunction{Func: mathExpm1, Args: []*core.ExtFunctionArg{}},              // math.go:185
			"fmod":                     &core.ExtFunction{Func: mathFmod, Args: []*core.ExtFunctionArg{}},               // math.go:196
			"hypot":                    &core.ExtFunction{Func: mathHypot, Args: []*core.ExtFunctionArg{}},              // math.go:207
			"pi":                       &core.ExtFunction{Func: mathPi, Args: []*core.ExtFunctionArg{}},                 // math.go:218
			"dl":                       &core.ExtFunction{Func: stdFuncDl, Args: []*core.ExtFunctionArg{}},              // base.go:11
			"extension_loaded":         &core.ExtFunction{Func: stdFunc, Args: []*core.ExtFunctionArg{}},                // base.go:16
			"phpversion":               &core.ExtFunction{Func: stdFuncPhpVersion, Args: []*core.ExtFunctionArg{}},      // base.go:26
			"zend_version":             &core.ExtFunction{Func: stdFuncZendVersion, Args: []*core.ExtFunctionArg{}},     // base.go:45
			"function_exists":          &core.ExtFunction{Func: stdFuncFuncExists, Args: []*core.ExtFunctionArg{}},      // base.go:50
			"get_cfg_var":              &core.ExtFunction{Func: stdFuncGetCfgVar, Args: []*core.ExtFunctionArg{}},       // base.go:62
		},
		Constants: map[core.ZString]*core.ZVal{
			"INF":                 core.ZFloat(math.Inf(0)).ZVal(),            // math.go:10
			"NAN":                 core.ZFloat(math.NaN()).ZVal(),             // math.go:11
			"M_PI":                core.ZFloat(math.Pi).ZVal(),                // math.go:12
			"M_E":                 core.ZFloat(math.E).ZVal(),                 // math.go:13
			"M_LOG2E":             core.ZFloat(math.Log2E).ZVal(),             // math.go:14
			"M_LOG10E":            core.ZFloat(math.Log10E).ZVal(),            // math.go:15
			"M_LN2":               core.ZFloat(math.Ln2).ZVal(),               // math.go:16
			"M_PI_2":              core.ZFloat(math.Pi / 2).ZVal(),            // math.go:17
			"M_PI_4":              core.ZFloat(math.Pi / 4).ZVal(),            // math.go:18
			"M_1_PI":              core.ZFloat(1 / math.Pi).ZVal(),            // math.go:19
			"M_2_PI":              core.ZFloat(2 / math.Pi).ZVal(),            // math.go:20
			"M_SQRTPI":            core.ZFloat(math.Sqrt(math.Pi)).ZVal(),     // math.go:21
			"M_2_SQRTPI":          core.ZFloat(2 / math.Sqrt(math.Pi)).ZVal(), // math.go:22
			"M_SQRT2":             core.ZFloat(math.Sqrt(2)).ZVal(),           // math.go:23
			"M_SQRT3":             core.ZFloat(math.Sqrt(3)).ZVal(),           // math.go:24
			"M_SQRT1_2":           core.ZFloat(1 / math.Sqrt(2)).ZVal(),       // math.go:25
			"M_LNPI":              core.ZFloat(math.Log(math.Pi)).ZVal(),      // math.go:26
			"M_EULER":             core.ZFloat(0.57721566490153286061).ZVal(), // math.go:27
			"PHP_ROUND_HALF_UP":   core.ZInt(1).ZVal(),                        // math.go:29
			"PHP_ROUND_HALF_DOWN": core.ZInt(2).ZVal(),                        // math.go:30
			"PHP_ROUND_HALF_EVEN": core.ZInt(3).ZVal(),                        // math.go:31
			"PHP_ROUND_HALF_ODD":  core.ZInt(4).ZVal(),                        // math.go:32
			"M_PHI":               core.ZFloat(math.Phi).ZVal(),               // math.go:34
			"PHP_VERSION":         core.ZString(core.VERSION).ZVal(),          // base.go:9
		},
	})
}
