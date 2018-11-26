// +build go1.9

package hash

import (
	"hash/fnv"

	"golang.org/x/crypto/ripemd160"
)

func init() {
	algos["ripemd160"] = ripemd160.New
	// fnv128: https://go-review.googlesource.com/c/go/+/38356/
	algos["fnv1128"] = fnv.New128
	algos["fnv1a128"] = fnv.New128a
}
