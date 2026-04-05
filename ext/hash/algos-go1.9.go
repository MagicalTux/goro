//go:build go1.9
// +build go1.9

package hash

import (
	gohash "hash"
	"hash/fnv"

	"golang.org/x/crypto/ripemd160"
)

func init() {
	algos["ripemd160"] = func() gohash.Hash { return newHashReplayable(ripemd160.New(), ripemd160.New) }
	// fnv128: https://go-review.googlesource.com/c/go/+/38356/
	algos["fnv1128"] = fnv.New128
	algos["fnv1a128"] = fnv.New128a
}
