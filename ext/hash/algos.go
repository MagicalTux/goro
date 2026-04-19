package hash

import (
	gohash "hash"
	"hash/fnv"

	"github.com/KarpelesLab/anyhash"
	"github.com/KarpelesLab/goro/core/phpv"
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

// anyhashName returns the anyhash-compatible name for a PHP algorithm name,
// resolving aliases from phpNameMap.
func anyhashName(algo phpv.ZString) string {
	if n, ok := phpNameMap[string(algo)]; ok {
		return n
	}
	return string(algo)
}

// isSeededAlgo reports whether an algorithm supports custom seeds (uint32).
// These are murmur3 variants and xxh32/xxh64.
func isSeededAlgo(algo phpv.ZString) bool {
	switch algo {
	case "murmur3a", "murmur3c", "murmur3f", "xxh32", "xxh64":
		return true
	}
	return false
}

// isSeeded64Algo reports whether an algorithm supports custom uint64 seeds
// and/or secrets. These are xxh3 and xxh128.
func isSeeded64Algo(algo phpv.ZString) bool {
	switch algo {
	case "xxh3", "xxh128":
		return true
	}
	return false
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
