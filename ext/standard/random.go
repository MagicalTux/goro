package standard

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > func int random_int ( int $min , int $max )
func fncRandomInt(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var min, max phpv.ZInt
	_, err := core.Expand(ctx, args, &min, &max)
	if err != nil {
		return nil, err
	}

	if min > max {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			fmt.Sprintf("random_int(): Argument #1 ($min) must be less than or equal to argument #2 ($max)"))
	}

	if min == max {
		return min.ZVal(), nil
	}

	// Range: max - min + 1
	rangeSize := new(big.Int).SetInt64(int64(max) - int64(min) + 1)
	n, err := rand.Int(rand.Reader, rangeSize)
	if err != nil {
		return nil, err
	}

	result := phpv.ZInt(n.Int64() + int64(min))
	return result.ZVal(), nil
}

// > func string random_bytes ( int $length )
func fncRandomBytes(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var length phpv.ZInt
	_, err := core.Expand(ctx, args, &length)
	if err != nil {
		return nil, err
	}

	if length < 1 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
			fmt.Sprintf("random_bytes(): Argument #1 ($length) must be greater than 0"))
	}

	buf := make([]byte, int(length))
	_, err = rand.Read(buf)
	if err != nil {
		return nil, err
	}

	return phpv.ZString(buf).ZVal(), nil
}
