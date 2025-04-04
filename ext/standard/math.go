package standard

import (
	"math"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpv"
)

// from: http://php.net/manual/en/math.constants.php
// > const
var (
	INF        = phpv.ZFloat(math.Inf(0)) // positive infinite
	NAN        = phpv.ZFloat(math.NaN())
	M_PI       = phpv.ZFloat(math.Pi)     // Pi
	M_E        = phpv.ZFloat(math.E)      // e
	M_LOG2E    = phpv.ZFloat(math.Log2E)  // log_2 e
	M_LOG10E   = phpv.ZFloat(math.Log10E) // log_10 e
	M_LN2      = phpv.ZFloat(math.Ln2)    // log_e 2
	M_LN10     = phpv.ZFloat(2.302585092994)
	M_PI_2     = phpv.ZFloat(math.Pi / 2)
	M_PI_4     = phpv.ZFloat(math.Pi / 4)
	M_1_PI     = phpv.ZFloat(1 / math.Pi)
	M_2_PI     = phpv.ZFloat(2 / math.Pi)
	M_SQRTPI   = phpv.ZFloat(math.Sqrt(math.Pi)) // PHP 5.2.0
	M_2_SQRTPI = phpv.ZFloat(2 / math.Sqrt(math.Pi))
	M_SQRT2    = phpv.ZFloat(math.Sqrt(2))
	M_SQRT3    = phpv.ZFloat(math.Sqrt(3)) // PHP 5.2.0
	M_SQRT1_2  = phpv.ZFloat(1 / math.Sqrt(2))
	M_LNPI     = phpv.ZFloat(math.Log(math.Pi))
	M_EULER    = phpv.ZFloat(0.57721566490153286061) // Euler constant

	PHP_ROUND_HALF_UP   = phpv.ZInt(1) // Round halves up
	PHP_ROUND_HALF_DOWN = phpv.ZInt(2) // Round halves down
	PHP_ROUND_HALF_EVEN = phpv.ZInt(3) // Round halves to even numbers
	PHP_ROUND_HALF_ODD  = phpv.ZInt(4) // Round halves to odd numbers

	M_PHI = phpv.ZFloat(math.Phi) // specific to this implementation of PHP
)

// > func number abs ( mixed $number )
func mathAbs(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var z *phpv.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	z, err = z.AsNumeric(ctx)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	switch z.GetType() {
	case phpv.ZtInt:
		i := z.AsInt(ctx)
		if i <= math.MinInt64 {
			return phpv.ZFloat(math.Abs(float64(z.AsFloat(ctx)))).ZVal(), nil
		}
		if i < 0 {
			return (-i).ZVal(), nil
		} else {
			return i.ZVal(), nil
		}
	case phpv.ZtFloat:
		return phpv.ZFloat(math.Abs(float64(z.AsFloat(ctx)))).ZVal(), nil
	default:
		return phpv.ZNull{}.ZVal(), nil
	}
}

// > func number ceil ( mixed $number )
func mathCeil(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var z *phpv.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	z, err = z.AsNumeric(ctx)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	i := float64(z.AsFloat(ctx))
	return phpv.ZFloat(math.Ceil(i)).ZVal(), nil
}

// > func number floor ( mixed $number )
func mathFloor(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var z *phpv.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	z, err = z.AsNumeric(ctx)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	i := float64(z.AsFloat(ctx))
	return phpv.ZFloat(math.Floor(i)).ZVal(), nil
}

