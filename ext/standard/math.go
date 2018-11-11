package standard

import (
	"math"

	"github.com/MagicalTux/gophp/core"
)

// from: http://php.net/manual/en/math.constants.php
//> const INF:                 core.ZFloat(math.Inf(0)) // positive infinite
//> const NAN:                 core.ZFloat(math.NaN())
//> const M_PI:                core.ZFloat(math.Pi)     // Pi
//> const M_E:                 core.ZFloat(math.E)      // e
//> const M_LOG2E:             core.ZFloat(math.Log2E)  // log_2 e
//> const M_LOG10E:            core.ZFloat(math.Log10E) // log_10 e
//> const M_LN2:               core.ZFloat(math.Ln2)    // log_e 2
//> const M_PI_2:              core.ZFloat(math.Pi / 2)
//> const M_PI_4:              core.ZFloat(math.Pi / 4)
//> const M_1_PI:              core.ZFloat(1 / math.Pi)
//> const M_2_PI:              core.ZFloat(2 / math.Pi)
//> const M_SQRTPI:            core.ZFloat(math.Sqrt(math.Pi)) // PHP 5.2.0
//> const M_2_SQRTPI:          core.ZFloat(2 / math.Sqrt(math.Pi))
//> const M_SQRT2:             core.ZFloat(math.Sqrt(2))
//> const M_SQRT3:             core.ZFloat(math.Sqrt(3)) // PHP 5.2.0
//> const M_SQRT1_2:           core.ZFloat(1 / math.Sqrt(2))
//> const M_LNPI:              core.ZFloat(math.Log(math.Pi))
//> const M_EULER:             core.ZFloat(0.57721566490153286061) // Euler constant

//> const PHP_ROUND_HALF_UP:   core.ZInt(1)                        // Round halves up
//> const PHP_ROUND_HALF_DOWN: core.ZInt(2)                        // Round halves down
//> const PHP_ROUND_HALF_EVEN: core.ZInt(3)                        // Round halves to even numbers
//> const PHP_ROUND_HALF_ODD:  core.ZInt(4)                        // Round halves to odd numbers

//> const M_PHI: core.ZFloat(math.Phi) // specific to this implementation of PHP

//> func number abs ( mixed $number )
func mathAbs(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var z *core.ZVal
	_, err := core.Expand(ctx, args)
	if err != nil {
		return nil, err
	}

	z, err = z.AsNumeric(ctx)
	if err != nil {
		return nil, err
	}

	switch z.GetType() {
	case core.ZtInt:
		i := z.AsInt(ctx)
		if i < 0 {
			return (-i).ZVal(), nil
		} else {
			return i.ZVal(), nil
		}
	case core.ZtFloat:
		return core.ZFloat(math.Abs(float64(z.AsFloat(ctx)))).ZVal(), nil
	default:
		return core.ZNull{}.ZVal(), nil
	}
}
