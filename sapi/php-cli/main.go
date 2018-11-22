package main

import (
	"context"
	"log"
	"os"

	"github.com/MagicalTux/gophp/core"
	_ "github.com/MagicalTux/gophp/ext/ctype"
	_ "github.com/MagicalTux/gophp/ext/gmp"
	_ "github.com/MagicalTux/gophp/ext/pcre"
	_ "github.com/MagicalTux/gophp/ext/standard"
)

func main() {
	p := core.NewProcess("cli")
	ctx := core.NewGlobal(context.Background(), p)
	if len(os.Args) == 2 {
		if err := ctx.RunFile(os.Args[1]); err != nil {
			log.Printf("failed to run file: %s", err)
			os.Exit(1)
		}
	}
}
