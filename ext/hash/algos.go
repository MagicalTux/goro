package hash

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"hash"
	gohash "hash"
	"hash/adler32"
	"hash/crc32"
	"hash/fnv"

	"github.com/MagicalTux/goro/core/phpv"
	"golang.org/x/crypto/md4"
	"golang.org/x/crypto/sha3"
)

var algos = map[phpv.ZString]func() gohash.Hash{
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
	"adler32":    hash32W(adler32.New).New,
	"crc32":      crc32New,
	"crc32b":     hash32W(crc32.NewIEEE).New,
	"crc32c":     crc32cNew,
	"fnv132":     hash32W(fnv.New32).New,
	"fnv1a32":    hash32W(fnv.New32a).New,
	"fnv164":     hash64W(fnv.New64).New,
	"fnv1a64":    hash64W(fnv.New64a).New,
	"joaat":      newJoaat,
}

// nonCryptoAlgos lists the hash algorithms that are NOT suitable for cryptographic use.
// PHP rejects these for HMAC, HKDF, and PBKDF2.
var nonCryptoAlgos = map[phpv.ZString]bool{
	"adler32": true,
	"crc32":   true,
	"crc32b":  true,
	"crc32c":  true,
	"fnv132":  true,
	"fnv1a32": true,
	"fnv164":  true,
	"fnv1a64": true,
	"fnv1128":  true,
	"fnv1a128": true,
	"joaat":   true,
}

func crc32cNew() gohash.Hash {
	return crc32.New(crc32.MakeTable(crc32.Castagnoli))
}

// joaat implements Jenkins's one-at-a-time hash
type joaatHash struct {
	hash uint32
}

func newJoaat() gohash.Hash {
	return &joaatHash{}
}

func (j *joaatHash) Write(p []byte) (int, error) {
	h := j.hash
	for _, b := range p {
		h += uint32(b)
		h += h << 10
		h ^= h >> 6
	}
	j.hash = h
	return len(p), nil
}

func (j *joaatHash) Sum(in []byte) []byte {
	h := j.hash
	h += h << 3
	h ^= h >> 11
	h += h << 15
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, h)
	return append(in, b...)
}

func (j *joaatHash) Reset()         { j.hash = 0 }
func (j *joaatHash) Size() int      { return 4 }
func (j *joaatHash) BlockSize() int { return 1 }

// for types returning hash.Hash32 types, wrap them so they return hash.Hash
type hash32W func() gohash.Hash32

func (h hash32W) New() hash.Hash {
	return h()
}

type hash64W func() gohash.Hash64

func (h hash64W) New() hash.Hash {
	return h()
}
