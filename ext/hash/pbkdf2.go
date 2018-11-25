package hash

import (
	"encoding/hex"
	"fmt"

	"github.com/MagicalTux/gophp/core"
	"golang.org/x/crypto/pbkdf2"
)

//> func string hash_pbkdf2 ( string $algo , string $password , string $salt , int $iterations [, int $length = 0 [, bool $raw_output = FALSE ]] )
func fncHashPbkdf2(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var algo core.ZString
	var password core.ZString
	var salt core.ZString
	var iter core.ZInt
	var l *core.ZInt
	var raw *core.ZBool

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
		return core.ZString(r).ZVal(), nil
	}

	// convert to hex, cut to "length" because PHP implementation is weird
	return core.ZString(hex.EncodeToString(r))[:length].ZVal(), nil
}
