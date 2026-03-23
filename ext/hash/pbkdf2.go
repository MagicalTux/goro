package hash

import (
	"encoding/hex"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"golang.org/x/crypto/pbkdf2"
)

// > func string hash_pbkdf2 ( string $algo , string $password , string $salt , int $iterations [, int $length = 0 [, bool $raw_output = FALSE ]] )
func fncHashPbkdf2(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var algo phpv.ZString
	var password phpv.ZString
	var salt phpv.ZString
	var iter phpv.ZInt
	var l *phpv.ZInt
	var raw *phpv.ZBool

	_, err := core.Expand(ctx, args, &algo, &password, &salt, &iter, &l, &raw)
	if err != nil {
		return nil, err
	}

	algN, ok := algos[algo.ToLower()]
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_pbkdf2(): Argument #1 ($algo) must be a valid cryptographic hashing algorithm")
	}
	if nonCryptoAlgos[algo.ToLower()] {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_pbkdf2(): Argument #1 ($algo) must be a valid cryptographic hashing algorithm")
	}

	if iter <= 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_pbkdf2(): Argument #4 ($iterations) must be greater than 0")
	}

	isRaw := raw != nil && *raw

	// PHP behavior: length=0 means use the full hash output length.
	// For raw output, that's the hash digest size.
	// For hex output, that's the hash digest size * 2.
	hashSize := algN().Size()
	length := 0
	if l != nil {
		length = int(*l)
	}

	if length < 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_pbkdf2(): Argument #5 ($length) must be greater than or equal to 0")
	}

	var keyLen int
	if length == 0 {
		keyLen = hashSize
	} else if isRaw {
		keyLen = length
	} else {
		// For hex output, we need ceil(length/2) raw bytes to get `length` hex chars
		keyLen = (length + 1) / 2
	}

	r := pbkdf2.Key([]byte(password), []byte(salt), int(iter), keyLen, algN)

	if isRaw {
		if length == 0 {
			return phpv.ZString(r).ZVal(), nil
		}
		return phpv.ZString(r[:length]).ZVal(), nil
	}

	// convert to hex
	hexStr := hex.EncodeToString(r)
	if length == 0 {
		return phpv.ZString(hexStr).ZVal(), nil
	}
	if length > len(hexStr) {
		return phpv.ZString(hexStr).ZVal(), nil
	}
	return phpv.ZString(hexStr[:length]).ZVal(), nil
}

