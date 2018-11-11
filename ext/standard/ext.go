package standard

import (
	"math"

	"git.atonline.com/tristantech/gophp/core"
)

func init() {
	core.RegisterExt(&core.Ext{
		Name: "standard",
		Functions: map[string]*core.ExtFunction{
			"echo":     &core.ExtFunction{Func: stdFuncEcho, Args: []*core.ExtFunctionArg{&core.ExtFunctionArg{ArgName: "output"}, &core.ExtFunctionArg{ArgName: "...", Optional: true}}},
			"var_dump": &core.ExtFunction{Func: stdFuncVarDump, Args: []*core.ExtFunctionArg{&core.ExtFunctionArg{ArgName: "vars"}, &core.ExtFunctionArg{ArgName: "...", Optional: true}}},
			"eval":     &core.ExtFunction{Func: stdFuncEval, Args: []*core.ExtFunctionArg{&core.ExtFunctionArg{ArgName: "code"}}},
		},
		Constants: map[core.ZString]*core.ZVal{
			// from: http://php.net/manual/en/math.constants.php
			"INF":                 core.ZFloat(math.Inf(0)).ZVal(),
			"NAN":                 core.ZFloat(math.NaN()).ZVal(),
			"M_PI":                core.ZFloat(math.Pi).ZVal(),     // Pi
			"M_E":                 core.ZFloat(math.E).ZVal(),      // e
			"M_LOG2E":             core.ZFloat(math.Log2E).ZVal(),  // log_2 e
			"M_LOG10E":            core.ZFloat(math.Log10E).ZVal(), // log_10 e
			"M_LN2":               core.ZFloat(math.Ln2).ZVal(),    // log_e 2
			"M_PI_2":              core.ZFloat(math.Pi / 2).ZVal(),
			"M_PI_4":              core.ZFloat(math.Pi / 4).ZVal(),
			"M_1_PI":              core.ZFloat(1 / math.Pi).ZVal(),
			"M_2_PI":              core.ZFloat(2 / math.Pi).ZVal(),
			"M_SQRTPI":            core.ZFloat(math.Sqrt(math.Pi)).ZVal(), // PHP 5.2.0
			"M_2_SQRTPI":          core.ZFloat(2 / math.Sqrt(math.Pi)).ZVal(),
			"M_SQRT2":             core.ZFloat(math.Sqrt(2)).ZVal(),
			"M_SQRT3":             core.ZFloat(math.Sqrt(3)).ZVal(), // PHP 5.2.0
			"M_SQRT1_2":           core.ZFloat(1 / math.Sqrt(2)).ZVal(),
			"M_LNPI":              core.ZFloat(math.Log(math.Pi)).ZVal(),
			"M_EULER":             core.ZFloat(0.57721566490153286061).ZVal(), // Euler constant
			"PHP_ROUND_HALF_UP":   core.ZInt(1).ZVal(),                        // Round halves up
			"PHP_ROUND_HALF_DOWN": core.ZInt(2).ZVal(),                        // Round halves down
			"PHP_ROUND_HALF_EVEN": core.ZInt(3).ZVal(),                        // Round halves to even numbers
			"PHP_ROUND_HALF_ODD":  core.ZInt(4).ZVal(),                        // Round halves to odd numbers

			"M_PHI": core.ZFloat(math.Phi).ZVal(), // specific to this implementation of PHP
		},
	})
}
