package hash

import (
	"crypto/hmac"
	gohash "hash"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > const
const HASH_HMAC = phpv.ZInt(1)

// > func HashContext hash_init ( string $algo [, int $options = 0 [, string $key = NULL ]] )
func fncHashInit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var algo phpv.ZString
	var opt *phpv.ZInt
	var key *phpv.ZString

	_, err := core.Expand(ctx, args, &algo, &opt, &key)
	if err != nil {
		return nil, err
	}

	algN, ok := algos[algo.ToLower()]
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_init(): Argument #1 ($algo) must be a valid hashing algorithm")
	}

	var h gohash.Hash
	var isHmac bool
	var hmacKey []byte

	if opt != nil && *opt == 1 {
		// HMAC
		if nonCryptoAlgos[algo.ToLower()] {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_init(): Argument #1 ($algo) must be a cryptographic hashing algorithm if HMAC is requested")
		}
		if key == nil || len(*key) == 0 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_init(): Argument #3 ($key) must not be empty when HMAC is requested")
		}
		hmacKey = []byte(*key)
		h = hmac.New(algN, hmacKey)
		isHmac = true
	} else {
		h = algN()
	}

	hcd := &hashContextData{
		Hash:    h,
		algo:    algo.ToLower(),
		isHmac:  isHmac,
		hmacKey: hmacKey,
	}

	z, err := phpobj.NewZObjectOpaque(ctx, HashContext, hcd)
	if err != nil {
		return nil, err
	}

	return z.ZVal(), nil
}
