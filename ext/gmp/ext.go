package gmp

import (
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpctx"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// WARNING: This file is auto-generated. DO NOT EDIT

func init() {
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "gmp",
		Version: core.VERSION,
		Classes: []*phpobj.ZClass{
			GMP,
		},
		// Note: ExtFunctionArg is currently unused
		Functions: map[string]*phpctx.ExtFunction{
			"gmp_abs":            {Func: gmpAbs, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_add":            {Func: gmpAdd, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_and":            {Func: gmpAnd, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_binomial":       {Func: gmpBinomial, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_clrbit":         {Func: gmpClrbit, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_cmp":            {Func: gmpCmp, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_com":            {Func: gmpCom, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_div":            {Func: gmpDiv, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_div_q":          {Func: gmpDivQ, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_div_qr":         {Func: gmpDivQR, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_div_r":          {Func: gmpDivR, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_divexact":       {Func: gmpDivexact, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_export":         {Func: gmpExport, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_fact":           {Func: gmpFact, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_gcd":            {Func: gmpGcd, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_gcdext":         {Func: gmpGcdext, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_hamdist":        {Func: gmpHamdist, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_import":         {Func: gmpImport, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_init":           {Func: gmpInit, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_intval":         {Func: gmpIntval, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_invert":         {Func: gmpInvert, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_jacobi":         {Func: gmpJacobi, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_kronecker":      {Func: gmpKronecker, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_lcm":            {Func: gmpLcm, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_legendre":       {Func: gmpLegendre, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_mod":            {Func: gmpMod, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_mul":            {Func: gmpMul, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_neg":            {Func: gmpNeg, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_nextprime":      {Func: gmpNextprime, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_or":             {Func: gmpOr, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_perfect_power":  {Func: gmpPerfectPower, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_perfect_square": {Func: gmpPerfectSquare, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_popcount":       {Func: gmpPopcount, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_pow":            {Func: gmpPow, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_powm":           {Func: gmpPowm, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_prob_prime":     {Func: gmpProbPrime, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_random_bits":    {Func: gmpRandomBits, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_random_range":   {Func: gmpRandomRange, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_random_seed":    {Func: gmpRandomSeed, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_root":           {Func: gmpRoot, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_rootrem":        {Func: gmpRootrem, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_scan0":          {Func: gmpScan0, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_scan1":          {Func: gmpScan1, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_setbit":         {Func: gmpSetbit, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_sign":           {Func: gmpSign, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_sqrt":           {Func: gmpSqrt, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_sqrtrem":        {Func: gmpSqrtrem, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_strval":         {Func: gmpStrval, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_sub":            {Func: gmpSub, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_testbit":        {Func: gmpTestbit, Args: []*phpctx.ExtFunctionArg{}},
			"gmp_xor":            {Func: gmpXor, Args: []*phpctx.ExtFunctionArg{}},
		},
		Constants: map[phpv.ZString]phpv.Val{
			"GMP_ROUND_ZERO":    phpv.ZInt(0),
			"GMP_ROUND_PLUSINF": phpv.ZInt(1),
			"GMP_ROUND_MINUSINF": phpv.ZInt(2),
			"GMP_MSW_FIRST":     phpv.ZInt(0x08),
			"GMP_LSW_FIRST":     phpv.ZInt(0x10),
			"GMP_BIG_ENDIAN":    phpv.ZInt(0x02),
			"GMP_LITTLE_ENDIAN": phpv.ZInt(0x04),
			"GMP_NATIVE_ENDIAN": phpv.ZInt(0x00),
			"GMP_VERSION":       phpv.ZString("6.3.0"),
		},
	})
}
