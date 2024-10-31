package hash

import (
	"errors"
	"fmt"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
	"golang.org/x/crypto/hkdf"
)

// > func string hash_hkdf ( string $algo , string $ikm [, int $length = 0 [, string $info = ” [, string $salt = ” ]]] )
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

	return phpv.ZString(b).ZVal(), nil
}
