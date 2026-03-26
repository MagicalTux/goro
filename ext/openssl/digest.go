package openssl

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"hash"
	"sort"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
	"golang.org/x/crypto/md4"
	"golang.org/x/crypto/sha3"
)

// digestAlgos maps lowercase digest names to their hash constructor.
var digestAlgos = map[string]func() hash.Hash{
	"md4":        md4.New,
	"md5":        md5.New,
	"sha1":       sha1.New,
	"sha224":     sha256.New224,
	"sha256":     sha256.New,
	"sha384":     sha512.New384,
	"sha512":     sha512.New,
	"sha512-224": sha512.New512_224,
	"sha512-256": sha512.New512_256,
	"sha3-224":   sha3.New224,
	"sha3-256":   sha3.New256,
	"sha3-384":   sha3.New384,
	"sha3-512":   sha3.New512,
}

// > func string openssl_digest ( string $data , string $method [, bool $raw_output = false ] )
func fncOpensslDigest(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var data phpv.ZString
	var method phpv.ZString
	var raw *phpv.ZBool

	_, err := core.Expand(ctx, args, &data, &method, &raw)
	if err != nil {
		return nil, err
	}

	methodLower := strings.ToLower(string(method))
	newHash, ok := digestAlgos[methodLower]
	if !ok {
		ctx.Warn("openssl_digest(): Unknown digest algorithm")
		return phpv.ZBool(false).ZVal(), nil
	}

	h := newHash()
	h.Write([]byte(data))
	result := h.Sum(nil)

	if raw != nil && bool(*raw) {
		return phpv.ZString(result).ZVal(), nil
	}

	return phpv.ZString(hex.EncodeToString(result)).ZVal(), nil
}

// > func array openssl_get_md_methods ([ bool $aliases = false ])
func fncOpensslGetMdMethods(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var aliases *phpv.ZBool
	core.Expand(ctx, args, &aliases)

	names := make([]string, 0, len(digestAlgos))
	for name := range digestAlgos {
		names = append(names, name)
	}
	sort.Strings(names)

	a := phpv.NewZArray()
	for _, name := range names {
		a.OffsetSet(ctx, nil, phpv.ZString(name).ZVal())
	}

	// If aliases requested, add uppercase versions
	if aliases != nil && bool(*aliases) {
		for _, name := range names {
			upper := strings.ToUpper(name)
			a.OffsetSet(ctx, nil, phpv.ZString(upper).ZVal())
		}
	}

	return a.ZVal(), nil
}
