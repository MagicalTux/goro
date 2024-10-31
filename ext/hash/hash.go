package hash

import (
	"crypto/subtle"
	"encoding/hex"
	"fmt"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

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
		return nil, fmt.Errorf("Unknown hashing algorithm: %s", algo)
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

// > func bool hash_equals ( string $known_string , string $user_string )
func fncHashEquals(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var known, user phpv.ZString

	_, err := core.Expand(ctx, args, &known, &user)
	if err != nil {
		return nil, err
	}

	r := subtle.ConstantTimeCompare([]byte(known), []byte(user))

	return phpv.ZBool(r == 1).ZVal(), nil
}