// > func number round ( float $val [, int $precision = 0 [, int $mode = PHP_ROUND_HALF_UP ]] )
func mathRound(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var val *phpv.ZVal
	var precisionArg *phpv.ZInt
	var modeArg *phpv.ZInt
	_, err := core.Expand(ctx, args, &val, &precisionArg, &modeArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	precision := core.Deref(precisionArg, 0)
	mode := core.Deref(modeArg, PHP_ROUND_HALF_UP)

	val, err = val.AsNumeric(ctx)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	shift := float64(1)
	n := float64(val.AsFloat(ctx))

	if precision != 0 {
		shift = math.Pow10(int(precision))
		n *= shift
	}

	t := math.Trunc(n)
	d := math.Abs(n - t)
	odd := math.Remainder(t, 2) != 0

	roundUp := false

	switch {
	case d < 0.5:
		roundUp = false
	case d > 0.5:
		roundUp = true
	default:
		switch mode {
		case PHP_ROUND_HALF_UP:
			roundUp = true
		case PHP_ROUND_HALF_DOWN:
			roundUp = false

		case PHP_ROUND_HALF_ODD:
			if !odd {
				roundUp = true
			}
		case PHP_ROUND_HALF_EVEN:
			if odd {
				roundUp = true
			}
		}
	}

	if roundUp {
		n = t + math.Copysign(1, n)
	} else {
		n = t
	}

	n /= shift

	return phpv.ZFloat(n).ZVal(), nil
}

// > func float acos ( float $arg )
func mathAcos(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var f phpv.ZFloat
	_, err := core.Expand(ctx, args, &f)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZFloat(math.Acos(float64(f))).ZVal(), nil
}

// > func float acosh ( float $arg )
func mathACosh(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var f phpv.ZFloat
	_, err := core.Expand(ctx, args, &f)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZFloat(math.Acosh(float64(f))).ZVal(), nil
}

// > func float asin ( float $arg )
func mathAsin(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var f phpv.ZFloat
	_, err := core.Expand(ctx, args, &f)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZFloat(math.Asin(float64(f))).ZVal(), nil
}

// > func float asinh ( float $arg )
func mathAsinh(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var f phpv.ZFloat
	_, err := core.Expand(ctx, args, &f)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZFloat(math.Asinh(float64(f))).ZVal(), nil
}

// > func float atan2 ( float $y , float $x )
func mathAtan2(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var y, x phpv.ZFloat
	_, err := core.Expand(ctx, args, &y, &x)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZFloat(math.Atan2(float64(y), float64(x))).ZVal(), nil
}

// > func float atan ( float $arg )
func mathAtan(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var x phpv.ZFloat
	_, err := core.Expand(ctx, args, &x)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZFloat(math.Atan(float64(x))).ZVal(), nil
}

// > func float atanh ( float $arg )
func mathAtanh(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var x phpv.ZFloat
	_, err := core.Expand(ctx, args, &x)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZFloat(math.Atanh(float64(x))).ZVal(), nil
}

// > func float sin ( float $arg )
func mathSin(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var x phpv.ZFloat
	_, err := core.Expand(ctx, args, &x)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZFloat(math.Sin(float64(x))).ZVal(), nil
}

// > func float sinh ( float $arg )
func mathSinh(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var x phpv.ZFloat
	_, err := core.Expand(ctx, args, &x)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZFloat(math.Sinh(float64(x))).ZVal(), nil
}

// > func float cos ( float $arg )
func mathCos(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var x phpv.ZFloat
	_, err := core.Expand(ctx, args, &x)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZFloat(math.Cos(float64(x))).ZVal(), nil
}

// > func float cosh ( float $arg )
func mathCosh(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var x phpv.ZFloat
	_, err := core.Expand(ctx, args, &x)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZFloat(math.Cosh(float64(x))).ZVal(), nil
}

// > func float tan ( float $arg )
func mathTan(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var x phpv.ZFloat
	_, err := core.Expand(ctx, args, &x)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZFloat(math.Tan(float64(x))).ZVal(), nil
}

// > func float tanh ( float $arg )
func mathTanh(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var x phpv.ZFloat
	_, err := core.Expand(ctx, args, &x)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZFloat(math.Tanh(float64(x))).ZVal(), nil
}

// > func float deg2rad ( float $number )
func mathDeg2rad(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var x phpv.ZFloat
	_, err := core.Expand(ctx, args, &x)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return (x / 180 * math.Pi).ZVal(), nil
}

// > func float rad2deg ( float $number )
func mathRad2Deg(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var x phpv.ZFloat
	_, err := core.Expand(ctx, args, &x)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return (x / math.Pi * 180).ZVal(), nil
}

// > func float exp ( float $arg )
func mathExp(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var x phpv.ZFloat
	_, err := core.Expand(ctx, args, &x)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZFloat(math.Exp(float64(x))).ZVal(), nil
}

// > func float expm1 ( float $arg )
func mathExpm1(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var x phpv.ZFloat
	_, err := core.Expand(ctx, args, &x)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZFloat(math.Expm1(float64(x))).ZVal(), nil
}

// > func float fmod ( float $x , float $y )
func mathFmod(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var x, y phpv.ZFloat
	_, err := core.Expand(ctx, args, &x, &y)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZFloat(math.Mod(float64(x), float64(y))).ZVal(), nil
}

// > func float hypot ( float $x , float $y )
func mathHypot(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var xArg, yArg *phpv.ZVal
	_, err := core.Expand(ctx, args, &xArg, &yArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}
	if xArg.GetType() == phpv.ZtString {
		s := xArg.AsString(ctx)
		if s.ContainsInvalidNumeric() {
			if err = ctx.Notice("A non well formed numeric value encountered", logopt.NoFuncName(true)); err != nil {
				return nil, err
			}
		}
	}
	if yArg.GetType() == phpv.ZtString {
		s := yArg.AsString(ctx)
		if s.ContainsInvalidNumeric() {
			if err = ctx.Notice("A non well formed numeric value encountered", logopt.NoFuncName(true)); err != nil {
				return nil, err
			}
		}
	}

	x, err := xArg.AsNumeric(ctx)
	if err != nil {
		return nil, ctx.FuncError(err)
	}
	y, err := yArg.AsNumeric(ctx)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	return phpv.ZFloat(math.Hypot(float64(x.AsFloat(ctx)), float64(y.AsFloat(ctx)))).ZVal(), nil
}

// > func float pi ( void )
func mathPi(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZFloat(math.Pi).ZVal(), nil
}

// > func mixed max ( array $values )
// > func mixed max ( mixed $value1 [, mixed $... ] )
func mathMax(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var firstArg *phpv.ZVal
	_, err := core.Expand(ctx, args, &firstArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	max := phpv.ZNULL.ZVal()
	if len(args) == 1 && firstArg.GetType() == phpv.ZtArray {
		array := firstArg.AsArray(ctx)
		if array.Count(ctx) == 0 {
			return max, nil
		}
		for _, v := range array.Iterate(ctx) {
			max = v
			break
		}
		for _, v := range array.Iterate(ctx) {
			cmp, err := phpv.Compare(ctx, max, v)
			if err != nil {
				return nil, ctx.Error(err)
			}
			if cmp < 0 {
				max = v
			}
		}
	} else {
		max = firstArg
		for _, v := range args {
			cmp, err := phpv.Compare(ctx, max, v)
			if err != nil {
				return nil, ctx.Error(err)
			}
			if cmp < 0 {
				max = v
			}
		}
	}

	return max, nil
}

// > func mixed min ( array $values )
// > func mixed min ( mixed $value1 [, mixed $... ] )
func mathMin(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var firstArg *phpv.ZVal
	_, err := core.Expand(ctx, args, &firstArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	min := phpv.ZNULL.ZVal()
	if len(args) == 1 && firstArg.GetType() == phpv.ZtArray {
		array := firstArg.AsArray(ctx)
		if array.Count(ctx) == 0 {
			return min, nil
		}
		for _, v := range array.Iterate(ctx) {
			min = v
			break
		}
		for _, v := range array.Iterate(ctx) {
			cmp, err := phpv.Compare(ctx, min, v)
			if err != nil {
				return nil, ctx.Error(err)
			}
			if cmp > 0 {
				min = v
			}
		}
	} else {
		min = firstArg
		for _, v := range args {
			cmp, err := phpv.Compare(ctx, min, v)
			if err != nil {
				return nil, ctx.Error(err)
			}
			if cmp > 0 {
				min = v
			}
		}
	}

	return min, nil
}

// > func bool is_finite ( float $val )
func mathIsFinite(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var val *phpv.ZVal
	_, err := core.Expand(ctx, args, &val)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	val, err = val.AsNumeric(ctx)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	n := float64(val.AsFloat(ctx))
	result := !math.IsInf(n, 0) && !math.IsNaN(n)
	return phpv.ZBool(result).ZVal(), nil
}

// > func bool is_infinite ( float $val )
func mathIsInfinite(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var val *phpv.ZVal
	_, err := core.Expand(ctx, args, &val)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	val, err = val.AsNumeric(ctx)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	result := math.IsInf(float64(val.AsFloat(ctx)), 0)
	return phpv.ZBool(result).ZVal(), nil
}

// > func bool is_nan ( float $val )
func mathIsNaN(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var val *phpv.ZVal
	_, err := core.Expand(ctx, args, &val)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	val, err = val.AsNumeric(ctx)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	result := math.IsNaN(float64(val.AsFloat(ctx)))
	return phpv.ZBool(result).ZVal(), nil
}

// > func number pow ( number $base , number $exp )
func mathPow(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var baseArg, expArg *phpv.ZVal
	_, err := core.Expand(ctx, args, &baseArg, &expArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	baseArg, err = baseArg.AsNumeric(ctx)
	if err != nil {
		return nil, ctx.FuncError(err)
	}
	expArg, err = expArg.AsNumeric(ctx)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	baseType := baseArg.GetType()
	extType := expArg.GetType()

	base := float64(baseArg.AsFloat(ctx))
	exp := float64(expArg.AsFloat(ctx))
	result := math.Pow(base, exp)

	switch {
	case baseType == phpv.ZtFloat,
		extType == phpv.ZtFloat,
		exp < 0,
		exp > 13,
		result > math.MaxInt64,
		result < math.MinInt64:

		return phpv.ZFloat(result).ZVal(), nil
	}

	return phpv.ZInt(result).ZVal(), nil
}

// > func number log ( float $arg [, float $base = M_E ] )
func mathLog(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var val phpv.ZFloat
	var baseArg *phpv.ZFloat
	_, err := core.Expand(ctx, args, &val, &baseArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	if baseArg == nil {
		return phpv.ZFloat(math.Log(float64(val))).ZVal(), nil
	}

	base := core.Deref(baseArg, M_E)

	result := math.Log(float64(val)) / math.Log(float64(base))
	return phpv.ZFloat(result).ZVal(), nil
}

// > func number log10 ( float $arg )
func mathLog10(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var val phpv.ZFloat
	_, err := core.Expand(ctx, args, &val)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	result := math.Log10(float64(val))
	return phpv.ZFloat(result).ZVal(), nil
}

// > func number log1p ( float $arg )
func mathLog1P(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var val phpv.ZFloat
	_, err := core.Expand(ctx, args, &val)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	result := math.Log1p(float64(val))
	return phpv.ZFloat(result).ZVal(), nil
}

// > func number sqrt ( float $arg )
func mathSqrt(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var val phpv.ZFloat
	_, err := core.Expand(ctx, args, &val)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	result := math.Sqrt(float64(val))
	return phpv.ZFloat(result).ZVal(), nil
}

// > func number intdiv ( int $dividend , int $divisor )
func mathIntDiv(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var dividend, divisor phpv.ZInt
	_, err := core.Expand(ctx, args, &dividend, &divisor)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	result := dividend / divisor
	return phpv.ZInt(result).ZVal(), nil
}

