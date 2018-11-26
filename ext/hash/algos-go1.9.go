package hash

import "golang.org/x/crypto/ripemd160"

// +build go1.9

func init() {
	algos["ripemd160"] = ripemd160.New
}
