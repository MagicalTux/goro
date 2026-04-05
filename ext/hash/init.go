package hash

import (
	"crypto/hmac"
	"fmt"
	gohash "hash"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// > const
const HASH_HMAC = phpv.ZInt(1)

// > func HashContext hash_init ( string $algo [, int $options = 0 [, string $key = NULL [, array $options = [] ]]] )
func fncHashInit(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var algo phpv.ZString
	var opt *phpv.ZInt
	var key *phpv.ZString
	var optionsArg core.Optional[*phpv.ZVal]

	// Check if the key arg was explicitly passed as null (PHP 8.1 deprecation)
	keyArgIsNull := len(args) > 2 && args[2] != nil && args[2].GetType() == phpv.ZtNull

	_, err := core.Expand(ctx, args, &algo, &opt, &key, &optionsArg)
	if err != nil {
		return nil, err
	}

	// Emit null-to-string deprecation if key was explicitly null
	if keyArgIsNull {
		if err := ctx.Deprecated("Passing null to parameter #3 ($key) of type string is deprecated"); err != nil {
			return nil, err
		}
	}

	algoLower := algo.ToLower()

	algN, ok := algos[algoLower]
	if !ok {
		// Check if it's a seeded algo
		if _, ok2 := seededAlgos[algoLower]; !ok2 {
			if _, ok3 := seededAlgos64[algoLower]; !ok3 {
				return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_init(): Argument #1 ($algo) must be a valid hashing algorithm")
			}
		}
	}

	var h gohash.Hash
	var isHmac bool
	var hmacKey []byte
	var seed uint32
	var seed64 uint64
	var secret []byte

	// Parse options array (PHP 8 fourth param)
	if optionsArg.HasArg() {
		options := optionsArg.Get()
		if options != nil && options.GetType() == phpv.ZtArray {
			arr := options.AsArray(ctx)

			// Handle "seed" option
			seedVal, hasSeed, _ := arr.OffsetCheck(ctx, phpv.ZString("seed"))
			secretVal, hasSecret, _ := arr.OffsetCheck(ctx, phpv.ZString("secret"))

			if hasSeed && seedVal != nil {
				// Check if it's a seeded algo
				isSeeded32 := seededAlgos[algoLower] != nil
				isSeeded64 := seededAlgos64[algoLower] != nil

				if isSeeded32 || isSeeded64 {
					if seedVal.GetType() != phpv.ZtInt {
						// Deprecation warning for non-int seed
						if isSeeded64 {
							// xxh3/xxh128: "ignored"
							if err := ctx.Deprecated("Passing a seed of a type other than int is deprecated because it is ignored"); err != nil {
								return nil, err
							}
						} else {
							// murmur3 / xxh32 / xxh64: "same as setting the seed to 0"
							if err := ctx.Deprecated("Passing a seed of a type other than int is deprecated because it is the same as setting the seed to 0"); err != nil {
								return nil, err
							}
						}
						// Use 0 as seed
					} else {
						seedInt := int64(seedVal.Value().(phpv.ZInt))
						if isSeeded64 {
							seed64 = uint64(seedInt)
						} else {
							seed = uint32(int32(seedInt))
						}
					}
				}
			}

			if hasSecret && secretVal != nil {
				if seededAlgos64[algoLower] != nil {
					// xxh3/xxh128 support "secret"
					if hasSeed && seedVal != nil {
						return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
							fmt.Sprintf("%s: Only one of seed or secret is to be passed for initialization", string(algo)))
					}
					if secretVal.GetType() != phpv.ZtString {
						// Deprecation warning
						if err := ctx.Deprecated("Passing a secret of a type other than string is deprecated because it implicitly converts to a string, potentially hiding bugs"); err != nil {
							return nil, err
						}
					}
					// Convert to string (this may panic/error on non-stringable)
					secretStr, err := secretVal.AsVal(ctx, phpv.ZtString)
					if err != nil {
						return nil, err
					}
					secretBytes := []byte(secretStr.Value().(phpv.ZString))
					if len(secretBytes) < 136 {
						return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
							fmt.Sprintf("%s: Secret length must be >= 136 bytes, %d bytes passed", string(algo), len(secretBytes)))
					}
					secret = secretBytes
				}
			}
		}
	}

	if opt != nil && *opt == 1 {
		// HMAC
		if nonCryptoAlgos[algoLower] {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_init(): Argument #1 ($algo) must be a cryptographic hashing algorithm if HMAC is requested")
		}
		if key == nil || len(*key) == 0 {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_init(): Argument #3 ($key) must not be empty when HMAC is requested")
		}
		hmacKey = []byte(*key)
		algN, ok = algos[algoLower]
		if !ok {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_init(): Argument #1 ($algo) must be a valid hashing algorithm")
		}
		h = hmac.New(algN, hmacKey)
		isHmac = true
	} else {
		// Use seeded constructor if seed was provided
		if seededAlgo, ok := seededAlgos[algoLower]; ok {
			h = seededAlgo(seed)
		} else if seededAlgo64, ok := seededAlgos64[algoLower]; ok {
			h = seededAlgo64(seed64, secret)
		} else {
			algN, ok = algos[algoLower]
			if !ok {
				return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_init(): Argument #1 ($algo) must be a valid hashing algorithm")
			}
			h = algN()
		}
	}

	hcd := &hashContextData{
		Hash:    h,
		algo:    algoLower,
		isHmac:  isHmac,
		hmacKey: hmacKey,
		seed:    seed,
		seed64:  seed64,
		secret:  secret,
	}

	z, err := phpobj.NewZObjectOpaque(ctx, HashContext, hcd)
	if err != nil {
		return nil, err
	}

	return z.ZVal(), nil
}
