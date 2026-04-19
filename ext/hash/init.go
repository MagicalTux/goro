package hash

import (
	"crypto/hmac"
	"fmt"
	gohash "hash"

	"github.com/KarpelesLab/anyhash"
	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
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

	if _, ok := algos[algoLower]; !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_init(): Argument #1 ($algo) must be a valid hashing algorithm")
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

			seedVal, hasSeed, _ := arr.OffsetCheck(ctx, phpv.ZString("seed"))
			secretVal, hasSecret, _ := arr.OffsetCheck(ctx, phpv.ZString("secret"))

			if hasSeed && seedVal != nil {
				isSeeded32 := isSeededAlgo(algoLower)
				isSeeded64 := isSeeded64Algo(algoLower)

				if isSeeded32 || isSeeded64 {
					if seedVal.GetType() != phpv.ZtInt {
						// Deprecation warning for non-int seed
						if isSeeded64 {
							if err := ctx.Deprecated("Passing a seed of a type other than int is deprecated because it is ignored"); err != nil {
								return nil, err
							}
						} else {
							if err := ctx.Deprecated("Passing a seed of a type other than int is deprecated because it is the same as setting the seed to 0"); err != nil {
								return nil, err
							}
						}
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
				if isSeeded64Algo(algoLower) {
					if hasSeed && seedVal != nil {
						return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
							fmt.Sprintf("%s: Only one of seed or secret is to be passed for initialization", string(algo)))
					}
					if secretVal.GetType() != phpv.ZtString {
						if err := ctx.Deprecated("Passing a secret of a type other than string is deprecated because it implicitly converts to a string, potentially hiding bugs"); err != nil {
							return nil, err
						}
					}
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
		algN := algos[algoLower]
		h = hmac.New(algN, hmacKey)
		isHmac = true
	} else {
		// Use anyhash.New with Options for seeded/secret algorithms
		if isSeededAlgo(algoLower) || isSeeded64Algo(algoLower) {
			var opts anyhash.Options
			if isSeeded64Algo(algoLower) {
				opts.Seed = seed64
				opts.Secret = secret
			} else {
				opts.Seed = uint64(seed)
			}
			h, err = anyhash.New(anyhashName(algoLower), opts)
			if err != nil {
				return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_init(): Argument #1 ($algo) must be a valid hashing algorithm")
			}
		} else {
			h = algos[algoLower]()
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
