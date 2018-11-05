package main

import (
	"context"
	"log"

	"git.atonline.com/tristantech/gophp/core"
)

func main() {
	// by default, run script test.php
	p := core.NewProcess()
	ctx := core.NewContext(context.Background(), p)
	if err := ctx.RunFile("test.php"); err != nil {
		log.Printf("failed to run test file: %s", err)
	}
}
