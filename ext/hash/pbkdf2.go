package hash

import (
	"encoding/hex"
	"fmt"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
	"golang.org/x/crypto/pbkdf2"
)

//> func string hash_pbkdf2 ( string $algo , string $password , string $salt , int $iterations [, int $length = 0 [, bool $raw_output = FALSE ]] )
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
		return nil, fmt.Errorf("Unknown hashing algorithm: %s", algo)
	}

	// not sure about key length, let's go with 128 by default?
	length := 128
	if l != nil {
		length = int(*l)
		if length <= 0 {
			length = 128
		}
	}
	r := pbkdf2.Key([]byte(password), []byte(salt), int(iter), length, algN)

	if raw != nil && *raw {
		// return as raw
		return phpv.ZString(r).ZVal(), nil
	}

	// convert to hex, cut to "length" because PHP implementation is weird
	return phpv.ZString(hex.EncodeToString(r))[:length].ZVal(), nil
}
