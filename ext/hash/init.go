package hash

import (
	"crypto/hmac"
	"errors"
	"fmt"
	gohash "hash"

	"github.com/MagicalTux/goro/core"
)

//> const HASH_HMAC: core.ZInt(1)

//> func HashContext hash_init ( string $algo [, int $options = 0 [, string $key = NULL ]] )
func fncHashInit(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var algo core.ZString
	var opt *core.ZInt
	var key *core.ZString

	_, err := core.Expand(ctx, args, &algo, &opt, &key)
	if err != nil {
		return nil, err
	}

	algN, ok := algos[algo.ToLower()]
	if !ok {
		return nil, fmt.Errorf("Unknown hashing algorithm: %s", algo)
	}

	var h gohash.Hash

	if opt != nil && *opt == 1 {
		// HMAC
		var k []byte
		if key == nil {
			return nil, errors.New("HMAC requested without a key") // TODO make this a warning
		} else {
			k = []byte(*key)
		}

		h = hmac.New(algN, k)
	} else {
		h = algN()
	}

	z, err := core.NewZObjectOpaque(ctx, HashContext, h)
	if err != nil {
		return nil, err
	}

	return z.ZVal(), nil
}
