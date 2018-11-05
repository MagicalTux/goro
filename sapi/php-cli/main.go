package main

import (
	"context"
	"log"
	"os"

	"git.atonline.com/tristantech/gophp/core"
)

func main() {
	ctx := core.NewContext(context.Background())
	if len(os.Args) == 1 {
		if err := ctx.RunFile(os.Args[1]); err != nil {
			log.Printf("failed to run test file: %s", err)
		}
	}
}
