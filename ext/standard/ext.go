package standard

import (
	"math"

	"git.atonline.com/tristantech/gophp/core"
)

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	core.RegisterExt(&core.Ext{
		Name: "standard",
		Functions: map[string]*core.ExtFunction{
			"get_magic_quotes_gpc":     &core.ExtFunction{Func: getMagicQuotesGpc, Args: []*core.ExtFunctionArg{}},      // deprecated.go:5
			"get_magic_quotes_runtime": &core.ExtFunction{Func: getMagicQuotesRuntime, Args: []*core.ExtFunctionArg{}},  // deprecated.go:10
			"var_dump":                 &core.ExtFunction{Func: stdFuncVarDump, Args: []*core.ExtFunctionArg{}},         // vardump.go:9
			"eval":                     &core.ExtFunction{Func: stdFuncEval, Args: []*core.ExtFunctionArg{}},            // eval.go:11
			"echo":                     &core.ExtFunction{Func: stdFuncEcho, Args: []*core.ExtFunctionArg{}},            // echo.go:5
			"gc_collect_cycles":        &core.ExtFunction{Func: stdFuncGcCollectCycles, Args: []*core.ExtFunctionArg{}}, // gc.go:5
			"gc_disable":               &core.ExtFunction{Func: stdFuncGcDisable, Args: []*core.ExtFunctionArg{}},       // gc.go:11
			"gc_enable":                &core.ExtFunction{Func: stdFuncGcEnable, Args: []*core.ExtFunctionArg{}},        // gc.go:16
			"gc_enabled":               &core.ExtFunction{Func: stdFuncGcEnabled, Args: []*core.ExtFunctionArg{}},       // gc.go:21
			"gc_mem_caches":            &core.ExtFunction{Func: stdFuncGcMemCaches, Args: []*core.ExtFunctionArg{}},     // gc.go:26
			"dl":                       &core.ExtFunction{Func: stdFuncDl, Args: []*core.ExtFunctionArg{}},              // base.go:11
			"extension_loaded":         &core.ExtFunction{Func: stdFunc, Args: []*core.ExtFunctionArg{}},                // base.go:16
			"phpversion":               &core.ExtFunction{Func: stdFuncPhpVersion, Args: []*core.ExtFunctionArg{}},      // base.go:26
			"get_cfg_var":              &core.ExtFunction{Func: stdFuncGetCfgVar, Args: []*core.ExtFunctionArg{}},       // base.go:45
		},
		Constants: map[core.ZString]*core.ZVal{
			"INF":                 core.ZFloat(math.Inf(0)).ZVal(),            // math.go:4
			"NAN":                 core.ZFloat(math.NaN()).ZVal(),             // math.go:5
			"M_PI":                core.ZFloat(math.Pi).ZVal(),                // math.go:6
			"M_E":                 core.ZFloat(math.E).ZVal(),                 // math.go:7
			"M_LOG2E":             core.ZFloat(math.Log2E).ZVal(),             // math.go:8
			"M_LOG10E":            core.ZFloat(math.Log10E).ZVal(),            // math.go:9
			"M_LN2":               core.ZFloat(math.Ln2).ZVal(),               // math.go:10
			"M_PI_2":              core.ZFloat(math.Pi / 2).ZVal(),            // math.go:11
			"M_PI_4":              core.ZFloat(math.Pi / 4).ZVal(),            // math.go:12
			"M_1_PI":              core.ZFloat(1 / math.Pi).ZVal(),            // math.go:13
			"M_2_PI":              core.ZFloat(2 / math.Pi).ZVal(),            // math.go:14
			"M_SQRTPI":            core.ZFloat(math.Sqrt(math.Pi)).ZVal(),     // math.go:15
			"M_2_SQRTPI":          core.ZFloat(2 / math.Sqrt(math.Pi)).ZVal(), // math.go:16
			"M_SQRT2":             core.ZFloat(math.Sqrt(2)).ZVal(),           // math.go:17
			"M_SQRT3":             core.ZFloat(math.Sqrt(3)).ZVal(),           // math.go:18
			"M_SQRT1_2":           core.ZFloat(1 / math.Sqrt(2)).ZVal(),       // math.go:19
			"M_LNPI":              core.ZFloat(math.Log(math.Pi)).ZVal(),      // math.go:20
			"M_EULER":             core.ZFloat(0.57721566490153286061).ZVal(), // math.go:21
			"PHP_ROUND_HALF_UP":   core.ZInt(1).ZVal(),                        // math.go:23
			"PHP_ROUND_HALF_DOWN": core.ZInt(2).ZVal(),                        // math.go:24
			"PHP_ROUND_HALF_EVEN": core.ZInt(3).ZVal(),                        // math.go:25
			"PHP_ROUND_HALF_ODD":  core.ZInt(4).ZVal(),                        // math.go:26
			"M_PHI":               core.ZFloat(math.Phi).ZVal(),               // math.go:28
			"PHP_VERSION":         core.ZString(core.VERSION).ZVal(),          // base.go:9
		},
	})
}
