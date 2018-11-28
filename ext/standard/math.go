package standard

import (
	"math"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// from: http://php.net/manual/en/math.constants.php
//> const INF:                 phpv.ZFloat(math.Inf(0)) // positive infinite
//> const NAN:                 phpv.ZFloat(math.NaN())
//> const M_PI:                phpv.ZFloat(math.Pi)     // Pi
//> const M_E:                 phpv.ZFloat(math.E)      // e
//> const M_LOG2E:             phpv.ZFloat(math.Log2E)  // log_2 e
//> const M_LOG10E:            phpv.ZFloat(math.Log10E) // log_10 e
//> const M_LN2:               phpv.ZFloat(math.Ln2)    // log_e 2
//> const M_PI_2:              phpv.ZFloat(math.Pi / 2)
//> const M_PI_4:              phpv.ZFloat(math.Pi / 4)
//> const M_1_PI:              phpv.ZFloat(1 / math.Pi)
//> const M_2_PI:              phpv.ZFloat(2 / math.Pi)
//> const M_SQRTPI:            phpv.ZFloat(math.Sqrt(math.Pi)) // PHP 5.2.0
//> const M_2_SQRTPI:          phpv.ZFloat(2 / math.Sqrt(math.Pi))
//> const M_SQRT2:             phpv.ZFloat(math.Sqrt(2))
//> const M_SQRT3:             phpv.ZFloat(math.Sqrt(3)) // PHP 5.2.0
//> const M_SQRT1_2:           phpv.ZFloat(1 / math.Sqrt(2))
//> const M_LNPI:              phpv.ZFloat(math.Log(math.Pi))
//> const M_EULER:             phpv.ZFloat(0.57721566490153286061) // Euler constant

//> const PHP_ROUND_HALF_UP:   phpv.ZInt(1)                        // Round halves up
//> const PHP_ROUND_HALF_DOWN: phpv.ZInt(2)                        // Round halves down
//> const PHP_ROUND_HALF_EVEN: phpv.ZInt(3)                        // Round halves to even numbers
//> const PHP_ROUND_HALF_ODD:  phpv.ZInt(4)                        // Round halves to odd numbers

//> const M_PHI: phpv.ZFloat(math.Phi) // specific to this implementation of PHP

//> func number abs ( mixed $number )
func mathAbs(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var z *phpv.ZVal
	_, err := core.Expand(ctx, args, &z)
	if err != nil {
		return nil, err
	}

	z, err = z.AsNumeric(ctx)
	if err != nil {
		return nil, err
	}

	switch z.GetType() {
	case phpv.ZtInt:
		i := z.AsInt(ctx)
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

//> func float acos ( float $arg )
func mathAcos(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var f phpv.ZFloat
	_, err := core.Expand(ctx, args, &f)
	if err != nil {
		return nil, err
	}

	return phpv.ZFloat(math.Acos(float64(f))).ZVal(), nil
}

//> func float acosh ( float $arg )
func mathACosh(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var f phpv.ZFloat
	_, err := core.Expand(ctx, args, &f)
	if err != nil {
		return nil, err
	}

	return phpv.ZFloat(math.Acosh(float64(f))).ZVal(), nil
}

//> func float asin ( float $arg )
func mathAsin(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var f phpv.ZFloat
	_, err := core.Expand(ctx, args, &f)
	if err != nil {
		return nil, err
	}

	return phpv.ZFloat(math.Asin(float64(f))).ZVal(), nil
}

//> func float asinh ( float $arg )
func mathAsinh(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var f phpv.ZFloat
	_, err := core.Expand(ctx, args, &f)
	if err != nil {
		return nil, err
	}

	return phpv.ZFloat(math.Asinh(float64(f))).ZVal(), nil
}

//> func float atan2 ( float $y , float $x )
func mathAtan2(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var y, x phpv.ZFloat
	_, err := core.Expand(ctx, args, &y, &x)
	if err != nil {
		return nil, err
	}

	return phpv.ZFloat(math.Atan2(float64(y), float64(x))).ZVal(), nil
}

//> func float atan ( float $arg )
func mathAtan(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var x phpv.ZFloat
	_, err := core.Expand(ctx, args, &x)
	if err != nil {
		return nil, err
	}

	return phpv.ZFloat(math.Atan(float64(x))).ZVal(), nil
}

//> func float atanh ( float $arg )
func mathAtanh(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var x phpv.ZFloat
	_, err := core.Expand(ctx, args, &x)
	if err != nil {
		return nil, err
	}

	return phpv.ZFloat(math.Atanh(float64(x))).ZVal(), nil
}

//> func float cos ( float $arg )
func mathCos(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var x phpv.ZFloat
	_, err := core.Expand(ctx, args, &x)
	if err != nil {
		return nil, err
	}

	return phpv.ZFloat(math.Cos(float64(x))).ZVal(), nil
}

//> func float cosh ( float $arg )
func mathCosh(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var x phpv.ZFloat
	_, err := core.Expand(ctx, args, &x)
	if err != nil {
		return nil, err
	}

	return phpv.ZFloat(math.Cosh(float64(x))).ZVal(), nil
}

//> func float deg2rad ( float $number )
func mathDeg2rad(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var x phpv.ZFloat
	_, err := core.Expand(ctx, args, &x)
	if err != nil {
		return nil, err
	}

	return (x / 180 * math.Pi).ZVal(), nil
}

//> func float exp ( float $arg )
func mathExp(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var x phpv.ZFloat
	_, err := core.Expand(ctx, args, &x)
	if err != nil {
		return nil, err
	}

	return phpv.ZFloat(math.Exp(float64(x))).ZVal(), nil
}

//> func float expm1 ( float $arg )
func mathExpm1(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var x phpv.ZFloat
	_, err := core.Expand(ctx, args, &x)
	if err != nil {
		return nil, err
	}

	return phpv.ZFloat(math.Expm1(float64(x))).ZVal(), nil
}

//> func float fmod ( float $x , float $y )
func mathFmod(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var x, y phpv.ZFloat
	_, err := core.Expand(ctx, args, &x, &y)
	if err != nil {
		return nil, err
	}

	return phpv.ZFloat(math.Mod(float64(x), float64(y))).ZVal(), nil
}

//> func float hypot ( float $x , float $y )
func mathHypot(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var x, y phpv.ZFloat
	_, err := core.Expand(ctx, args, &x, &y)
	if err != nil {
		return nil, err
	}

	return phpv.ZFloat(math.Hypot(float64(x), float64(y))).ZVal(), nil
}

//> func float pi ( void )
func mathPi(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZFloat(math.Pi).ZVal(), nil
}
