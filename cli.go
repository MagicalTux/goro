package main

import (
	"log"

	"git.atonline.com/tristantech/gophp/core"
)

func main() {
	// by default, run script test.php
	ctx := core.NewContext()
	if err := ctx.RunFile("test.php"); err != nil {
		log.Printf("failed to run test file: %s", err)
	}
}
