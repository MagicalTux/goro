package hash

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"fmt"
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
	"md2":        newMD2,
	"md4":        func() gohash.Hash { return newHashReplayable(md4.New(), md4.New) },
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
	"ripemd128":  newRipemd128,
	"ripemd256":  newRipemd256,
	"ripemd320":  newRipemd320,
	"whirlpool":  newWhirlpool,
	"tiger128,3": func() gohash.Hash { return newTiger(128, 3) },
	"tiger160,3": func() gohash.Hash { return newTiger(160, 3) },
	"tiger192,3": func() gohash.Hash { return newTiger(192, 3) },
	"tiger128,4": func() gohash.Hash { return newTiger(128, 4) },
	"tiger160,4": func() gohash.Hash { return newTiger(160, 4) },
	"tiger192,4": func() gohash.Hash { return newTiger(192, 4) },
	"snefru":     func() gohash.Hash { return newSnefru(256) },
	"snefru256":  func() gohash.Hash { return newSnefru(256) },
	"gost":       func() gohash.Hash { return newGost(false) },
	"gost-crypto": func() gohash.Hash { return newGost(true) },
	"adler32":    hash32W(adler32.New).New,
	"crc32":      crc32New,
	"crc32b":     hash32W(crc32.NewIEEE).New,
	"crc32c":     crc32cNew,
	"fnv132":     hash32W(fnv.New32).New,
	"fnv1a32":    hash32W(fnv.New32a).New,
	"fnv164":     hash64W(fnv.New64).New,
	"fnv1a64":    hash64W(fnv.New64a).New,
	"joaat":      newJoaat,
	// murmur3 - seeded via seededAlgos, default seed=0
	"murmur3a": func() gohash.Hash { return newMurmur3A(0) },
	"murmur3c": func() gohash.Hash { return newMurmur3C(0) },
	"murmur3f": func() gohash.Hash { return newMurmur3F(0) },
	// xxhash - seeded via seededAlgos, default seed=0
	"xxh32":  func() gohash.Hash { return newXXH32(0) },
	"xxh64":  func() gohash.Hash { return newXXH64(0) },
	"xxh3":   func() gohash.Hash { return newXXH3WithSeed(0) },
	"xxh128": func() gohash.Hash { return newXXH128WithSeed(0) },
}

// seededAlgos maps algo name → constructor that takes a uint32 seed.
// Used by murmur3 and xxh32/xxh64.
var seededAlgos = map[phpv.ZString]func(seed uint32) gohash.Hash{
	"murmur3a": newMurmur3A,
	"murmur3c": newMurmur3C,
	"murmur3f": newMurmur3F,
	"xxh32":    newXXH32,
	"xxh64":    func(seed uint32) gohash.Hash { return newXXH64(uint64(seed)) },
}

// seededAlgos64 maps algo name → constructor that takes a uint64 seed + secret.
// Used by xxh3 and xxh128.
var seededAlgos64 = map[phpv.ZString]func(seed uint64, secret []byte) gohash.Hash{
	"xxh3":   newXXH3WithSeedOrSecret,
	"xxh128": newXXH128WithSeedOrSecret,
}

// havalAlgos map - populated dynamically
func init() {
	// Add HAVAL variants
	for _, bits := range []int{128, 160, 192, 224, 256} {
		for _, passes := range []int{3, 4, 5} {
			name := phpv.ZString(havalName(bits, passes))
			b, p := bits, passes // capture loop vars
			algos[name] = func() gohash.Hash { return newHaval(b, p) }
		}
	}
}

func havalName(bits, passes int) string {
	return fmt.Sprintf("haval%d,%d", bits, passes)
}

// nonCryptoAlgos lists the hash algorithms that are NOT suitable for cryptographic use.
// PHP rejects these for HMAC, HKDF, and PBKDF2.
var nonCryptoAlgos = map[phpv.ZString]bool{
	"adler32":  true,
	"crc32":    true,
	"crc32b":   true,
	"crc32c":   true,
	"fnv132":   true,
	"fnv1a32":  true,
	"fnv164":   true,
	"fnv1a64":  true,
	"fnv1128":  true,
	"fnv1a128": true,
	"joaat":    true,
	"murmur3a": true,
	"murmur3c": true,
	"murmur3f": true,
	"xxh32":    true,
	"xxh64":    true,
	"xxh3":     true,
	"xxh128":   true,
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
	binary.BigEndian.PutUint32(b, h)
	return append(in, b...)
}

func (j *joaatHash) Reset()         { j.hash = 0 }
func (j *joaatHash) Size() int      { return 4 }
func (j *joaatHash) BlockSize() int { return 1 }
func (j *joaatHash) CloneHash() hash.Hash {
	c := *j
	return &c
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
