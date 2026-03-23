package hash

import (
	"fmt"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
	"golang.org/x/crypto/hkdf"
)

// > func string hash_hkdf ( string $algo , string $ikm [, int $length = 0 [, string $info = " [, string $salt = " ]]] )
func fncHashHkdf(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var algo phpv.ZString
	var ikm phpv.ZString
	var l *phpv.ZInt
	var info *phpv.ZString
	var salt *phpv.ZString

	_, err := core.Expand(ctx, args, &algo, &ikm, &l, &info, &salt)
	if err != nil {
		return nil, err
	}

	algN, ok := algos[algo.ToLower()]
	if !ok {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_hkdf(): Argument #1 ($algo) must be a valid cryptographic hashing algorithm")
	}
	if nonCryptoAlgos[algo.ToLower()] {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_hkdf(): Argument #1 ($algo) must be a valid cryptographic hashing algorithm")
	}

	if len(ikm) == 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_hkdf(): Argument #2 ($key) must not be empty")
	}

	var i, s []byte
	if info != nil {
		i = []byte(*info)
	}
	if salt != nil {
		s = []byte(*salt)
	}

	hashSize := algN().Size()
	length := hashSize // default: hash length
	if l != nil {
		length = int(*l)
	}

	if length < 0 {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "hash_hkdf(): Argument #3 ($length) must be greater than or equal to 0")
	}
	if length == 0 {
		length = hashSize
	}

	maxLength := hashSize * 255
	if length > maxLength {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, fmt.Sprintf("hash_hkdf(): Argument #3 ($length) must be less than or equal to %d", maxLength))
	}

	v := hkdf.New(algN, []byte(ikm), s, i)

	b := make([]byte, length)
	n, err := v.Read(b)
	if err != nil {
		return nil, err
	}
	if n != length {
		return nil, fmt.Errorf("failed to read that many bytes")
	}

	return phpv.ZString(b).ZVal(), nil
}
