package hash

import (
	"crypto/hmac"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
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

// > func string hash ( string $algo , string $data [, bool $raw_output = FALSE ] )
func fncHash(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var algo phpv.ZString
	var data phpv.ZString
	var raw *phpv.ZBool

	_, err := core.Expand(ctx, args, &algo, &data, &raw)
	if err != nil {
		return nil, err
	}

	algN, ok := algos[algo.ToLower()]
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash(): Argument #1 ($algo) must be a valid hashing algorithm")
	}

	a := algN()
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

// > func string hash_file ( string $algo, string $filename [, bool $raw_output = FALSE ] )
func fncHashFile(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var algo phpv.ZString
	var filename phpv.ZString
	var raw *phpv.ZBool

	_, err := core.Expand(ctx, args, &algo, &filename, &raw)
	if err != nil {
		return nil, err
	}

	algN, ok := algos[algo.ToLower()]
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_file(): Argument #1 ($algo) must be a valid hashing algorithm")
	}

	f, err := openFileChecked(ctx, string(filename), "hash_file")
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
	}
	defer f.Close()

	a := algN()
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
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_hmac_file(): Argument #1 ($algo) must be a valid hashing algorithm")
	}
	if nonCryptoAlgos[algo.ToLower()] {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_hmac_file(): Argument #1 ($algo) must be a valid cryptographic hashing algorithm")
	}

	f, err := openFileChecked(ctx, string(filename), "hash_hmac_file")
	if err != nil {
		return phpv.ZBool(false).ZVal(), nil
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
