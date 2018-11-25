package main

import (
	"context"
	"log"
	"os"

	"github.com/MagicalTux/goro/core"
	_ "github.com/MagicalTux/goro/ext/ctype"
	_ "github.com/MagicalTux/goro/ext/gmp"
	_ "github.com/MagicalTux/goro/ext/hash"
	_ "github.com/MagicalTux/goro/ext/pcre"
	_ "github.com/MagicalTux/goro/ext/standard"
)

func main() {
	p := core.NewProcess("cli")
	p.CommandLine(os.Args)
	ctx := core.NewGlobal(context.Background(), p)
	if len(os.Args) == 2 {
		if err := ctx.RunFile(os.Args[1]); err != nil {
			log.Printf("failed to run file: %s", err)
			os.Exit(1)
		}
	}
}
