package main

import (
	"context"
	"log"
	"os"

	"git.atonline.com/tristantech/gophp/core"
	_ "git.atonline.com/tristantech/gophp/ext/standard"
)

func main() {
	p := core.NewProcess()
	ctx := core.NewGlobal(context.Background(), p)
	if len(os.Args) == 2 {
		if err := ctx.RunFile(os.Args[1]); err != nil {
			log.Printf("failed to run file: %s", err)
			os.Exit(1)
		}
	}
}
