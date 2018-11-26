package hash

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"hash"
	gohash "hash"
	"hash/adler32"
	"hash/crc32"
	"hash/fnv"

	"github.com/MagicalTux/goro/core"
	"golang.org/x/crypto/md4"
	"golang.org/x/crypto/sha3"
)

var algos = map[core.ZString]func() gohash.Hash{
	"md4":        md4.New,
	"md5":        md5.New,
	"sha1":       sha1.New,
	"sha256":     sha256.New,
	"sha224":     sha256.New224,
	"sha384":     sha512.New384,
	"sha512":     sha512.New,
	"sha512/224": sha512.New512_224,
	"sha512/256": sha512.New512_256,
	"sha3-224":   sha3.New224,
	"sha3-256":   sha3.New256,
	"sha3-384":   sha3.New384,
	"sha3-512":   sha3.New512,
	"keccak256":  sha3.NewLegacyKeccak256, // ?
	"adler32":    hash32W(adler32.New).New,
	"crc32":      crc32New,
	"crc32b":     hash32W(crc32.NewIEEE).New,
	"fnv132":     hash32W(fnv.New32).New,
	"fnv1a32":    hash32W(fnv.New32a).New,
	"fnv164":     hash64W(fnv.New64).New,
	"fnv1a64":    hash64W(fnv.New64a).New,
}

// for types returning hash.Hash32 types, wrap them so they return hash.Hash
type hash32W func() gohash.Hash32

func (h hash32W) New() hash.Hash {
	return h()
}

type hash64W func() gohash.Hash64

func (h hash64W) New() hash.Hash {
	return h()
}
