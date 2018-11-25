package hash

import (
	"crypto/subtle"
	"encoding/hex"
	"fmt"

	"github.com/MagicalTux/goro/core"
)

//> func string hash ( string $algo , string $data [, bool $raw_output = FALSE ] )
func fncHash(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var algo core.ZString
	var data core.ZString
	var raw *core.ZBool

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
		return core.ZString(r).ZVal(), nil
	}

	// convert to hex
	return core.ZString(hex.EncodeToString(r)).ZVal(), nil
}

//> func bool hash_equals ( string $known_string , string $user_string )
func fncHashEquals(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var known, user core.ZString

	_, err := core.Expand(ctx, args, &known, &user)
	if err != nil {
		return nil, err
	}

	r := subtle.ConstantTimeCompare([]byte(known), []byte(user))

	return core.ZBool(r == 1).ZVal(), nil
}
