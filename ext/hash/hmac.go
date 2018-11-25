package hash

import (
	"crypto/hmac"
	"encoding/hex"
	"fmt"

	"github.com/MagicalTux/goro/core"
)

//> func string hash_hmac ( string $algo , string $data , string $key [, bool $raw_output = FALSE ] )
func fncHashHmac(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var algo core.ZString
	var data core.ZString
	var key core.ZString
	var raw *core.ZBool

	_, err := core.Expand(ctx, args, &algo, &data, &key, &raw)
	if err != nil {
		return nil, err
	}

	algN, ok := algos[algo.ToLower()]
	if !ok {
		return nil, fmt.Errorf("Unknown hashing algorithm: %s", algo)
	}

	a := hmac.New(algN, []byte(key))
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
