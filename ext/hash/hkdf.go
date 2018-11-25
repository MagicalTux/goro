package hash

import (
	"errors"
	"fmt"

	"github.com/MagicalTux/gophp/core"
	"golang.org/x/crypto/hkdf"
)

//> func string hash_hkdf ( string $algo , string $ikm [, int $length = 0 [, string $info = '' [, string $salt = '' ]]] )
func fncHashHkdf(ctx core.Context, args []*core.ZVal) (*core.ZVal, error) {
	var algo core.ZString
	var ikm core.ZString
	var l *core.ZInt
	var info *core.ZString
	var salt *core.ZString

	_, err := core.Expand(ctx, args, &algo, &ikm, &l, &info, &salt)
	if err != nil {
		return nil, err
	}

	algN, ok := algos[algo.ToLower()]
	if !ok {
		return nil, fmt.Errorf("Unknown hashing algorithm: %s", algo)
	}

	var i, s []byte
	if info != nil {
		i = []byte(*info)
	}
	if salt != nil {
		s = []byte(*salt)
	}

	v := hkdf.New(algN, []byte(ikm), s, i)

	length := algN().Size() // hash length
	if l != nil {
		length = int(*l)
	}

	b := make([]byte, length)
	n, err := v.Read(b)
	if err != nil {
		return nil, err
	}
	if n != length {
		return nil, errors.New("failed to read that many bytes")
	}

	return core.ZString(b).ZVal(), nil
}
