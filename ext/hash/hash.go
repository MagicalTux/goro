package hash

import (
	"crypto/hmac"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	gohash "hash"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/KarpelesLab/anyhash"
	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/logopt"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// openFileChecked resolves a filename relative to the working directory,
// checks open_basedir restrictions, and opens it for reading.
func openFileChecked(ctx phpv.Context, filename string, funcName string) (io.ReadCloser, error) {
	if !filepath.IsAbs(filename) {
		filename = filepath.Join(string(ctx.Global().Getwd()), filename)
	}
	if err := ctx.Global().CheckOpenBasedir(ctx, filename, funcName); err != nil {
		return nil, err
	}
	return ctx.Global().OpenFile(ctx, filename)
}

// > func string hash ( string $algo , string $data [, bool $raw_output = FALSE [, array $options = [] ]] )
func fncHash(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var algo phpv.ZString
	var data phpv.ZString
	var raw *phpv.ZBool
	var optionsArg core.Optional[*phpv.ZVal]

	_, err := core.Expand(ctx, args, &algo, &data, &raw, &optionsArg)
	if err != nil {
		return nil, err
	}

	algoLower := algo.ToLower()

	var a gohash.Hash
	if optionsArg.HasArg() {
		options := optionsArg.Get()
		if options != nil && options.GetType() == phpv.ZtArray {
			arr := options.AsArray(ctx)
			seedVal, hasSeed, _ := arr.OffsetCheck(ctx, phpv.ZString("seed"))
			secretVal, hasSecret, _ := arr.OffsetCheck(ctx, phpv.ZString("secret"))

			if hasSecret && secretVal != nil && isSeeded64Algo(algoLower) {
				if hasSeed && seedVal != nil {
					return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
						fmt.Sprintf("%s: Only one of seed or secret is to be passed for initialization", string(algo)))
				}
				if secretVal.GetType() != phpv.ZtString {
					if err := ctx.Deprecated("hash_init(): Passing a secret of a type other than string is deprecated because it implicitly converts to a string, potentially hiding bugs"); err != nil {
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
				a, _ = anyhash.New(anyhashName(algoLower), anyhash.Options{Secret: secretBytes})
			} else if hasSeed && seedVal != nil && (isSeededAlgo(algoLower) || isSeeded64Algo(algoLower)) {
				var seed64 uint64
				if seedVal.GetType() == phpv.ZtInt {
					if isSeeded64Algo(algoLower) {
						seed64 = uint64(int64(seedVal.Value().(phpv.ZInt)))
					} else {
						seed64 = uint64(uint32(int32(seedVal.Value().(phpv.ZInt))))
					}
				}
				a, _ = anyhash.New(anyhashName(algoLower), anyhash.Options{Seed: seed64})
			}
		}
	}

	if a == nil {
		algN, ok := algos[algoLower]
		if !ok {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash(): Argument #1 ($algo) must be a valid hashing algorithm")
		}
		a = algN()
	}

	_, err = a.Write([]byte(data))
	if err != nil {
		return nil, err
	}

	r := a.Sum(nil)

	if raw != nil && *raw {
		// return as raw
		return phpv.ZString(r).ZVal(), nil
	}

	// convert to hex
	return phpv.ZString(hex.EncodeToString(r)).ZVal(), nil
}

// > func string hash_file ( string $algo, string $filename [, bool $raw_output = FALSE [, array $options = [] ]] )
func fncHashFile(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var algo phpv.ZString
	var filename phpv.ZString
	var raw *phpv.ZBool
	var optionsArg core.Optional[*phpv.ZVal]

	_, err := core.Expand(ctx, args, &algo, &filename, &raw, &optionsArg)
	if err != nil {
		return nil, err
	}

	algoLower := algo.ToLower()

	// Determine the hash constructor, handling seeded algos
	var a gohash.Hash
	if optionsArg.HasArg() {
		options := optionsArg.Get()
		if options != nil && options.GetType() == phpv.ZtArray {
			arr := options.AsArray(ctx)
			seedVal, hasSeed, _ := arr.OffsetCheck(ctx, phpv.ZString("seed"))
			secretVal, hasSecret, _ := arr.OffsetCheck(ctx, phpv.ZString("secret"))

			if hasSecret && secretVal != nil && isSeeded64Algo(algoLower) {
				secretStr, serr := secretVal.AsVal(ctx, phpv.ZtString)
				if serr != nil {
					return nil, serr
				}
				secretBytes := []byte(secretStr.Value().(phpv.ZString))
				if len(secretBytes) < 136 {
					return nil, phpobj.ThrowError(ctx, phpobj.ValueError,
						fmt.Sprintf("%s: Secret length must be >= 136 bytes, %d bytes passed", string(algo), len(secretBytes)))
				}
				a, _ = anyhash.New(anyhashName(algoLower), anyhash.Options{Secret: secretBytes})
			} else if hasSeed && seedVal != nil && (isSeededAlgo(algoLower) || isSeeded64Algo(algoLower)) {
				var seed64 uint64
				if seedVal.GetType() == phpv.ZtInt {
					if isSeeded64Algo(algoLower) {
						seed64 = uint64(int64(seedVal.Value().(phpv.ZInt)))
					} else {
						seed64 = uint64(uint32(int32(seedVal.Value().(phpv.ZInt))))
					}
				}
				a, _ = anyhash.New(anyhashName(algoLower), anyhash.Options{Seed: seed64})
			}
		}
	}

	if a == nil {
		algN, ok := algos[algoLower]
		if !ok {
			return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_file(): Argument #1 ($algo) must be a valid hashing algorithm")
		}
		a = algN()
	}

	f, err := openFileChecked(ctx, string(filename), "hash_file")
	if err != nil {
		fname := string(filename)
		if errors.Is(err, os.ErrNotExist) {
			return phpv.ZBool(false).ZVal(), ctx.Warn("hash_file(%s): Failed to open stream: No such file or directory", fname, logopt.NoFuncName(true))
		}
		return phpv.ZBool(false).ZVal(), ctx.Warn("hash_file(%s): Failed to open stream: %s", fname, err, logopt.NoFuncName(true))
	}
	defer f.Close()

	if _, err := io.Copy(a, f); err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	r := a.Sum(nil)
	if raw != nil && *raw {
		return phpv.ZString(r).ZVal(), nil
	}
	return phpv.ZString(hex.EncodeToString(r)).ZVal(), nil
}

// > func string hash_hmac_file ( string $algo, string $filename, string $key [, bool $raw_output = FALSE ] )
func fncHashHmacFile(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var algo phpv.ZString
	var filename phpv.ZString
	var key phpv.ZString
	var raw *phpv.ZBool

	_, err := core.Expand(ctx, args, &algo, &filename, &key, &raw)
	if err != nil {
		return nil, err
	}

	algN, ok := algos[algo.ToLower()]
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_hmac_file(): Argument #1 ($algo) must be a valid cryptographic hashing algorithm")
	}
	if nonCryptoAlgos[algo.ToLower()] {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_hmac_file(): Argument #1 ($algo) must be a valid cryptographic hashing algorithm")
	}

	// Check for null bytes in filename
	if strings.ContainsRune(string(filename), 0) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_hmac_file(): Argument #2 ($filename) must not contain any null bytes")
	}

	f, err := openFileChecked(ctx, string(filename), "hash_hmac_file")
	if err != nil {
		fname := string(filename)
		if errors.Is(err, os.ErrNotExist) {
			return phpv.ZBool(false).ZVal(), ctx.Warn("hash_hmac_file(%s): Failed to open stream: No such file or directory", fname, logopt.NoFuncName(true))
		}
		return phpv.ZBool(false).ZVal(), ctx.Warn("hash_hmac_file(%s): Failed to open stream: %s", fname, err, logopt.NoFuncName(true))
	}
	defer f.Close()

	a := hmac.New(algN, []byte(key))
	if _, err := io.Copy(a, f); err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}

	r := a.Sum(nil)
	if raw != nil && *raw {
		return phpv.ZString(r).ZVal(), nil
	}
	return phpv.ZString(hex.EncodeToString(r)).ZVal(), nil
}

// > func bool hash_equals ( string $known_string , string $user_string )
func fncHashEquals(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	if len(args) < 2 {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "hash_equals() expects exactly 2 arguments, "+fmt.Sprintf("%d", len(args))+" given")
	}

	// Strict type checking: must be string, no coercion from int/null/etc.
	if args[0].GetType() != phpv.ZtString {
		typeName := phpv.ZValTypeName(args[0])
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("hash_equals(): Argument #1 ($known_string) must be of type string, %s given", typeName))
	}
	if args[1].GetType() != phpv.ZtString {
		typeName := phpv.ZValTypeName(args[1])
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, fmt.Sprintf("hash_equals(): Argument #2 ($user_string) must be of type string, %s given", typeName))
	}

	known := args[0].Value().(phpv.ZString)
	user := args[1].Value().(phpv.ZString)

	r := subtle.ConstantTimeCompare([]byte(known), []byte(user))

	return phpv.ZBool(r == 1).ZVal(), nil
}
