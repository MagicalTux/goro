package standard

import (
	"math"
	"time"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func int mt_getrandmax ( void )
// > alias getrandmax
func mathMtGetRandMax(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZInt(math.MaxInt32).ZVal(), nil
}

// > func int mt_rand ( int $min , int $max )
// > alias rand
func mathMtRand(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var min, max core.Optional[phpv.ZInt]
	_, err := core.Expand(ctx, args, &min, &max)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	r := ctx.Global().Random()
	if min.HasArg() || max.HasArg() {
		a := int64(min.GetOrDefault(phpv.ZInt(0)))
		b := int64(max.GetOrDefault(phpv.ZInt(0)))
		n := a + r.Mt.Int64N(b-a)
		return phpv.ZInt(n).ZVal(), nil
	}

	return phpv.ZInt(r.Mt.Int32()).ZVal(), nil
}

// > func void mt_srand ([ int $seed ])
// > alias srand
func mathMtSRand(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var seedArg *phpv.ZInt
	_, err := core.Expand(ctx, args, &seedArg)
	if err != nil {
		return nil, ctx.FuncError(err)
	}

	var seed int64
	if seedArg != nil {
		seed = int64(*seedArg)
	} else {
		seed = time.Now().UnixMicro()
	}

	r := ctx.Global().Random()
	r.MtSeed(seed)

	return nil, nil
}
