package hash

import (
	gohash "hash"
	"hash/fnv"

	"github.com/KarpelesLab/anyhash"
	"github.com/MagicalTux/goro/core/phpv"
)

// algos maps algorithm names to constructors. Populated from anyhash.List() at init.
var algos = map[phpv.ZString]func() gohash.Hash{}

// phpNameMap maps PHP algorithm names to anyhash names where they differ.
// anyhash normalizes hyphens away, so "sha3-256" becomes "sha3256" internally.
var phpNameMap = map[string]string{
	"gost-crypto": "gostcrypto",
	"sha3-224":    "sha3224",
	"sha3-256":    "sha3256",
	"sha3-384":    "sha3384",
	"sha3-512":    "sha3512",
}

func init() {
	for _, name := range anyhash.List() {
		n := name // capture loop var
		algos[phpv.ZString(n)] = func() gohash.Hash {
			h, _ := anyhash.New(n)
			return h
		}
	}
	// Add PHP-compatible aliases for algorithms with different names
	for phpName, ahName := range phpNameMap {
		n := ahName // capture
		algos[phpv.ZString(phpName)] = func() gohash.Hash {
			h, _ := anyhash.New(n)
			return h
		}
	}
	// fnv128 not in anyhash, add from stdlib
	algos["fnv1128"] = func() gohash.Hash { return fnv.New128() }
	algos["fnv1a128"] = func() gohash.Hash { return fnv.New128a() }
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
