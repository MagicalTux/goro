package hash

// +build go1.9

import "golang.org/x/crypto/ripemd160"

func init() {
	algos["ripemd160"] = ripemd160.New
}
